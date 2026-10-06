//go:build windows

package app

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/gookit/goutil/x/assert"
)

func TestWindowsSelfReplaceScriptContainsQuotedPaths(t *testing.T) {
	script := windowsSelfReplaceScript(`C:\Tools\eget.exe`, `C:\Temp\eget-new.exe`, `C:\Tools\eget.exe.old`, `C:\Temp\eget-self-update.log`, 1234, `C:\Temp\eget-self-update-1234`)

	assert.Contains(t, script, `"C:\Tools\eget.exe"`)
	assert.Contains(t, script, `"C:\Temp\eget-new.exe"`)
	assert.Contains(t, script, `"C:\Tools\eget.exe.old"`)
	assert.Contains(t, script, "move /Y")
}

func TestWindowsSelfReplaceScriptFastRetriesBeforeOneSecondSleep(t *testing.T) {
	script := windowsSelfReplaceScript(`C:\Tools\eget.exe`, `C:\Temp\eget-new.exe`, `C:\Tools\eget.exe.old`, `C:\Temp\eget-self-update.log`, 1234, `C:\Temp\eget-self-update-1234`)

	assert.Contains(t, script, "set /A attempts+=1")
	assert.Contains(t, script, "EnableDelayedExpansion")
	assert.Contains(t, script, "if !attempts! GEQ 2000")
	assert.Contains(t, script, "timeout /T 1 /NOBREAK")
}

func TestWindowsSelfReplaceScriptWritesDiagnosticLog(t *testing.T) {
	script := windowsSelfReplaceScript(`C:\Tools\eget.exe`, `C:\Temp\eget-new.exe`, `C:\Tools\eget.exe.old`, `C:\Temp\eget-self-update.log`, 1234, `C:\Temp\eget-self-update-1234`)

	assert.Contains(t, script, `"C:\Temp\eget-self-update.log"`)
	assert.Contains(t, script, "replace succeeded")
	assert.Contains(t, script, "restore backup")
	assert.Contains(t, script, ":replace")
	assert.Contains(t, script, "waiting to move replacement")
}

func TestWindowsSelfReplaceScriptWaitsForParentProcessBeforeMovingExecutable(t *testing.T) {
	script := windowsSelfReplaceScript(`C:\Tools\eget.exe`, `C:\Temp\eget-new.exe`, `C:\Tools\eget.exe.old`, `C:\Temp\eget-self-update.log`, 1234, `C:\Temp\eget-self-update-1234`)

	assert.Contains(t, script, "set parent_pid=1234")
	assert.Contains(t, script, ":wait_parent")
	assert.Contains(t, script, `tasklist /FI "PID eq !parent_pid!"`)
	assert.Contains(t, script, "parent process exited")
	assert.Contains(t, script, ":wait")
}

func TestWindowsSelfReplaceScriptCleansSelfUpdateTempDir(t *testing.T) {
	script := windowsSelfReplaceScript(
		`C:\Tools\eget.exe`,
		`C:\Temp\eget-self-update-1234\eget.exe`,
		`C:\Tools\eget.exe.old`,
		`C:\Temp\eget-self-update-1234\eget.exe.replace.log`,
		1234,
		`C:\Temp\eget-self-update-1234`,
	)

	assert.Contains(t, script, ":cleanup")
	assert.Contains(t, script, `if /I "!temp_base:~0,17!"=="eget-self-update-"`)
	assert.Contains(t, script, `rmdir /S /Q "C:\Temp\eget-self-update-1234"`)
}

func TestWindowsSelfReplaceScriptRemovesTempDirEndToEnd(t *testing.T) {
	root := t.TempDir()
	tempDir := filepath.Join(root, "eget-self-update-1234")
	assert.NoErr(t, os.MkdirAll(tempDir, 0o755))
	current := filepath.Join(root, "eget.exe")
	replacement := filepath.Join(tempDir, "eget.exe")
	assert.NoErr(t, os.WriteFile(current, []byte("old"), 0o644))
	assert.NoErr(t, os.WriteFile(replacement, []byte("new"), 0o644))

	scriptPath := filepath.Join(root, "self-replace.cmd")
	script := windowsSelfReplaceScript(
		current,
		replacement,
		current+".old",
		replacement+".replace.log",
		999999,
		tempDir,
	)
	assert.NoErr(t, os.WriteFile(scriptPath, []byte(script), 0o644))

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	// The helper deletes its own batch file as the last step, so cmd can report
	// a spurious "batch file cannot be found" after the work is done. eget runs
	// the helper detached and ignores its exit status, so verify the effects.
	out, _ := exec.CommandContext(ctx, "cmd", "/C", scriptPath).CombinedOutput()

	updated, readErr := os.ReadFile(current)
	assert.NoErr(t, readErr)
	assert.Eq(t, "new", string(updated), "replacement not installed, script output: %s", out)

	_, statErr := os.Stat(tempDir)
	assert.True(t, os.IsNotExist(statErr), "self-update temp dir should be removed by the helper script, script output: %s", out)
}
