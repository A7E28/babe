package platform

import (
	"os"
	"strings"
)

func InitConsole() error {
	return initConsolePlatform()
}

func ClearConsole() {
	clearConsolePlatform()
}

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
