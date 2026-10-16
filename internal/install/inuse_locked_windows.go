//go:build windows

package install

import (
	"errors"
	"syscall"

	"golang.org/x/sys/windows"
)

// lockedFileError reports whether err means the target file is currently
// executed/in-use and cannot be opened for writing on Windows. A running
// executable surfaces as access denied, an open handle as sharing violation.
func lockedFileError(err error) bool {
	var errno syscall.Errno
	if !errors.As(err, &errno) {
		return false
	}
	return errno == windows.ERROR_SHARING_VIOLATION || errno == windows.ERROR_ACCESS_DENIED
}
