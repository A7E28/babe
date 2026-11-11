//go:build linux

package platform

import "fmt"

func initConsolePlatform() error {
	return nil
}

func clearConsolePlatform() {
	fmt.Print("\x1b[2J\x1b[3J\x1b[H")
}
