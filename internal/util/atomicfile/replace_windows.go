//go:build windows

package atomicfile

import "golang.org/x/sys/windows"

// Replace moves source over target, replacing an existing destination (plain
// os.Rename fails on Windows when the destination already exists).
func Replace(source, target string) error {
	from, err := windows.UTF16PtrFromString(source)
	if err != nil {
		return err
	}
	to, err := windows.UTF16PtrFromString(target)
	if err != nil {
		return err
	}
	return windows.MoveFileEx(from, to, windows.MOVEFILE_REPLACE_EXISTING|windows.MOVEFILE_WRITE_THROUGH)
}
