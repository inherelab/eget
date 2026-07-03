package prompts

import (
	"context"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/gookit/cliui/interact/backend"
	"github.com/gookit/cliui/interact/backend/readline"
	interactui "github.com/gookit/cliui/interact/ui"
	"golang.org/x/term"
)

func Index(choices []string) (int, error) {
	return Select("Select package resource", "Filter assets", choices)
}

func Select(title, filterPrompt string, choices []string) (int, error) {
	return runSelectIndex(os.Stdin, os.Stderr, readline.New(), title, filterPrompt, choices)
}

func MultiSelect(title, filterPrompt string, choices []string) ([]int, error) {
	return runMultiSelectIndexes(os.Stdin, os.Stderr, readline.New(), title, filterPrompt, choices)
}

func runSelectIndex(in io.Reader, out io.Writer, be backend.Backend, title, filterPrompt string, choices []string) (int, error) {
	items := make([]interactui.Item, 0, len(choices))
	for i, choice := range choices {
		key := strconv.Itoa(i + 1)
		items = append(items, interactui.Item{
			Key:   key,
			Label: choice,
			Value: i,
		})
	}

	selectUI := interactui.NewSelect(fmt.Sprintf("%s (%d)", title, len(items)), items)
	selectUI.Filterable = true
	selectUI.FilterPrompt = filterPrompt
	selectUI.PageSize = 12
	if len(items) > 0 {
		selectUI.DefaultKey = items[0].Key
	}

	result, err := selectUI.RunWithIO(context.Background(), be, selectInputReader(in), out)
	if err != nil {
		return 0, err
	}
	if index, ok := result.Value.(int); ok {
		return index, nil
	}
	picked, err := strconv.Atoi(result.Key)
	return picked - 1, err
}

func runMultiSelectIndexes(in io.Reader, out io.Writer, be backend.Backend, title, filterPrompt string, choices []string) ([]int, error) {
	items := make([]interactui.Item, 0, len(choices))
	for i, choice := range choices {
		key := strconv.Itoa(i + 1)
		items = append(items, interactui.Item{
			Key:   key,
			Label: choice,
			Value: i,
		})
	}

	selectUI := interactui.NewMultiSelect(fmt.Sprintf("%s (%d)", title, len(items)), items)
	selectUI.Filterable = true
	selectUI.FilterPrompt = filterPrompt
	selectUI.PageSize = 12

	result, err := selectUI.RunWithIO(context.Background(), be, selectInputReader(in), out)
	if err != nil {
		return nil, err
	}
	indexes := make([]int, 0, len(result.Values))
	for _, value := range result.Values {
		if index, ok := value.(int); ok {
			indexes = append(indexes, index)
		}
	}
	if len(indexes) == len(result.Values) {
		return indexes, nil
	}
	for _, key := range result.Keys {
		picked, err := strconv.Atoi(key)
		if err != nil {
			return nil, err
		}
		indexes = append(indexes, picked-1)
	}
	return indexes, nil
}

func selectInputReader(in io.Reader) io.Reader {
	if file, ok := in.(*os.File); ok && term.IsTerminal(int(file.Fd())) {
		return file
	}
	return singleByteReader{reader: in}
}

type singleByteReader struct {
	reader io.Reader
}

func (r singleByteReader) Read(p []byte) (int, error) {
	if len(p) > 1 {
		p = p[:1]
	}
	return r.reader.Read(p)
}

func readStdinLine() (string, error) {
	var b strings.Builder
	buf := make([]byte, 1)
	for {
		n, err := os.Stdin.Read(buf)
		if n > 0 {
			b.WriteByte(buf[0])
			if buf[0] == '\n' {
				return b.String(), nil
			}
		}
		if err != nil {
			if err == io.EOF {
				return b.String(), nil
			}
			return "", err
		}
	}
}

func ConfirmOverwrite(path string) (bool, error) {
	fmt.Fprintf(os.Stderr, "Config file already exists: %s\n", path)
	fmt.Fprint(os.Stderr, "Overwrite it? [y/N]: ")
	return ConfirmDefaultNo()
}

func ConfirmRemove(target string) (bool, error) {
	fmt.Fprintf(os.Stderr, "Remove %s? [y/N]: ", target)
	return ConfirmDefaultNo()
}

func ConfirmDefaultNo() (bool, error) {
	answer, err := readStdinLine()
	if err != nil {
		return false, err
	}
	answer = strings.ToLower(strings.TrimSpace(answer))
	return answer == "y" || answer == "yes", nil
}
