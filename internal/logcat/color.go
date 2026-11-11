package logcat

import (
	"fmt"

	"github.com/A7E28/babe/internal/platform"
)

const (
	ColorReset   = "\x1b[0m"
	ColorGray    = "\x1b[90m"
	ColorCyan    = "\x1b[36m"
	ColorGreen   = "\x1b[32m"
	ColorYellow  = "\x1b[33m"
	ColorRed     = "\x1b[31m"
	ColorBoldRed = "\x1b[1;31m"
)

// returns the ANSI escape code for a given log level.
func LevelColor(lvl Level) string {
	switch lvl {
	case LevelVerbose:
		return ColorGray
	case LevelDebug:
		return ColorCyan
	case LevelInfo:
		return ColorGreen
	case LevelWarn:
		return ColorYellow
	case LevelError:
		return ColorRed
	case LevelFatal:
		return ColorBoldRed
	default:
		return ColorReset
	}
}

// returns a formatted string for an entry, applying ANSI colors when supported.
func Colorize(entry Entry) string {
	useColor := platform.ColorsSupported()

	if entry.Time != "" && entry.Tag != "" {
		if !useColor {
			return fmt.Sprintf("%s %-1.1s/%-16.16s [%s] %s",
				entry.Time,
				entry.Level.String(),
				entry.Tag,
				entry.PID,
				entry.Message,
			)
		}
		color := LevelColor(entry.Level)
		return fmt.Sprintf("%s%s %-1.1s/%-16.16s [%s]%s %s",
			color,
			entry.Time,
			entry.Level.String(),
			entry.Tag,
			entry.PID,
			ColorReset,
			entry.Message,
		)
	}

	if !useColor {
		return entry.Raw
	}
	return LevelColor(entry.Level) + entry.Raw + ColorReset
}
