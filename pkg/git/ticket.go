package git

import "regexp"

// DefaultTicketPatterns are tried in order to extract ticket IDs from branch names.
var DefaultTicketPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)((?:[A-Z]{2,10})-\d+)`),   // JIRA/Linear/generic: PROJ-123, ENG-456
	regexp.MustCompile(`(?i)(GH-\d+)`),                 // GitHub alternative: GH-123
	regexp.MustCompile(`(?:^|[/\-_])#(\d+)(?:$|[/\-_])`), // GitHub issue: #123 in path segments
}

// ExtractTicketID scans a branch name for a ticket identifier.
// If customPattern is non-empty, it is compiled and tried first.
// Returns the matched ticket string, or "" if none found.
func ExtractTicketID(branchName, customPattern string) string {
	if branchName == "" {
		return ""
	}

	// Try custom pattern first
	if customPattern != "" {
		re, err := regexp.Compile(customPattern)
		if err == nil {
			match := re.FindStringSubmatch(branchName)
			if len(match) > 1 {
				return match[1]
			}
			if len(match) == 1 {
				return match[0]
			}
		}
	}

	// Try default patterns
	for i, re := range DefaultTicketPatterns {
		match := re.FindStringSubmatch(branchName)
		if len(match) > 1 {
			// For the GitHub #N pattern, prepend #
			if i == 2 {
				return "#" + match[1]
			}
			return match[1]
		}
	}

	return ""
}
