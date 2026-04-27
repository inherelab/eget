package cli

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

func promptIndex(choices []string) (int, error) {
	for i, choice := range choices {
		fmt.Fprintf(os.Stderr, "(%d) %s\n", i+1, choice)
	}
	fmt.Fprint(os.Stderr, "Enter selection number: ")
	line, err := readStdinLine()
	if err != nil {
		return 0, err
	}
	picked, err := strconv.Atoi(strings.TrimSpace(line))
	if err != nil {
		return 0, err
	}
	return picked - 1, nil
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

func promptConfirmOverwrite(path string) (bool, error) {
	fmt.Fprintf(os.Stderr, "Config file already exists: %s\n", path)
	fmt.Fprint(os.Stderr, "Overwrite it? [y/N]: ")

	answer, err := readStdinLine()
	if err != nil {
		return false, err
	}
	answer = strings.ToLower(strings.TrimSpace(answer))
	return answer == "y" || answer == "yes", nil
}
