//go:build !windows

package config

import "os"

func platformReplaceConfigFile(source, target string) error {
	return os.Rename(source, target)
}
