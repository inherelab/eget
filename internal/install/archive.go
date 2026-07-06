package install

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type FileType byte

const (
	TypeNormal FileType = iota
	TypeDir
	TypeLink
	TypeSymlink
	TypeOther
)

type File struct {
	Name     string
	LinkName string
	Mode     fs.FileMode
	ModTime  time.Time
	Type     FileType
}

func (f File) Dir() bool {
	return f.Type == TypeDir
}

type Archive interface {
	Next() (File, error)
	ReadAll() ([]byte, error)
}

type archiveEntryWriter interface {
	WriteTo(w io.Writer) (int64, error)
}

type ArchiveFn func(data []byte, decomp DecompFn) (Archive, error)
type DecompFn func(r io.Reader) (io.Reader, error)

type ArchiveExtractor struct {
	File       Chooser
	Ar         ArchiveFn
	Decompress DecompFn
}

type ArchiveExtractOptions struct {
	StripComponents int
}

func NewArchiveExtractor(file Chooser, ar ArchiveFn, decompress DecompFn) *ArchiveExtractor {
	return &ArchiveExtractor{File: file, Ar: ar, Decompress: decompress}
}

func (a *ArchiveExtractor) Extract(data []byte, multiple bool) (ExtractedFile, []ExtractedFile, error) {
	var candidates []ExtractedFile
	var dirs []string
	ar, err := a.Ar(data, a.Decompress)
	if err != nil {
		return ExtractedFile{}, nil, err
	}
	for {
		f, err := ar.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return ExtractedFile{}, nil, fmt.Errorf("extract: %w", err)
		}
		var hasdir bool
		for _, d := range dirs {
			if strings.HasPrefix(f.Name, d) {
				hasdir = true
				break
			}
		}
		if hasdir {
			continue
		}
		direct, possible := a.File.Choose(f.Name, f.Dir(), f.Mode)
		if direct || possible {
			name := rename(f.Name, f.Name)
			fdata, err := ar.ReadAll()
			if err != nil {
				return ExtractedFile{}, nil, fmt.Errorf("extract: %w", err)
			}
			var extract func(to string) error
			if !f.Dir() {
				extract = func(to string) error {
					return writeFileWithModTime(fdata, to, modeFrom(name, f.Mode), f.ModTime)
				}
			} else {
				dirs = append(dirs, f.Name)
				extract = func(to string) error {
					subAr, err := a.Ar(data, a.Decompress)
					if err != nil {
						return err
					}
					type link struct {
						newname, oldname, archiveName string
						sym                           bool
					}
					stripComponents := len(archivePathParts(f.Name))
					var extractedDirs []File
					var links []link
					for {
						subf, err := subAr.Next()
						if err == io.EOF {
							break
						}
						if err != nil {
							return fmt.Errorf("extract: %w", err)
						}
						rel, ok := archiveChildPath(f.Name, subf.Name)
						if !ok {
							continue
						}
						if rel == "" {
							if subf.Dir() {
								if err := os.MkdirAll(to, 0o755); err != nil {
									return fmt.Errorf("extract: %w", err)
								}
								extractedDirs = append(extractedDirs, File{Name: to, ModTime: subf.ModTime})
							}
							continue
						}
						if subf.Dir() {
							dir, err := safeArchiveOutputPath(to, rel)
							if err != nil {
								return fmt.Errorf("extract: %w", err)
							}
							if err := os.MkdirAll(dir, 0o755); err != nil {
								return fmt.Errorf("extract: %w", err)
							}
							extractedDirs = append(extractedDirs, File{Name: dir, ModTime: subf.ModTime})
							continue
						}
						if subf.Type == TypeLink || subf.Type == TypeSymlink {
							newname, err := safeArchiveOutputPath(to, rel)
							if err != nil {
								return fmt.Errorf("extract: %w", err)
							}
							linkName := subf.LinkName
							if subf.Type == TypeLink {
								if err := validateArchiveLinkTarget(subf.LinkName); err != nil {
									return fmt.Errorf("extract: %w", err)
								}
							} else {
								rewritten, ok := archiveSymlinkTargetForOutput(to, newname, subf.Name, subf.LinkName, stripComponents)
								if !ok {
									continue
								}
								linkName = rewritten
							}
							links = append(links, link{
								newname:     newname,
								oldname:     linkName,
								archiveName: subf.Name,
								sym:         subf.Type == TypeSymlink,
							})
							continue
						}
						subData, err := subAr.ReadAll()
						if err != nil {
							return fmt.Errorf("extract: %w", err)
						}
						name, err := safeArchiveOutputPath(to, rel)
						if err != nil {
							return fmt.Errorf("extract: %w", err)
						}
						if err := writeFileWithModTime(subData, name, subf.Mode, subf.ModTime); err != nil {
							return fmt.Errorf("extract: %w", err)
						}
					}
					for _, l := range links {
						os.Remove(l.newname)
						os.MkdirAll(filepath.Dir(l.newname), 0o755)
						var err error
						if l.sym {
							_, err = safeArchiveRelativeLinkOutputPath(to, l.newname, l.oldname)
							if err == nil {
								err = os.Symlink(l.oldname, l.newname)
							}
						} else {
							oldname, pathErr := safeArchiveHardlinkOutputPath(to, l.archiveName, l.oldname, 0)
							if pathErr != nil {
								return fmt.Errorf("extract: %w", pathErr)
							}
							err = os.Link(oldname, l.newname)
						}
						if err != nil && err != os.ErrExist {
							return fmt.Errorf("extract: %w", err)
						}
					}
					for i := len(extractedDirs) - 1; i >= 0; i-- {
						if err := applyModTime(extractedDirs[i].Name, extractedDirs[i].ModTime); err != nil {
							return fmt.Errorf("extract: %w", err)
						}
					}
					return nil
				}
			}
			ef := ExtractedFile{Name: name, ArchiveName: f.Name, mode: f.Mode, Extract: extract, Dir: f.Dir()}
			candidates = append(candidates, ef)
		}
	}
	if len(candidates) == 1 {
		return candidates[0], nil, nil
	}
	if len(candidates) == 0 {
		return ExtractedFile{}, candidates, fmt.Errorf("target %v not found in archive", a.File)
	}
	return ExtractedFile{}, candidates, fmt.Errorf("%d candidates for target %v found", len(candidates), a.File)
}

func (a *ArchiveExtractor) ExtractAllTo(data []byte, output string) ([]string, error) {
	return a.ExtractAllToWithOptions(data, output, ArchiveExtractOptions{})
}

func (a *ArchiveExtractor) ExtractAllToWithOptions(data []byte, output string, opts ArchiveExtractOptions) ([]string, error) {
	if output == "" {
		output = "."
	}
	ar, err := a.Ar(data, a.Decompress)
	if err != nil {
		return nil, err
	}
	writer, _ := ar.(archiveEntryWriter)
	var extracted []string
	var dirs []File
	var links []struct {
		newname     string
		oldname     string
		archiveName string
		sym         bool
	}
	for {
		f, err := ar.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("extract: %w", err)
		}
		direct, possible := a.File.Choose(f.Name, f.Dir(), f.Mode)
		if !direct && !possible {
			continue
		}
		name, ok, err := stripArchivePath(f.Name, opts.StripComponents)
		if err != nil {
			return nil, fmt.Errorf("extract: %w", err)
		}
		if !ok {
			continue
		}
		target, err := safeArchiveOutputPath(output, name)
		if err != nil {
			return nil, fmt.Errorf("extract: %w", err)
		}
		switch f.Type {
		case TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				return nil, fmt.Errorf("extract: %w", err)
			}
			dirs = append(dirs, File{Name: target, ModTime: f.ModTime})
		case TypeLink, TypeSymlink:
			linkName := f.LinkName
			if f.Type == TypeLink {
				if err := validateArchiveLinkTarget(f.LinkName); err != nil {
					return nil, fmt.Errorf("extract: %w", err)
				}
			} else {
				rewritten, ok := archiveSymlinkTargetForOutput(output, target, f.Name, f.LinkName, opts.StripComponents)
				if !ok {
					continue
				}
				linkName = rewritten
			}
			links = append(links, struct {
				newname     string
				oldname     string
				archiveName string
				sym         bool
			}{newname: target, oldname: linkName, archiveName: f.Name, sym: f.Type == TypeSymlink})
		default:
			if err := writeArchiveEntry(ar, writer, target, f.Mode, f.ModTime); err != nil {
				return nil, fmt.Errorf("extract: %w", err)
			}
			extracted = append(extracted, target)
		}
	}
	for _, l := range links {
		os.Remove(l.newname)
		if err := os.MkdirAll(filepath.Dir(l.newname), 0o755); err != nil {
			return nil, fmt.Errorf("extract: %w", err)
		}
		var err error
		if l.sym {
			_, err = safeArchiveRelativeLinkOutputPath(output, l.newname, l.oldname)
			if err != nil {
				continue
			}
			err = os.Symlink(l.oldname, l.newname)
		} else {
			oldname, pathErr := safeArchiveHardlinkOutputPath(output, l.archiveName, l.oldname, opts.StripComponents)
			if pathErr != nil {
				return nil, fmt.Errorf("extract: %w", pathErr)
			}
			err = os.Link(oldname, l.newname)
		}
		if err != nil && err != os.ErrExist {
			return nil, fmt.Errorf("extract: %w", err)
		}
	}
	for i := len(dirs) - 1; i >= 0; i-- {
		if err := applyModTime(dirs[i].Name, dirs[i].ModTime); err != nil {
			return nil, fmt.Errorf("extract: %w", err)
		}
	}
	if opts.StripComponents > 0 && len(extracted) == 0 {
		return nil, fmt.Errorf("extract: no files extracted")
	}
	return extracted, nil
}

func stripArchivePath(name string, components int) (string, bool, error) {
	if components <= 0 {
		return name, true, nil
	}
	clean, err := safeArchiveRelativePath(name)
	if err != nil {
		return "", false, err
	}
	parts := strings.Split(filepath.ToSlash(clean), "/")
	if len(parts) <= components {
		return "", false, nil
	}
	return strings.Join(parts[components:], "/"), true, nil
}

func writeArchiveEntry(ar Archive, writer archiveEntryWriter, target string, mode fs.FileMode, modTime time.Time) error {
	if target == "-" {
		if writer != nil {
			_, err := writer.WriteTo(os.Stdout)
			return err
		}
		data, err := ar.ReadAll()
		if err != nil {
			return err
		}
		_, err = os.Stdout.Write(data)
		return err
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	out, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, modeFrom(target, mode))
	if err != nil {
		return err
	}
	defer out.Close()
	if writer != nil {
		_, err = writer.WriteTo(out)
		if err != nil {
			return err
		}
		return applyModTime(target, modTime)
	}
	data, err := ar.ReadAll()
	if err != nil {
		return err
	}
	if _, err = out.Write(data); err != nil {
		return err
	}
	return applyModTime(target, modTime)
}
