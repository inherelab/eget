package install

import (
	"fmt"
	"io"
	"os"
	"path"
	"time"
)

type downloadedResponseBody struct {
	io.ReadCloser
	cleanup func() error
}

func (b *downloadedResponseBody) Close() error {
	err := b.ReadCloser.Close()
	if cleanupErr := b.cleanup(); err == nil {
		err = cleanupErr
	}
	return err
}

type downloadBodyResult struct {
	Path     string
	Size     int64
	ModTime  time.Time
	Filename string
	Temp     bool
}

func (d downloadBodyResult) Open() (*os.File, error) {
	return os.Open(d.Path)
}

func (d downloadBodyResult) Close() error {
	if !d.Temp || d.Path == "" {
		return nil
	}
	err := os.Remove(d.Path)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

func shouldApplyDownloadedModTime(file ExtractedFile, assetURL string, opts Options, modTime time.Time) bool {
	if modTime.IsZero() || opts.ExtractFile != "" || opts.All {
		return false
	}
	return file.ArchiveName == path.Base(assetURL)
}

func extractAllTo(extractor DirectAllExtractor, source io.ReadSeeker, size int64, output string, stripComponents int) ([]string, error) {
	if withOptions, ok := extractor.(directAllExtractorWithOptions); ok {
		return withOptions.ExtractAllToWithOptions(source, size, output, ArchiveExtractOptions{StripComponents: stripComponents})
	}
	if stripComponents > 0 {
		return nil, fmt.Errorf("strip-components is not supported for this extractor")
	}
	return extractor.ExtractAllTo(source, size, output)
}
