package config

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

const UserConfigDirectoryNotFoundErrorMessage = "user config directory not found"

var UserConfigDirectory = func() (string, error) {
	return os.UserConfigDir()
}

var ExecCommand = func(name string, arg ...string) ([]byte, error) {
	return exec.Command(name, arg...).Output()
}

var WslInteropFile = "/proc/sys/fs/binfmt_misc/WSLInterop"

func RunningInWSL() bool {
	if runtime.GOOS != "linux" {
		return false
	}
	_, err := os.Stat(WslInteropFile)
	return err == nil
}

func ObsidianFile() (string, error) {
	configDir, err := UserConfigDirectory()
	if err != nil {
		return "", errors.New(UserConfigDirectoryNotFoundErrorMessage)
	}

	if runtime.GOOS == "linux" {
		if RunningInWSL() {
			out, err := ExecCommand("cmd.exe", "/C", "echo %APPDATA%")
			if err == nil {
				appData := strings.TrimSpace(strings.TrimRight(string(out), "\r\n"))
				if len(appData) >= 2 && appData[1] == ':' {
					driveLetter := strings.ToLower(string(appData[0]))
					path := "/mnt/" + driveLetter + strings.ReplaceAll(appData[2:], "\\", "/")
					return filepath.Join(path, "obsidian", "obsidian.json"), nil
				}
			}
		}

		home, _ := os.UserHomeDir()

		// Native Obsidian (highest priority when it exists)
		nativePath := filepath.Join(configDir, "obsidian", "obsidian.json")
		if _, err := os.Stat(nativePath); err == nil {
			return nativePath, nil
		}

		// Flatpak
		flatpakPath := filepath.Join(home, ".var", "app", "md.obsidian.Obsidian", "config", "obsidian", "obsidian.json")
		if _, err := os.Stat(flatpakPath); err == nil {
			return flatpakPath, nil
		}

		// Snap
		snapPath := filepath.Join(home, "snap", "obsidian", "current", ".config", "obsidian", "obsidian.json")
		if _, err := os.Stat(snapPath); err == nil {
			return snapPath, nil
		}

		// Fall back to native path even if it doesn't exist
		return nativePath, nil
	}

	return filepath.Join(configDir, "obsidian", "obsidian.json"), nil
}

func CliPath() (string, string, error) {
	configDir, err := UserConfigDirectory()
	if err != nil {
		return "", "", errors.New(UserConfigDirectoryNotFoundErrorMessage)
	}
	dir := filepath.Join(configDir, "notesmd-cli")
	file := filepath.Join(dir, "config.json")
	return dir, file, nil
}
