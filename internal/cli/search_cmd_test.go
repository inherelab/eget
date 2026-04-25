package cli

import (
	"strings"
	"testing"
)

func TestNewSearchCmdIncludesExamplesInLongHelp(t *testing.T) {
	cmd, _ := newSearchCmd(func(name string, options any) error {
		return nil
	})

	if !strings.Contains(cmd.LongHelp, "eget search markview") {
		t.Fatalf("expected basic example in long help, got %q", cmd.LongHelp)
	}
	if !strings.Contains(cmd.LongHelp, "eget search --limit 5 --sort stars --order desc terminal ui") {
		t.Fatalf("expected sort example in long help, got %q", cmd.LongHelp)
	}
	if !strings.Contains(cmd.LongHelp, "eget search --json picoclaw user:sipeed") {
		t.Fatalf("expected json example in long help, got %q", cmd.LongHelp)
	}
}
