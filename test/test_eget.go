package main

import (
	"fmt"
	"os"
	"os/exec"
)

func fileExists(path string) error {
	_, err := os.Stat(path)
	return err
}

func run(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

func runWithEnv(env []string, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = append(os.Environ(), env...)

	return cmd.Run()
}

func must(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func main() {
	eget := os.Getenv("TEST_EGET")
	configInitPath := "tmp.eget.toml"
	localSource := "../LICENSE"

	must(run(eget, "install", "--to", "install-license.txt", localSource))
	must(fileExists("install-license.txt"))

	must(run(eget, "download", "--to", "LICENSE.txt", localSource))
	must(fileExists("LICENSE.txt"))

	must(run(eget, "config", "--info"))
	must(runWithEnv([]string{"EGET_CONFIG=" + configInitPath}, eget, "config", "--init"))
	must(fileExists(configInitPath))

	fmt.Println("ALL TESTS PASS")
}
