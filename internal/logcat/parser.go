package logcat

import (
	"strings"
)

type Level int

const (
	LevelVerbose Level = iota
	LevelDebug
	LevelInfo
	LevelWarn
	LevelError
	LevelFatal
)

// returns the single-letter logcat tag.
func (l Level) String() string {
	switch l {
	case LevelVerbose:
		return "V"
	case LevelDebug:
		return "D"
	case LevelInfo:
		return "I"
	case LevelWarn:
		return "W"
	case LevelError:
		return "E"
	case LevelFatal:
		return "F"
	default:
		return "?"
	}
}

// converts a string into a Level, defaulting to LevelVerbose.
func ParseLevel(s string) Level {
	lvl, err := ValidateLevel(s)
	if err != nil {
		return LevelVerbose
	}
	return lvl
}

// parses and validates a logcat level string (V, D, I, W, E, F).
func ValidateLevel(s string) (Level, error) {
	switch strings.ToUpper(strings.TrimSpace(s)) {
	case "", "V", "VERBOSE":
		return LevelVerbose, nil
	case "D", "DEBUG":
		return LevelDebug, nil
	case "I", "INFO":
		return LevelInfo, nil
	case "W", "WARN", "WARNING":
		return LevelWarn, nil
	case "E", "ERR", "ERROR":
		return LevelError, nil
	case "F", "FATAL":
		return LevelFatal, nil
	default:
		return LevelVerbose, &InvalidLevelError{Input: s}
	}
}

type InvalidLevelError struct {
	Input string
}

func (e *InvalidLevelError) Error() string {
	return "invalid log level \"" + e.Input + "\" (valid: V, D, I, W, E, F)"
}

// represents a parsed logcat log record.
type Entry struct {
	Raw     string
	Time    string
	Level   Level
	PID     string
	Tag     string
	Message string
}

// parses a standard "-v time" logcat line:
// format: "10-31 00:15:32.123 D/Tag( 1234): Message"
// or:     "10-31 00:15:32.123  1234  5678 D Tag     : Message"
func ParseLine(raw string) Entry {
	entry := Entry{Raw: raw, Level: LevelVerbose}
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return entry
	}

	parts := strings.SplitN(trimmed, " ", 3)
	if len(parts) >= 3 && len(parts[0]) == 5 && strings.Contains(parts[0], "-") {
		entry.Time = parts[0] + " " + parts[1]
		rest := parts[2]

		if len(rest) >= 2 && rest[1] == '/' {
			entry.Level = ParseLevel(string(rest[0]))
			tagPart := rest[2:]

			colonIdx := strings.Index(tagPart, ":")
			if colonIdx != -1 {
				header := tagPart[:colonIdx]
				entry.Message = strings.TrimSpace(tagPart[colonIdx+1:])

				if openParen := strings.Index(header, "("); openParen != -1 {
					closeParen := strings.Index(header, ")")
					if closeParen > openParen {
						entry.Tag = strings.TrimSpace(header[:openParen])
						entry.PID = strings.TrimSpace(header[openParen+1 : closeParen])
					} else {
						entry.Tag = strings.TrimSpace(header)
					}
				} else {
					entry.Tag = strings.TrimSpace(header)
				}
				return entry
			}
		}

		fields := strings.Fields(rest)
		if len(fields) >= 4 && len(fields[2]) == 1 {
			colonIdx := strings.Index(rest, ":")
			if colonIdx != -1 {
				headerFields := strings.Fields(rest[:colonIdx])
				if len(headerFields) >= 4 && len(headerFields[2]) == 1 {
					entry.PID = headerFields[0]
					entry.Level = ParseLevel(headerFields[2])
					entry.Tag = strings.Join(headerFields[3:], " ")
					entry.Message = strings.TrimSpace(rest[colonIdx+1:])
					return entry
				}
			}
		}
	}

	entry.Message = trimmed
	return entry
}
