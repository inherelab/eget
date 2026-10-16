package install

import (
	"errors"
	"io/fs"
	"os"
)

// openReplaceFile opens target for writing (create/truncate), making room for
// "replace a binary that a running process holds locked":
//
//  1. best-effort delete the stale "<target>.old" backup from a previous update
//  2. plain open; on success the caller's job is done
//  3. when the open fails because the file is locked (in-use .exe), the old
//     file is renamed to "<target>.old" — Windows allows renaming running
//     executables but not truncating them — and the write retries on the
//     original path
//
// This keeps the extracted file at NAME(.exe) while the old version survives
// as NAME(.exe).old, matching the behavior of common Windows updaters.
func openReplaceFile(target string, mode fs.FileMode) (*os.File, error) {
	// Best-effort cleanup of the previous backup; failure is tolerated here
	// and reruns in renameLockedAside where it matters.
	os.Remove(target + ".old")

	out, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode)
	if err == nil {
		return out, nil
	}
	if !lockedFileError(err) || !isRegularFile(target) {
		return nil, err
	}
	if renameErr := renameLockedAside(target); renameErr != nil {
		return nil, err
	}
	return os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode)
}

// renameLockedAside moves target to "<target>.old" after removing any stale
// backup. A running executable cannot be deleted/truncated, but it can be
// renamed on Windows, so the old copy stays around until a later update
// removes it.
func renameLockedAside(target string) error {
	if err := os.Remove(target + ".old"); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return os.Rename(target, target+".old")
}

func isRegularFile(target string) bool {
	info, err := os.Stat(target)
	return err == nil && info.Mode().IsRegular()
}
