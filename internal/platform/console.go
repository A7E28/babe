package platform

import (
	"os"
	"strings"
)

// initializes platform-specific terminal features (e.g. Virtual Terminal on Windows).
func InitConsole() error {
	return initConsolePlatform()
}

// completely clears both visible viewport and scrollback buffer.
func ClearConsole() {
	clearConsolePlatform()
}

// returns true if the current environment supports ANSI coloring.
func ColorsSupported() bool {
	if _, ok := os.LookupEnv("NO_COLOR"); ok {
		return false
	}
	term := os.Getenv("TERM")
	if strings.EqualFold(term, "dumb") {
		return false
	}
	return true
}
