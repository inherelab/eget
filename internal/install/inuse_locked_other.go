//go:build !windows

package install

import (
	"errors"
	"syscall"
)

// lockedFileError reports whether err means the target file is currently
// executed/in-use and cannot be opened for writing (unix: text file busy).
func lockedFileError(err error) bool {
	return errors.Is(err, syscall.ETXTBSY)
}
