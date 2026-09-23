//go:build !windows

package atomicfile

import "os"

// Replace moves source over target in one step.
func Replace(source, target string) error {
	return os.Rename(source, target)
}
