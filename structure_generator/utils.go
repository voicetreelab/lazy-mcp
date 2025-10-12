package structure_generator

import "strings"

// ParseToolPath splits a dot-notation tool path into its component parts
// Example: "coding_tools.serena.find_symbol" -> ["coding_tools", "serena", "find_symbol"]
func ParseToolPath(path string) []string {
	if path == "" {
		return []string{}
	}
	return strings.Split(path, ".")
}
