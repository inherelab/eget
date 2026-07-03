package install

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gookit/goutil/x/assert"
)

func TestNewExtractorSupports7zArchives(t *testing.T) {
	extractor := NewExtractor("tool.7z", "tool", NewBinaryChooser("tool"))
	if _, ok := extractor.(*ArchiveExtractor); !ok {
		t.Fatalf("expected 7z extractor to use archive extractor, got %T", extractor)
	}
}

func TestExtractFileRequestsMultipleMatches(t *testing.T) {
	if extractAllFromFileSpec("README") {
		t.Fatal("expected single file spec to keep single extraction mode")
	}
	if !extractAllFromFileSpec("README,LICENSE") {
		t.Fatal("expected comma-separated file spec to enable multi extraction mode")
	}
	if !extractAllFromFileSpec("*.exe") {
		t.Fatal("expected glob file spec to enable multi extraction mode")
	}
}

func TestArchiveDirectoryExtractRejectsPathTraversal(t *testing.T) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	if _, err := zw.Create("safe/"); err != nil {
		t.Fatalf("create zip dir: %v", err)
	}
	w, err := zw.Create("safe/../../evil.txt")
	if err != nil {
		t.Fatalf("create zip file: %v", err)
	}
	if _, err := w.Write([]byte("evil")); err != nil {
		t.Fatalf("write zip file: %v", err)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("close zip: %v", err)
	}

	chooser, err := NewFileChooser("*")
	if err != nil {
		t.Fatalf("NewFileChooser: %v", err)
	}
	extractor := NewArchiveExtractor(chooser, NewZipArchive, nil)
	file, _, err := extractor.Extract(buf.Bytes(), true)
	if err == nil || !strings.Contains(err.Error(), "unsafe archive path") {
		t.Fatalf("expected unsafe archive path error, got %v", err)
	}

	root := t.TempDir()
	if file.Extract != nil {
		t.Fatal("expected no extract function for unsafe archive")
	}
	if _, statErr := os.Stat(filepath.Join(root, "evil.txt")); !os.IsNotExist(statErr) {
		t.Fatalf("expected traversal output to be absent, stat error: %v", statErr)
	}
}

func TestSafeArchiveRelativePathNormalizesBackslashes(t *testing.T) {
	got, err := safeArchiveRelativePath(`Plugins\`)
	if err != nil {
		t.Fatalf("safe archive path: %v", err)
	}
	if got != "Plugins" {
		t.Fatalf("expected Plugins, got %q", got)
	}

	got, err = safeArchiveRelativePath(`docs\readme.txt`)
	if err != nil {
		t.Fatalf("safe archive file path: %v", err)
	}
	if got != filepath.Join("docs", "readme.txt") {
		t.Fatalf("expected normalized path, got %q", got)
	}

	root := t.TempDir()
	out, err := safeArchiveOutputPath(root, `docs\readme.txt`)
	if err != nil {
		t.Fatalf("safe archive output path: %v", err)
	}
	if out != filepath.Join(root, "docs", "readme.txt") {
		t.Fatalf("expected nested output path, got %q", out)
	}
}

func TestSafeArchiveRelativePathStripsAbsoluteMemberRoot(t *testing.T) {
	got, err := safeArchiveRelativePath(`/home/runner/work/app/bin/tool`)
	if err != nil {
		t.Fatalf("safe archive path: %v", err)
	}
	assert.Eq(t, filepath.Join("home", "runner", "work", "app", "bin", "tool"), got)

	root := t.TempDir()
	out, err := safeArchiveOutputPath(root, `/home/runner/work/app/bin/tool`)
	if err != nil {
		t.Fatalf("safe archive output path: %v", err)
	}
	assert.Eq(t, filepath.Join(root, "home", "runner", "work", "app", "bin", "tool"), out)
}

func TestArchiveExtractorExtractAllToPreservesEmptyDirsAndTimestamps(t *testing.T) {
	pluginTime := time.Date(2024, 1, 2, 3, 4, 4, 0, time.UTC)
	fileTime := time.Date(2024, 2, 3, 4, 5, 6, 0, time.UTC)
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	dirHeader := &zip.FileHeader{Name: `Plugins\`, Method: zip.Store, Modified: pluginTime}
	dirHeader.SetMode(0o755 | os.ModeDir)
	if _, err := zw.CreateHeader(dirHeader); err != nil {
		t.Fatalf("create zip dir: %v", err)
	}
	fileHeader := &zip.FileHeader{Name: "KeePass.exe", Method: zip.Store, Modified: fileTime}
	fileHeader.SetMode(0o644)
	w, err := zw.CreateHeader(fileHeader)
	if err != nil {
		t.Fatalf("create zip file: %v", err)
	}
	if _, err := w.Write([]byte("keepass")); err != nil {
		t.Fatalf("write zip file: %v", err)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("close zip: %v", err)
	}

	chooser, err := NewFileChooser("*")
	if err != nil {
		t.Fatalf("NewFileChooser: %v", err)
	}
	extractor := NewArchiveExtractor(chooser, NewZipArchive, nil)
	root := t.TempDir()
	files, err := extractor.ExtractAllTo(buf.Bytes(), root)
	if err != nil {
		t.Fatalf("extract all: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("expected one extracted file, got %#v", files)
	}

	dirInfo, err := os.Stat(filepath.Join(root, "Plugins"))
	if err != nil {
		t.Fatalf("expected empty Plugins dir: %v", err)
	}
	if !dirInfo.IsDir() {
		t.Fatalf("expected Plugins to be a directory")
	}
	if !dirInfo.ModTime().Equal(pluginTime) {
		t.Fatalf("expected Plugins mtime %s, got %s", pluginTime, dirInfo.ModTime())
	}
	fileInfo, err := os.Stat(filepath.Join(root, "KeePass.exe"))
	if err != nil {
		t.Fatalf("expected KeePass.exe: %v", err)
	}
	if !fileInfo.ModTime().Equal(fileTime) {
		t.Fatalf("expected KeePass.exe mtime %s, got %s", fileTime, fileInfo.ModTime())
	}
}

func TestZipArchiveDecodesCP866CyrillicNames(t *testing.T) {
	assertZipArchiveDecodesLegacyName(t, []byte{0x90, 0xe3, 0xe1, 0xe1, 0xaa, 0xa8, 0xa9}, "Русский")
}

func TestZipArchiveDecodesCP1251CyrillicNames(t *testing.T) {
	assertZipArchiveDecodesLegacyName(t, []byte{0xd0, 0xf3, 0xf1, 0xf1, 0xea, 0xe8, 0xe9}, "Русский")
}

func assertZipArchiveDecodesLegacyName(t *testing.T, encoded []byte, decoded string) {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	header := &zip.FileHeader{Name: string(encoded) + `/readme.txt`}
	header.NonUTF8 = true
	w, err := zw.CreateHeader(header)
	if err != nil {
		t.Fatalf("create zip file: %v", err)
	}
	if _, err := w.Write([]byte("ok")); err != nil {
		t.Fatalf("write zip file: %v", err)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("close zip: %v", err)
	}

	chooser, err := NewFileChooser("*")
	if err != nil {
		t.Fatalf("NewFileChooser: %v", err)
	}
	extractor := NewArchiveExtractor(chooser, NewZipArchive, nil)
	output := t.TempDir()
	paths, err := extractor.ExtractAllTo(buf.Bytes(), output)
	if err != nil {
		t.Fatalf("extract all: %v", err)
	}

	want := filepath.Join(output, decoded, "readme.txt")
	if _, err := os.Stat(want); err != nil {
		t.Fatalf("expected decoded Cyrillic file %q: %v; paths=%q", want, err, paths)
	}
}

func TestArchiveExtractorExtractPreservesFileTimestamp(t *testing.T) {
	fileTime := time.Date(2024, 3, 4, 5, 6, 8, 0, time.UTC)
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	fileHeader := &zip.FileHeader{Name: "bin/tool.exe", Method: zip.Store, Modified: fileTime}
	fileHeader.SetMode(0o755)
	w, err := zw.CreateHeader(fileHeader)
	if err != nil {
		t.Fatalf("create zip file: %v", err)
	}
	if _, err := w.Write([]byte("tool")); err != nil {
		t.Fatalf("write zip file: %v", err)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("close zip: %v", err)
	}

	extractor := NewArchiveExtractor(NewBinaryChooser("tool"), NewZipArchive, nil)
	file, candidates, err := extractor.Extract(buf.Bytes(), false)
	if err != nil {
		t.Fatalf("extract candidate: %v", err)
	}
	if len(candidates) != 0 {
		t.Fatalf("expected direct file, got candidates %#v", candidates)
	}
	target := filepath.Join(t.TempDir(), "tool.exe")
	if err := file.Extract(target); err != nil {
		t.Fatalf("extract file: %v", err)
	}
	info, err := os.Stat(target)
	if err != nil {
		t.Fatalf("stat extracted file: %v", err)
	}
	if !info.ModTime().Equal(fileTime) {
		t.Fatalf("expected tool.exe mtime %s, got %s", fileTime, info.ModTime())
	}
}

func TestZipArchivePreservesLegacyDOSTimestampWallTime(t *testing.T) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	fileHeader := &zip.FileHeader{
		Name:         "DiskSpd64.exe",
		Method:       zip.Store,
		ModifiedDate: uint16(19 + int(time.June)<<5 + (2025-1980)<<9),
		ModifiedTime: uint16(44<<5 + 21<<11),
	}
	w, err := zw.CreateHeader(fileHeader)
	if err != nil {
		t.Fatalf("create zip file: %v", err)
	}
	if _, err := w.Write([]byte("tool")); err != nil {
		t.Fatalf("write zip file: %v", err)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("close zip: %v", err)
	}

	ar, err := NewZipArchive(buf.Bytes(), nil)
	if err != nil {
		t.Fatalf("NewZipArchive: %v", err)
	}
	file, err := ar.Next()
	if err != nil {
		t.Fatalf("next zip file: %v", err)
	}

	assert.Eq(t, 2025, file.ModTime.Year())
	assert.Eq(t, time.June, file.ModTime.Month())
	assert.Eq(t, 19, file.ModTime.Day())
	assert.Eq(t, 21, file.ModTime.Hour())
	assert.Eq(t, 44, file.ModTime.Minute())
	assert.Eq(t, time.Local.String(), file.ModTime.Location().String())
}

func TestArchiveExtractorExtractPreservesSelectedDirectoryTimestamp(t *testing.T) {
	dirTime := time.Date(2024, 4, 5, 6, 7, 8, 0, time.UTC)
	fileTime := time.Date(2024, 5, 6, 7, 8, 10, 0, time.UTC)
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	dirHeader := &zip.FileHeader{Name: "CdmResource/", Method: zip.Store, Modified: dirTime}
	dirHeader.SetMode(0o755 | os.ModeDir)
	if _, err := zw.CreateHeader(dirHeader); err != nil {
		t.Fatalf("create zip dir: %v", err)
	}
	fileHeader := &zip.FileHeader{Name: "CdmResource/resource.txt", Method: zip.Store, Modified: fileTime}
	fileHeader.SetMode(0o644)
	w, err := zw.CreateHeader(fileHeader)
	if err != nil {
		t.Fatalf("create zip file: %v", err)
	}
	if _, err := w.Write([]byte("resource")); err != nil {
		t.Fatalf("write zip file: %v", err)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("close zip: %v", err)
	}

	chooser, err := NewFileChooser("CdmResource")
	if err != nil {
		t.Fatalf("NewFileChooser: %v", err)
	}
	extractor := NewArchiveExtractor(chooser, NewZipArchive, nil)
	file, candidates, err := extractor.Extract(buf.Bytes(), true)
	if err != nil {
		t.Fatalf("extract candidate: %v", err)
	}
	if len(candidates) != 0 {
		t.Fatalf("expected one selected directory, got candidates %#v", candidates)
	}
	target := filepath.Join(t.TempDir(), "CdmResource")
	if err := file.Extract(target); err != nil {
		t.Fatalf("extract selected directory: %v", err)
	}

	info, err := os.Stat(target)
	if err != nil {
		t.Fatalf("stat selected directory: %v", err)
	}
	if !info.ModTime().Equal(dirTime) {
		t.Fatalf("expected CdmResource mtime %s, got %s", dirTime, info.ModTime())
	}
}

type streamArchiveEntry struct {
	file    File
	content string
}

type streamArchive struct {
	entries []streamArchiveEntry
	idx     int
}

func TestArchiveExtractorExtractAllToStreamsArchiveOnce(t *testing.T) {
	opens := 0
	extractor := NewArchiveExtractor(&GlobChooser{all: true, expr: "*"}, func(data []byte, d DecompFn) (Archive, error) {
		opens++
		return &streamArchive{entries: []streamArchiveEntry{
			{file: File{Name: "docs", Mode: 0o755, Type: TypeDir}},
			{file: File{Name: "docs/readme.txt", Mode: 0o644, Type: TypeNormal}, content: "readme"},
			{file: File{Name: "bin/tool.exe", Mode: 0o755, Type: TypeNormal}, content: "tool"},
		}}, nil
	}, nil)

	tmp := t.TempDir()
	files, err := extractor.ExtractAllTo([]byte("archive"), tmp)
	if err != nil {
		t.Fatalf("extract all: %v", err)
	}

	if opens != 1 {
		t.Fatalf("expected archive to be opened once, got %d", opens)
	}
	if len(files) != 2 {
		t.Fatalf("expected 2 extracted files, got %#v", files)
	}
	data, err := os.ReadFile(filepath.Join(tmp, "docs", "readme.txt"))
	if err != nil {
		t.Fatalf("read extracted file: %v", err)
	}
	if string(data) != "readme" {
		t.Fatalf("expected readme content, got %q", string(data))
	}
	data, err = os.ReadFile(filepath.Join(tmp, "bin", "tool.exe"))
	if err != nil {
		t.Fatalf("read extracted executable: %v", err)
	}
	if string(data) != "tool" {
		t.Fatalf("expected tool content, got %q", string(data))
	}
}

func TestArchiveExtractorExtractAllToWithOptionsStripsComponents(t *testing.T) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	files := map[string]string{
		"go/bin/go":       "go",
		"go/pkg/tool.txt": "tool",
	}
	for name, content := range files {
		header := &zip.FileHeader{Name: name, Method: zip.Store}
		header.SetMode(0o644)
		w, err := zw.CreateHeader(header)
		if err != nil {
			t.Fatalf("create zip file: %v", err)
		}
		if _, err := w.Write([]byte(content)); err != nil {
			t.Fatalf("write zip file: %v", err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("close zip: %v", err)
	}

	extractor := NewArchiveExtractor(&GlobChooser{all: true, expr: "*"}, NewZipArchive, nil)
	root := t.TempDir()
	filesOut, err := extractor.ExtractAllToWithOptions(buf.Bytes(), root, ArchiveExtractOptions{StripComponents: 1})
	if err != nil {
		t.Fatalf("extract all with strip: %v", err)
	}
	if len(filesOut) != 2 {
		t.Fatalf("expected 2 extracted files, got %#v", filesOut)
	}
	data, err := os.ReadFile(filepath.Join(root, "bin", "go"))
	if err != nil {
		t.Fatalf("read stripped go file: %v", err)
	}
	if string(data) != "go" {
		t.Fatalf("expected go content, got %q", data)
	}
	if _, err := os.Stat(filepath.Join(root, "go")); !os.IsNotExist(err) {
		t.Fatalf("expected stripped root directory to be absent, stat err=%v", err)
	}
}

func TestArchiveExtractorExtractAllToWithOptionsStripsAbsoluteEntryRoot(t *testing.T) {
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	name := "/home/runner/work/AionUi/AionUi/resources/app/bin/aionui"
	if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0o755, Size: int64(len("app"))}); err != nil {
		t.Fatalf("write tar header: %v", err)
	}
	if _, err := tw.Write([]byte("app")); err != nil {
		t.Fatalf("write tar file: %v", err)
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("close tar: %v", err)
	}

	extractor := NewArchiveExtractor(&GlobChooser{all: true, expr: "*"}, NewTarArchive, func(r io.Reader) (io.Reader, error) { return r, nil })
	root := t.TempDir()
	filesOut, err := extractor.ExtractAllToWithOptions(buf.Bytes(), root, ArchiveExtractOptions{StripComponents: 5})
	if err != nil {
		t.Fatalf("extract all with strip: %v", err)
	}
	assert.Eq(t, []string{filepath.Join(root, "resources", "app", "bin", "aionui")}, filesOut)

	data, err := os.ReadFile(filepath.Join(root, "resources", "app", "bin", "aionui"))
	assert.NoErr(t, err)
	assert.Eq(t, "app", string(data))
	if _, err := os.Stat(filepath.Join(string(os.PathSeparator), "home", "runner", "work", "AionUi", "AionUi", "resources", "app", "bin", "aionui")); err == nil {
		t.Fatal("absolute archive path was written outside the extraction root")
	}
}

func TestArchiveExtractorExtractAllToWithOptionsAllowsSafeRelativeHardlinkTarget(t *testing.T) {
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	if err := tw.WriteHeader(&tar.Header{Name: "zulu/legal/java.base/LICENSE", Mode: 0o644, Size: int64(len("license"))}); err != nil {
		t.Fatalf("write tar file header: %v", err)
	}
	if _, err := tw.Write([]byte("license")); err != nil {
		t.Fatalf("write tar file: %v", err)
	}
	if err := tw.WriteHeader(&tar.Header{Name: "zulu/legal/java.desktop/LICENSE", Typeflag: tar.TypeLink, Linkname: "../java.base/LICENSE", Mode: 0o644}); err != nil {
		t.Fatalf("write tar link header: %v", err)
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("close tar: %v", err)
	}

	extractor := NewArchiveExtractor(&GlobChooser{all: true, expr: "*"}, NewTarArchive, func(r io.Reader) (io.Reader, error) { return r, nil })
	root := t.TempDir()
	_, err := extractor.ExtractAllToWithOptions(buf.Bytes(), root, ArchiveExtractOptions{StripComponents: 1})
	assert.NoErr(t, err)

	data, err := os.ReadFile(filepath.Join(root, "legal", "java.desktop", "LICENSE"))
	assert.NoErr(t, err)
	assert.Eq(t, "license", string(data))
}

func TestArchiveExtractorExtractAllToWithOptionsRejectsAbsoluteHardlinkTarget(t *testing.T) {
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	if err := tw.WriteHeader(&tar.Header{Name: "pkg/bin/tool", Typeflag: tar.TypeLink, Linkname: "/etc/passwd", Mode: 0o644}); err != nil {
		t.Fatalf("write tar link header: %v", err)
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("close tar: %v", err)
	}

	extractor := NewArchiveExtractor(&GlobChooser{all: true, expr: "*"}, NewTarArchive, func(r io.Reader) (io.Reader, error) { return r, nil })
	_, err := extractor.ExtractAllToWithOptions(buf.Bytes(), t.TempDir(), ArchiveExtractOptions{StripComponents: 1})
	if err == nil || !strings.Contains(err.Error(), "unsafe archive path") {
		t.Fatalf("expected unsafe archive path error, got %v", err)
	}
}

func TestArchiveExtractorExtractAllToWithOptionsRejectsEscapingRelativeHardlinkTarget(t *testing.T) {
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	if err := tw.WriteHeader(&tar.Header{Name: "zulu/legal/java.desktop/LICENSE", Typeflag: tar.TypeLink, Linkname: "../../../evil", Mode: 0o644}); err != nil {
		t.Fatalf("write tar link header: %v", err)
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("close tar: %v", err)
	}

	extractor := NewArchiveExtractor(&GlobChooser{all: true, expr: "*"}, NewTarArchive, func(r io.Reader) (io.Reader, error) { return r, nil })
	_, err := extractor.ExtractAllToWithOptions(buf.Bytes(), t.TempDir(), ArchiveExtractOptions{StripComponents: 1})
	if err == nil || !strings.Contains(err.Error(), "unsafe archive path") {
		t.Fatalf("expected unsafe archive path error, got %v", err)
	}
}

func TestArchiveExtractorExtractAllToWithOptionsRejectsAllSkippedEntries(t *testing.T) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	dirHeader := &zip.FileHeader{Name: "go/", Method: zip.Store}
	dirHeader.SetMode(0o755 | os.ModeDir)
	if _, err := zw.CreateHeader(dirHeader); err != nil {
		t.Fatalf("create zip dir: %v", err)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("close zip: %v", err)
	}

	extractor := NewArchiveExtractor(&GlobChooser{all: true, expr: "*"}, NewZipArchive, nil)
	_, err := extractor.ExtractAllToWithOptions(buf.Bytes(), t.TempDir(), ArchiveExtractOptions{StripComponents: 1})
	if err == nil {
		t.Fatal("expected extract all with strip to fail when all entries are skipped")
	}
}

func TestTarArchiveNextRejectsPathTraversal(t *testing.T) {
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	if err := tw.WriteHeader(&tar.Header{Name: "../evil.txt", Mode: 0o644, Size: int64(len("evil"))}); err != nil {
		t.Fatalf("write tar header: %v", err)
	}
	if _, err := tw.Write([]byte("evil")); err != nil {
		t.Fatalf("write tar file: %v", err)
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("close tar: %v", err)
	}

	ar, err := NewTarArchive(buf.Bytes(), func(r io.Reader) (io.Reader, error) { return r, nil })
	if err != nil {
		t.Fatalf("NewTarArchive: %v", err)
	}
	_, err = ar.Next()
	if err == nil || !strings.Contains(err.Error(), "unsafe archive path") {
		t.Fatalf("expected unsafe archive path error, got %v", err)
	}
}

func TestZipArchiveNextRejectsPathTraversal(t *testing.T) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, err := zw.Create("../evil.txt")
	if err != nil {
		t.Fatalf("create zip file: %v", err)
	}
	if _, err := w.Write([]byte("evil")); err != nil {
		t.Fatalf("write zip file: %v", err)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("close zip: %v", err)
	}

	ar, err := NewZipArchive(buf.Bytes(), nil)
	if err != nil {
		t.Fatalf("NewZipArchive: %v", err)
	}
	_, err = ar.Next()
	if err == nil || !strings.Contains(err.Error(), "unsafe archive path") {
		t.Fatalf("expected unsafe archive path error, got %v", err)
	}
}
