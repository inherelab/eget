package sourceforge

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
)

type FileType string

const (
	TypeFile      FileType = "f"
	TypeDirectory FileType = "d"
)

type File struct {
	Name         string   `json:"name"`
	Path         string   `json:"path"`
	DownloadURL  string   `json:"download_url"`
	URL          string   `json:"url"`
	FullPath     string   `json:"full_path"`
	Type         FileType `json:"type"`
	Downloadable bool     `json:"downloadable"`
}

func ParseFilesPage(body []byte) ([]File, error) {
	const marker = "net.sf.files"

	start := bytes.Index(body, []byte(marker))
	if start < 0 {
		return nil, fmt.Errorf("sourceforge files data not found")
	}

	assign := bytes.IndexByte(body[start:], '=')
	if assign < 0 {
		return nil, fmt.Errorf("sourceforge files data not found")
	}

	objectStart := bytes.IndexByte(body[start+assign+1:], '{')
	if objectStart < 0 {
		return nil, fmt.Errorf("sourceforge files data not found")
	}
	objectStart += start + assign + 1

	objectEnd, err := findJSONObjectEnd(body, objectStart)
	if err != nil {
		return nil, err
	}

	filesByKey := make(map[string]File)
	if err := json.Unmarshal(body[objectStart:objectEnd], &filesByKey); err != nil {
		return nil, fmt.Errorf("parse sourceforge files data: %w", err)
	}
	if len(filesByKey) == 0 {
		return nil, fmt.Errorf("sourceforge files data not found")
	}

	keys := make([]string, 0, len(filesByKey))
	for key := range filesByKey {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		left, right := filesByKey[keys[i]], filesByKey[keys[j]]
		if left.Name != right.Name {
			return left.Name < right.Name
		}
		return keys[i] < keys[j]
	})

	files := make([]File, 0, len(keys))
	for _, key := range keys {
		files = append(files, filesByKey[key])
	}
	return files, nil
}

func findJSONObjectEnd(body []byte, start int) (int, error) {
	depth := 0
	inString := false
	escaped := false

	for i := start; i < len(body); i++ {
		ch := body[i]

		if inString {
			if escaped {
				escaped = false
				continue
			}
			switch ch {
			case '\\':
				escaped = true
			case '"':
				inString = false
			}
			continue
		}

		switch ch {
		case '"':
			inString = true
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return i + 1, nil
			}
		}
	}

	return 0, fmt.Errorf("sourceforge files data object is incomplete")
}
