package platform

import (
	"fmt"
	"strings"
)

// defines the cross-platform operations for system path and UI dialogs.
type PlatformManager interface {
	GetDefaultInstallDir() (string, error)
	GetBabeBinDir() (string, error)
	IsInPath(dir string) (bool, error)
	AddToPath(dir string) error
	RemoveFromPath(dir string) error
	OpenFilePicker(title string) (string, error)
}

// returns the OS-specific implementation of PlatformManager.
func DefaultManager() PlatformManager {
	return defaultManager()
}

// splits a PATH environment string by delimiter and cleans whitespace.
func CleanPathString(pathStr string, delimiter rune) []string {
	parts := strings.Split(pathStr, string(delimiter))
	var cleaned []string
	for _, p := range parts {
		trimmed := strings.TrimSpace(p)
		if trimmed != "" {
			cleaned = append(cleaned, trimmed)
		}
	}
	return cleaned
}

// joins path segments with the given delimiter.
func FormatPathString(segments []string, delimiter rune) string {
	return strings.Join(segments, string(delimiter))
}

func normalizePathForCompare(p string) string {
	trimmed := strings.TrimSpace(p)
	if len(trimmed) > 1 {
		trimmed = strings.TrimRight(trimmed, `\/`)
	}
	return trimmed
}

// checks if a path list contains a directory (case-insensitive on Windows).
func ContainsPath(list []string, target string, caseInsensitive bool) bool {
	normTarget := normalizePathForCompare(target)
	for _, p := range list {
		normP := normalizePathForCompare(p)
		if caseInsensitive {
			if strings.EqualFold(normP, normTarget) {
				return true
			}
		} else {
			if normP == normTarget {
				return true
			}
		}
	}
	return false
}

// removes a target directory from a slice of paths.
func FilterOutPath(list []string, target string, caseInsensitive bool) []string {
	normTarget := normalizePathForCompare(target)
	var result []string
	for _, p := range list {
		normP := normalizePathForCompare(p)
		match := false
		if caseInsensitive {
			match = strings.EqualFold(normP, normTarget)
		} else {
			match = (normP == normTarget)
		}
		if !match {
			result = append(result, p)
		}
	}
	return result
}

var ErrPathAlreadyExists = fmt.Errorf("directory is already in PATH")
