package git

import "testing"

func TestExtractTicketID(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name          string
		branch        string
		customPattern string
		want          string
	}{
		// JIRA-style
		{"JIRA ticket", "feature/PROJ-456-add-login", "", "PROJ-456"},
		{"JIRA at start", "PROJ-123-fix-bug", "", "PROJ-123"},
		{"Linear ticket", "fix/ENG-789-resolve-issue", "", "ENG-789"},
		{"short project key", "AB-12-minor", "", "AB-12"},

		// GitHub GH-style
		{"GH prefix", "feature/GH-789-update", "", "GH-789"},
		{"GH lowercase", "fix/gh-42-hotfix", "", "gh-42"},

		// GitHub #N style
		{"hash number in path", "feature/#123-add-feature", "", "#123"},
		{"hash number with dash", "fix-#42-crash", "", "#42"},
		{"hash number with underscore", "feat_#99_new", "", "#99"},

		// No match
		{"plain branch", "main", "", ""},
		{"no ticket", "feature/add-new-feature", "", ""},
		{"empty branch", "", "", ""},
		{"just numbers", "feature/123", "", ""},

		// Custom pattern
		{"custom pattern match", "dev/CUSTOM-999-thing", `(CUSTOM-\d+)`, "CUSTOM-999"},
		{"custom no match falls to default", "feature/PROJ-123", `(NOMATCH-\d+)`, "PROJ-123"},
		{"custom invalid regex falls to default", "feature/PROJ-456", `([invalid`, "PROJ-456"},
		{"custom no capture group", "dev/TICKET999", `TICKET\d+`, "TICKET999"},

		// Edge cases
		{"ticket at end", "add-login-PROJ-789", "", "PROJ-789"},
		{"multiple tickets picks first", "PROJ-1-and-PROJ-2", "", "PROJ-1"},
		{"case insensitive JIRA", "feature/proj-123-lower", "", "proj-123"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := ExtractTicketID(tt.branch, tt.customPattern)
			if got != tt.want {
				t.Errorf("ExtractTicketID(%q, %q) = %q, want %q",
					tt.branch, tt.customPattern, got, tt.want)
			}
		})
	}
}
