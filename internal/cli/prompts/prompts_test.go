package prompts

import (
	"io"
	"os"
	"strings"
	"testing"

	"github.com/gookit/cliui/interact/backend/readline"
	"github.com/gookit/goutil/x/assert"
)

func TestPromptIndexConsumesTrailingNewline(t *testing.T) {
	origStdin := os.Stdin
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("create stdin pipe: %v", err)
	}
	os.Stdin = reader
	defer func() {
		os.Stdin = origStdin
		_ = reader.Close()
	}()
	if _, err := writer.WriteString("14\ny\n"); err != nil {
		t.Fatalf("write stdin: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close stdin writer: %v", err)
	}

	choices := make([]string, 14)
	for i := range choices {
		choices[i] = "choice"
	}
	picked, err := Index(choices)
	if err != nil {
		t.Fatalf("prompt index: %v", err)
	}
	if picked != 13 {
		t.Fatalf("expected zero-based selection 13, got %d", picked)
	}

	rest, err := io.ReadAll(os.Stdin)
	if err != nil {
		t.Fatalf("read remaining stdin: %v", err)
	}
	if string(rest) != "y\n" {
		t.Fatalf("expected prompt index to consume selection newline, remaining stdin %q", rest)
	}
}

func TestPromptIndexRendersInteractiveSelect(t *testing.T) {
	origStdin := os.Stdin
	origStderr := os.Stderr
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("create stdin pipe: %v", err)
	}
	errReader, errWriter, err := os.Pipe()
	if err != nil {
		t.Fatalf("create stderr pipe: %v", err)
	}
	os.Stdin = reader
	os.Stderr = errWriter
	defer func() {
		os.Stdin = origStdin
		os.Stderr = origStderr
		_ = reader.Close()
		_ = errReader.Close()
		_ = errWriter.Close()
	}()
	if _, err := writer.WriteString("2\n"); err != nil {
		t.Fatalf("write stdin: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close stdin writer: %v", err)
	}

	picked, err := Index([]string{"first.zip", "second.zip"})
	assert.NoErr(t, err)
	assert.Eq(t, 1, picked)

	if err := errWriter.Close(); err != nil {
		t.Fatalf("close stderr writer: %v", err)
	}
	rendered, err := io.ReadAll(errReader)
	if err != nil {
		t.Fatalf("read stderr: %v", err)
	}
	got := string(rendered)
	assert.Contains(t, got, "Select package resource (2)")
	assert.Contains(t, got, "1) first.zip")
	assert.Contains(t, got, "2) second.zip")
}

func TestRunMultiSelectIndexesConsumesChoices(t *testing.T) {
	picked, err := runMultiSelectIndexes(
		strings.NewReader("1,3\n"),
		io.Discard,
		readline.New(),
		"Select packages",
		"Filter packages",
		[]string{"fzf  v0.50.0 -> v0.51.0", "rg  v13.0.0 -> v14.0.0", "fd  v9.0.0 -> v10.0.0"},
	)

	assert.NoErr(t, err)
	assert.Eq(t, []int{0, 2}, picked)
}
