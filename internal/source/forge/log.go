package forge

import (
	"fmt"
	"io"
	"os"
	"strings"
)

var verboseWriter io.Writer = os.Stderr
var verboseEnabled bool

func SetVerbose(enabled bool, writer io.Writer) {
	verboseEnabled = enabled
	if writer == nil {
		verboseWriter = io.Discard
		return
	}
	verboseWriter = writer
}

func verbosef(format string, args ...any) {
	if !verboseEnabled || verboseWriter == nil {
		return
	}
	fmt.Fprintf(verboseWriter, "[verbose] "+format+"\n", args...)
}

func truncateBody(body []byte) string {
	const limit = 240
	text := strings.TrimSpace(string(body))
	if len(text) <= limit {
		return text
	}
	return text[:limit] + "...(truncated)"
}

func VerboseEnabledForTest() bool {
	return verboseEnabled
}
