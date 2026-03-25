package changelog

import (
	"strings"
	"testing"
	"time"

	gogitobj "github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing"
)

func TestGroupCommitsByType(t *testing.T) {
	t.Parallel()
	commits := []*gogitobj.Commit{
		{Hash: plumbing.NewHash("aaa"), Message: "feat: add login"},
		{Hash: plumbing.NewHash("bbb"), Message: "fix(auth): resolve null pointer"},
		{Hash: plumbing.NewHash("ccc"), Message: "feat(ui): new dashboard"},
		{Hash: plumbing.NewHash("ddd"), Message: "docs: update README"},
		{Hash: plumbing.NewHash("eee"), Message: "no conventional prefix"},
		{Hash: plumbing.NewHash("fff"), Message: "chore: update deps"},
	}

	grouped := GroupCommitsByType(commits)

	if len(grouped["feat"]) != 2 {
		t.Errorf("expected 2 feat commits, got %d", len(grouped["feat"]))
	}
	if len(grouped["fix"]) != 1 {
		t.Errorf("expected 1 fix commit, got %d", len(grouped["fix"]))
	}
	if len(grouped["docs"]) != 1 {
		t.Errorf("expected 1 docs commit, got %d", len(grouped["docs"]))
	}
	if len(grouped["other"]) != 1 {
		t.Errorf("expected 1 other commit, got %d", len(grouped["other"]))
	}
	if len(grouped["chore"]) != 1 {
		t.Errorf("expected 1 chore commit, got %d", len(grouped["chore"]))
	}
}

func TestGroupCommitsByType_EmptyList(t *testing.T) {
	t.Parallel()
	grouped := GroupCommitsByType(nil)
	if len(grouped) != 0 {
		t.Errorf("expected empty map, got %d entries", len(grouped))
	}
}

func TestGroupCommitsByType_AllSameType(t *testing.T) {
	t.Parallel()
	commits := []*gogitobj.Commit{
		{Hash: plumbing.NewHash("aaa"), Message: "fix: bug 1"},
		{Hash: plumbing.NewHash("bbb"), Message: "fix: bug 2"},
		{Hash: plumbing.NewHash("ccc"), Message: "fix(api): bug 3"},
	}
	grouped := GroupCommitsByType(commits)
	if len(grouped["fix"]) != 3 {
		t.Errorf("expected 3 fix commits, got %d", len(grouped["fix"]))
	}
	if len(grouped) != 1 {
		t.Errorf("expected 1 group, got %d", len(grouped))
	}
}

func TestGroupCommitsByType_CaseInsensitive(t *testing.T) {
	t.Parallel()
	commits := []*gogitobj.Commit{
		{Hash: plumbing.NewHash("aaa"), Message: "Feat: uppercase type"},
		{Hash: plumbing.NewHash("bbb"), Message: "CHORE: all caps"},
	}
	grouped := GroupCommitsByType(commits)
	if len(grouped["feat"]) != 1 {
		t.Errorf("expected 1 feat commit, got %d", len(grouped["feat"]))
	}
	if len(grouped["chore"]) != 1 {
		t.Errorf("expected 1 chore commit, got %d", len(grouped["chore"]))
	}
}

func TestFormatGroupedCommits(t *testing.T) {
	t.Parallel()
	commits := []*gogitobj.Commit{
		{Hash: plumbing.NewHash("aaaaaaa"), Message: "feat: add login"},
		{Hash: plumbing.NewHash("bbbbbbb"), Message: "fix: resolve crash"},
	}
	grouped := GroupCommitsByType(commits)
	result := formatGroupedCommits(grouped)

	if !strings.Contains(result, "### feat") {
		t.Error("expected feat section header")
	}
	if !strings.Contains(result, "### fix") {
		t.Error("expected fix section header")
	}
	if !strings.Contains(result, "feat: add login") {
		t.Error("expected feat commit message")
	}
}

func TestFormatGroupedCommits_DeterministicOrder(t *testing.T) {
	t.Parallel()
	commits := []*gogitobj.Commit{
		{Hash: plumbing.NewHash("aaa"), Message: "chore: deps"},
		{Hash: plumbing.NewHash("bbb"), Message: "feat: feature"},
		{Hash: plumbing.NewHash("ccc"), Message: "fix: bugfix"},
	}
	grouped := GroupCommitsByType(commits)
	result := formatGroupedCommits(grouped)

	featIdx := strings.Index(result, "### feat")
	fixIdx := strings.Index(result, "### fix")
	choreIdx := strings.Index(result, "### chore")

	if featIdx > fixIdx {
		t.Error("feat should come before fix")
	}
	if fixIdx > choreIdx {
		t.Error("fix should come before chore")
	}
}

func TestParseSince(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		input   string
		wantErr bool
		check   func(t time.Time) bool
	}{
		{
			name:  "2 weeks ago",
			input: "2 weeks ago",
			check: func(t time.Time) bool {
				expected := time.Now().AddDate(0, 0, -14)
				return t.Sub(expected).Abs() < time.Minute
			},
		},
		{
			name:  "3 days ago",
			input: "3 days ago",
			check: func(t time.Time) bool {
				expected := time.Now().AddDate(0, 0, -3)
				return t.Sub(expected).Abs() < time.Minute
			},
		},
		{
			name:  "1 month ago",
			input: "1 month ago",
			check: func(t time.Time) bool {
				expected := time.Now().AddDate(0, -1, 0)
				return t.Sub(expected).Abs() < time.Minute
			},
		},
		{
			name:  "6 hours ago",
			input: "6 hours ago",
			check: func(t time.Time) bool {
				expected := time.Now().Add(-6 * time.Hour)
				return t.Sub(expected).Abs() < time.Minute
			},
		},
		{
			name:  "1 year ago",
			input: "1 year ago",
			check: func(t time.Time) bool {
				expected := time.Now().AddDate(-1, 0, 0)
				return t.Sub(expected).Abs() < time.Minute
			},
		},
		{
			name:    "invalid format",
			input:   "yesterday",
			wantErr: true,
		},
		{
			name:    "empty string",
			input:   "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result, err := ParseSince(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !tt.check(result) {
				t.Errorf("time %v did not match expectation for %q", result, tt.input)
			}
		})
	}
}
