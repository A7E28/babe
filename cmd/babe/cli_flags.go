package main

import "strings"

// isHelpArg returns true if the argument requests command help.
func isHelpArg(arg string) bool {
	return arg == "-h" || arg == "--help" || arg == "help" || arg == "-help"
}

// reorderFlags moves flag tokens before positional arguments
func reorderFlags(args []string, boolFlags map[string]bool) []string {
	var flags, pos []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if strings.HasPrefix(arg, "-") {
			flags = append(flags, arg)
			if !strings.Contains(arg, "=") && !boolFlags[arg] && i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
				flags = append(flags, args[i+1])
				i++
			}
		} else {
			pos = append(pos, arg)
		}
	}
	return append(flags, pos...)
}
