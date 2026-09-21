package install

import (
	"bufio"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

var system7zCandidates = []string{"7z", "7zz", "7za"}

func resolveSystem7zPath(configured string) string {
	if configured != "" {
		if info, err := os.Stat(configured); err == nil && !info.IsDir() {
			return configured
		}
	}
	for _, name := range system7zCandidates {
		if path, err := exec.LookPath(name); err == nil {
			return path
		}
	}
	return ""
}

func shouldUseSystem7z(filename string, extractArchive bool) bool {
	name := strings.ToLower(filepath.Base(filename))
	switch {
	case strings.HasSuffix(name, ".7z"),
		strings.HasSuffix(name, ".rar"),
		strings.HasSuffix(name, ".msi"),
		strings.HasSuffix(name, ".cab"),
		strings.HasSuffix(name, ".iso"):
		return true
	case strings.HasSuffix(name, ".exe") && extractArchive:
		return true
	default:
		return false
	}
}

type System7zExtractor struct {
	Filename string
	Tool     string
	Chooser  Chooser
	Exe      string
}

type system7zCommandRunner func(exe string, args ...string) ([]byte, error)

var runSystem7zCommand system7zCommandRunner = func(exe string, args ...string) ([]byte, error) {
	cmd := exec.Command(exe, args...)
	return cmd.CombinedOutput()
}

func NewSystem7zExtractor(filename, tool string, chooser Chooser, exe string) *System7zExtractor {
	return &System7zExtractor{Filename: filename, Tool: tool, Chooser: chooser, Exe: exe}
}

func (e *System7zExtractor) Extract(source io.ReadSeeker, size int64, multiple bool) (ExtractedFile, []ExtractedFile, error) {
	archivePath, cleanup, err := sourceArchivePath(source, e.Filename)
	if err != nil {
		return ExtractedFile{}, nil, err
	}
	defer cleanup()

	output, err := runSystem7zCommand(e.Exe, "l", "-slt", archivePath)
	if err != nil {
		return ExtractedFile{}, nil, fmt.Errorf("7z list: %w: %s", err, strings.TrimSpace(string(output)))
	}
	files, err := parseSystem7zListOutput(output)
	if err != nil {
		return ExtractedFile{}, nil, err
	}

	var candidates []ExtractedFile
	var shared *system7zExtractAllState
	if multiple {
		shared = &system7zExtractAllState{}
	}
	for _, file := range files {
		direct, possible := e.Chooser.Choose(file.Name, file.Dir(), file.Mode)
		if !direct && !possible {
			continue
		}
		name := rename(file.Name, file.Name)
		archiveName := filepath.ToSlash(file.Name)
		mode := file.Mode
		extracted := ExtractedFile{
			Name:        name,
			ArchiveName: file.Name,
			mode:        mode,
			Dir:         file.Dir(),
			Extract: func(to string) error {
				if shared != nil {
					return e.extractSharedMember(shared, source, e.Filename, archiveName, to, mode)
				}
				return e.extractMember(source, e.Filename, archiveName, to, mode)
			},
		}
		if direct && !multiple {
			return extracted, nil, nil
		}
		candidates = append(candidates, extracted)
	}

	if len(candidates) == 1 {
		if shared != nil {
			shared.remaining = 1
		}
		return candidates[0], nil, nil
	}
	if len(candidates) == 0 {
		return ExtractedFile{}, candidates, fmt.Errorf("target %v not found in archive", e.Chooser)
	}
	if shared != nil {
		shared.remaining = len(candidates)
	}
	return ExtractedFile{}, candidates, fmt.Errorf("%d candidates for target %v found", len(candidates), e.Chooser)
}

type system7zExtractAllState struct {
	once      sync.Once
	mu        sync.Mutex
	tempDir   string
	err       error
	remaining int
}

func (e *System7zExtractor) extractSharedMember(state *system7zExtractAllState, source io.ReadSeeker, filename, archiveName, to string, mode fs.FileMode) error {
	state.once.Do(func() {
		state.tempDir, state.err = e.extractAllToTempDir(source, filename)
	})
	if state.err != nil {
		return state.err
	}
	defer state.release()

	src, err := safeArchiveOutputPath(state.tempDir, archiveName)
	if err != nil {
		return err
	}
	return copyExtractedPath(src, to, mode)
}

func (s *system7zExtractAllState) release() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.remaining--
	if s.remaining <= 0 && s.tempDir != "" {
		_ = os.RemoveAll(s.tempDir)
		s.tempDir = ""
	}
}

func (e *System7zExtractor) extractAllToTempDir(source io.ReadSeeker, filename string) (string, error) {
	archivePath, cleanupArchive, err := sourceArchivePath(source, filename)
	if err != nil {
		return "", err
	}
	defer cleanupArchive()

	tempDir, err := os.MkdirTemp("", "eget-7z-out-*")
	if err != nil {
		return "", err
	}
	output, err := runSystem7zCommand(e.Exe, "x", "-y", "-o"+tempDir, archivePath)
	if err != nil {
		_ = os.RemoveAll(tempDir)
		return "", fmt.Errorf("7z extract: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return tempDir, nil
}

func (e *System7zExtractor) ExtractAllTo(source io.ReadSeeker, size int64, output string) ([]string, error) {
	return e.ExtractAllToWithOptions(source, size, output, ArchiveExtractOptions{})
}

func (e *System7zExtractor) ExtractAllToWithOptions(source io.ReadSeeker, size int64, output string, opts ArchiveExtractOptions) ([]string, error) {
	tempDir, err := e.extractAllToTempDir(source, e.Filename)
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tempDir)

	if output == "" {
		output = "."
	}
	var extracted []string
	err = filepath.WalkDir(tempDir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(tempDir, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		safeRel, err := safeArchiveRelativePath(filepath.ToSlash(rel))
		if err != nil {
			return err
		}
		strippedRel, ok, err := stripArchivePath(filepath.ToSlash(safeRel), opts.StripComponents)
		if err != nil {
			return err
		}
		if !ok {
			return nil
		}
		direct, possible := e.Chooser.Choose(filepath.ToSlash(safeRel), entry.IsDir(), 0)
		if !direct && !possible {
			return nil
		}
		target, err := safeArchiveOutputPath(output, strippedRel)
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if err := copyExtractedPath(path, target, info.Mode()); err != nil {
			return err
		}
		extracted = append(extracted, target)
		return nil
	})
	if err != nil {
		return nil, err
	}
	if opts.StripComponents > 0 && len(extracted) == 0 {
		return nil, fmt.Errorf("extract: no files extracted")
	}
	return extracted, nil
}

func (e *System7zExtractor) extractMember(source io.ReadSeeker, filename, archiveName, to string, mode fs.FileMode) error {
	archivePath, cleanupArchive, err := sourceArchivePath(source, filename)
	if err != nil {
		return err
	}
	defer cleanupArchive()

	tempDir, err := os.MkdirTemp("", "eget-7z-out-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tempDir)

	output, err := runSystem7zCommand(e.Exe, "x", "-y", "-o"+tempDir, archivePath, archiveName)
	if err != nil {
		return fmt.Errorf("7z extract: %w: %s", err, strings.TrimSpace(string(output)))
	}
	src, err := safeArchiveOutputPath(tempDir, archiveName)
	if err != nil {
		return err
	}
	return copyExtractedPath(src, to, mode)
}

func parseSystem7zListOutput(output []byte) ([]File, error) {
	var files []File
	fields := map[string]string{}
	flush := func() error {
		rawPath := fields["Path"]
		if rawPath == "" {
			return nil
		}
		if _, ok := fields["Size"]; !ok {
			return nil
		}
		name, err := safeArchiveRelativePath(rawPath)
		if err != nil {
			return err
		}
		typ := TypeNormal
		if fields["Folder"] == "+" || strings.HasSuffix(rawPath, "/") || strings.HasSuffix(rawPath, `\`) {
			typ = TypeDir
		}
		files = append(files, File{Name: name, Mode: 0o666, Type: typ})
		return nil
	}

	scanner := bufio.NewScanner(strings.NewReader(string(output)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || line == "----------" {
			if err := flush(); err != nil {
				return nil, err
			}
			fields = map[string]string{}
			continue
		}
		key, value, ok := strings.Cut(line, " = ")
		if !ok {
			continue
		}
		fields[key] = value
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if err := flush(); err != nil {
		return nil, err
	}
	return files, nil
}

func sourceArchivePath(source io.ReadSeeker, filename string) (string, func(), error) {
	if file, ok := source.(*os.File); ok && file.Name() != "" {
		return file.Name(), func() {}, nil
	}
	return writeTempArchive(source, filename)
}

func writeTempArchive(source io.ReadSeeker, filename string) (string, func(), error) {
	ext := filepath.Ext(filename)
	if ext == "" {
		ext = ".archive"
	}
	file, err := os.CreateTemp("", "eget-7z-*"+ext)
	if err != nil {
		return "", nil, err
	}
	cleanup := func() { _ = os.Remove(file.Name()) }
	if _, err := source.Seek(0, io.SeekStart); err != nil {
		_ = file.Close()
		cleanup()
		return "", nil, err
	}
	if _, err := io.Copy(file, source); err != nil {
		_ = file.Close()
		cleanup()
		return "", nil, err
	}
	if err := file.Close(); err != nil {
		cleanup()
		return "", nil, err
	}
	return file.Name(), cleanup, nil
}

func copyExtractedPath(src, dst string, mode fs.FileMode) error {
	info, err := os.Stat(src)
	if err != nil {
		return err
	}
	if info.IsDir() {
		var dirs []File
		err := filepath.WalkDir(src, func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			rel, err := filepath.Rel(src, path)
			if err != nil {
				return err
			}
			target := filepath.Join(dst, rel)
			if rel == "." {
				target = dst
			}
			if entry.IsDir() {
				if err := os.MkdirAll(target, 0o755); err != nil {
					return err
				}
				entryInfo, err := entry.Info()
				if err != nil {
					return err
				}
				dirs = append(dirs, File{Name: target, ModTime: entryInfo.ModTime()})
				return nil
			}
			entryInfo, err := entry.Info()
			if err != nil {
				return err
			}
			return copyFile(path, target, modeFrom(target, entryInfo.Mode()), entryInfo.ModTime())
		})
		if err != nil {
			return err
		}
		for i := len(dirs) - 1; i >= 0; i-- {
			if err := applyModTime(dirs[i].Name, dirs[i].ModTime); err != nil {
				return err
			}
		}
		return nil
	}
	return copyFile(src, dst, modeFrom(dst, mode), info.ModTime())
}

func copyFile(src, dst string, mode fs.FileMode, modTime time.Time) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err = io.Copy(out, in); err != nil {
		return err
	}
	return applyModTime(dst, modTime)
}
