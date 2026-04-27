//go:build windows

package install

import "golang.org/x/sys/windows"

type shellExecuteFunc func(verb, file, args string) error

var shellExecute shellExecuteFunc = func(verb, file, args string) error {
	verbPtr, err := windows.UTF16PtrFromString(verb)
	if err != nil {
		return err
	}
	filePtr, err := windows.UTF16PtrFromString(file)
	if err != nil {
		return err
	}
	argsPtr, err := windows.UTF16PtrFromString(args)
	if err != nil {
		return err
	}
	return windows.ShellExecute(0, verbPtr, filePtr, argsPtr, nil, windows.SW_SHOWNORMAL)
}

func launchWindowsInstaller(path string, kind InstallerKind) error {
	file, args, err := windowsInstallerCommand(path, kind)
	if err != nil {
		return err
	}
	return shellExecute("runas", file, args)
}
