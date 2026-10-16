package install

import (
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/gookit/goutil/x/assert"
)

// TestOpenReplaceFileReplacesRunningExecutable verifies the real in-use
// scenario end-to-end: a running executable image is locked for writing, so
// the replace must rename the locked copy to "<target>.old" and write the new
// content to the original path.
func TestOpenReplaceFileReplacesRunningExecutable(t *testing.T) {
	dir := t.TempDir()
	var exePath string
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		// Run a private copy of cmd.exe so it is a real running image.
		comspec := os.Getenv("COMSPEC")
		if comspec == "" {
			t.Skip("COMSPEC not set")
		}
		exePath = filepath.Join(dir, "tool.exe")
		copyFixture(t, comspec, exePath)
		cmd = exec.Command(exePath, "/c", "timeout", "/t", "60", "/nobreak", ">NUL")
	case "darwin", "linux":
		// Run a private copy of sleep so it is a real running image.
		src, err := exec.LookPath("sleep")
		assert.Nil(t, err)
		exePath = filepath.Join(dir, "sleep")
		copyFixture(t, src, exePath)
		cmd = exec.Command(exePath, "60")
	default:
		t.Skipf("no known running-executable fixture for %s", runtime.GOOS)
	}
	t.Cleanup(func() {
		if cmd.Process != nil {
			cmd.Process.Kill()
			cmd.Wait()
		}
	})

	assert.Nil(t, cmd.Start())
	waitForImageLock(t, exePath)

	// Precondition: the running image is really locked for writing.
	out, err := os.OpenFile(exePath, os.O_WRONLY, 0o666)
	if err == nil {
		assert.Nil(t, out.Close())
		t.Fatal("expected a running executable to be locked for writing")
	}

	// The locked copy is renamed aside and the write succeeds in place.
	out, err = openReplaceFile(exePath, 0o755)
	assert.Nil(t, err)
	_, err = io.WriteString(out, "new-content")
	assert.Nil(t, err)
	assert.Nil(t, out.Close())

	got, readErr := os.ReadFile(exePath)
	assert.Nil(t, readErr)
	assert.Eq(t, "new-content", string(got))

	saved, err := os.Stat(exePath + ".old")
	assert.Nil(t, err)
	assert.True(t, saved.Size() > 0)
}

func copyFixture(t *testing.T, src, dst string) {
	t.Helper()
	in, err := os.Open(src)
	assert.Nil(t, err)
	defer in.Close()
	info, err := in.Stat()
	assert.Nil(t, err)
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, info.Mode().Perm())
	assert.Nil(t, err)
	n, err := io.Copy(out, in)
	assert.Nil(t, err)
	assert.Nil(t, out.Close())
	assert.Eq(t, info.Size(), n)
}

// waitForImageLock waits until the fixture process holds the image so that
// opening for write fails (that is the failure state being fixed).
func waitForImageLock(t *testing.T, exePath string) {
	t.Helper()
	for i := 0; i < 50; i++ {
		out, err := os.OpenFile(exePath, os.O_WRONLY, 0o666)
		if err != nil {
			return
		}
		assert.Nil(t, out.Close())
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("fixture process never locked the executable image")
}
