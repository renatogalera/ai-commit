package git

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	gogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/renatogalera/ai-commit/pkg/committypes"
	"github.com/renatogalera/ai-commit/pkg/config"
)

func init() {
	committypes.InitCommitTypes([]config.CommitTypeConfig{
		{Type: "feat", Emoji: "✨"},
		{Type: "fix", Emoji: "🐛"},
		{Type: "docs", Emoji: "📚"},
		{Type: "style", Emoji: "💎"},
		{Type: "refactor", Emoji: "♻️"},
		{Type: "test", Emoji: "🧪"},
		{Type: "chore", Emoji: "🔧"},
		{Type: "perf", Emoji: "🚀"},
		{Type: "build", Emoji: "📦"},
		{Type: "ci", Emoji: "👷"},
	})
}

// --- Pure function tests ---

func TestPrependCommitType(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		message   string
		typ       string
		withEmoji bool
		want      string
	}{
		{
			name:    "empty type returns message as-is",
			message: "add login",
			typ:     "",
			want:    "add login",
		},
		{
			name:    "prepends type without emoji",
			message: "add login feature",
			typ:     "feat",
			want:    "feat: add login feature",
		},
		{
			name:      "prepends type with emoji",
			message:   "add login feature",
			typ:       "feat",
			withEmoji: true,
			want:      "✨ feat: add login feature",
		},
		{
			name:    "strips existing type prefix before prepending",
			message: "fix: resolve null pointer",
			typ:     "feat",
			want:    "feat: resolve null pointer",
		},
		{
			name:      "strips existing emoji+type and re-adds",
			message:   "🐛 fix: resolve bug",
			typ:       "feat",
			withEmoji: true,
			want:      "✨ feat: resolve bug",
		},
		{
			name:    "strips type with scope",
			message: "feat(auth): add oauth",
			typ:     "fix",
			want:    "fix: add oauth",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := PrependCommitType(tt.message, tt.typ, tt.withEmoji)
			if got != tt.want {
				t.Errorf("PrependCommitType(%q, %q, %v) = %q, want %q",
					tt.message, tt.typ, tt.withEmoji, got, tt.want)
			}
		})
	}
}

func TestAddGitmoji(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		message string
		typ     string
		want    string
	}{
		{
			name:    "empty type returns message",
			message: "some change",
			typ:     "",
			want:    "some change",
		},
		{
			name:    "adds emoji for feat",
			message: "add login",
			typ:     "feat",
			want:    "✨ feat: add login",
		},
		{
			name:    "adds emoji for fix",
			message: "resolve crash",
			typ:     "fix",
			want:    "🐛 fix: resolve crash",
		},
		{
			name:    "replaces existing emoji prefix",
			message: "🐛 fix: old message",
			typ:     "feat",
			want:    "✨ feat: old message",
		},
		{
			name:    "type with no emoji configured",
			message: "something",
			typ:     "unknown",
			want:    "unknown: something",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := AddGitmoji(tt.message, tt.typ)
			if got != tt.want {
				t.Errorf("AddGitmoji(%q, %q) = %q, want %q",
					tt.message, tt.typ, got, tt.want)
			}
		})
	}
}

func TestParseFilePath(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		line string
		want string
	}{
		{
			name: "standard diff header",
			line: "diff --git a/pkg/git/git.go b/pkg/git/git.go",
			want: "pkg/git/git.go",
		},
		{
			name: "renamed file returns b path",
			line: "diff --git a/old/path.go b/new/path.go",
			want: "new/path.go",
		},
		{
			name: "too few parts",
			line: "diff --git",
			want: "",
		},
		{
			name: "root level file",
			line: "diff --git a/main.go b/main.go",
			want: "main.go",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := parseFilePath(tt.line)
			if got != tt.want {
				t.Errorf("parseFilePath(%q) = %q, want %q", tt.line, got, tt.want)
			}
		})
	}
}

func TestParseDiffToChunks(t *testing.T) {
	t.Parallel()

	t.Run("single file single hunk", func(t *testing.T) {
		t.Parallel()
		diff := `diff --git a/main.go b/main.go
@@ -1,3 +1,4 @@
 package main
+import "fmt"
 func main() {}`

		chunks, err := ParseDiffToChunks(diff)
		if err != nil {
			t.Fatal(err)
		}
		if len(chunks) != 1 {
			t.Fatalf("expected 1 chunk, got %d", len(chunks))
		}
		if chunks[0].FilePath != "main.go" {
			t.Errorf("FilePath = %q, want main.go", chunks[0].FilePath)
		}
		if len(chunks[0].Lines) != 3 {
			t.Errorf("expected 3 lines, got %d", len(chunks[0].Lines))
		}
	})

	t.Run("multiple files multiple hunks", func(t *testing.T) {
		t.Parallel()
		diff := `diff --git a/a.go b/a.go
@@ -1,2 +1,3 @@
+line1
 line2
@@ -10,2 +11,3 @@
+line3
 line4
diff --git a/b.go b/b.go
@@ -1,1 +1,2 @@
+new line`

		chunks, err := ParseDiffToChunks(diff)
		if err != nil {
			t.Fatal(err)
		}
		if len(chunks) != 3 {
			t.Fatalf("expected 3 chunks, got %d", len(chunks))
		}
		if chunks[0].FilePath != "a.go" || chunks[1].FilePath != "a.go" || chunks[2].FilePath != "b.go" {
			t.Error("unexpected file paths")
		}
	})

	t.Run("empty diff", func(t *testing.T) {
		t.Parallel()
		chunks, err := ParseDiffToChunks("")
		if err != nil {
			t.Fatal(err)
		}
		if len(chunks) != 0 {
			t.Errorf("expected 0 chunks, got %d", len(chunks))
		}
	})
}

func TestFilterLockFiles(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		diff      string
		lockFiles []string
		wantParts []string
		noParts   []string
	}{
		{
			name: "filters go.sum",
			diff: `diff --git a/main.go b/main.go
@@ -1,2 +1,3 @@
+import "fmt"
diff --git a/go.sum b/go.sum
@@ -1,100 +1,101 @@
+hash123`,
			lockFiles: []string{"go.sum"},
			wantParts: []string{"main.go"},
			noParts:   []string{"go.sum", "hash123"},
		},
		{
			name: "filters multiple lock files",
			diff: `diff --git a/main.go b/main.go
+code
diff --git a/go.mod b/go.mod
+module
diff --git a/package-lock.json b/package-lock.json
+lockdata`,
			lockFiles: []string{"go.mod", "package-lock.json"},
			wantParts: []string{"main.go"},
			noParts:   []string{"go.mod", "package-lock.json"},
		},
		{
			name:      "no lock files returns unchanged",
			diff:      "diff --git a/main.go b/main.go\n+code",
			lockFiles: nil,
			wantParts: []string{"main.go", "+code"},
		},
		{
			name: "nested path lock file",
			diff: `diff --git a/vendor/go.sum b/vendor/go.sum
+hash`,
			lockFiles: []string{"go.sum"},
			noParts:   []string{"go.sum"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := FilterLockFiles(tt.diff, tt.lockFiles)
			for _, p := range tt.wantParts {
				if !strings.Contains(got, p) {
					t.Errorf("expected %q in result, got:\n%s", p, got)
				}
			}
			for _, p := range tt.noParts {
				if strings.Contains(got, p) {
					t.Errorf("expected %q NOT in result, got:\n%s", p, got)
				}
			}
		})
	}
}

func TestIsCommentOnlyChange(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		line string
		want bool
	}{
		{"C-style comment add", "+// this is a comment", true},
		{"C-style comment remove", "-// removed comment", true},
		{"block comment", "+/* block comment */", true},
		{"star continuation", "+* continued block", true},
		{"hash comment", "+# python comment", true},
		{"SQL comment", "+-- sql comment", true},
		{"HTML comment", "+<!-- html comment -->", true},
		{"code change", "+fmt.Println(\"hello\")", false},
		{"context line", " unchanged line", false},
		{"empty line", "", false},
		{"just plus", "+", false},
		{"semicolon comment", "+; lisp comment", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := isCommentOnlyChange(tt.line)
			if got != tt.want {
				t.Errorf("isCommentOnlyChange(%q) = %v, want %v", tt.line, got, tt.want)
			}
		})
	}
}

func TestIsPureMovement(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		lines []string
		idx   int
		want  bool
	}{
		{
			name:  "matching delete-add pair",
			lines: []string{"-func foo() {}", "+func foo() {}"},
			idx:   0,
			want:  true,
		},
		{
			name:  "different content",
			lines: []string{"-func foo() {}", "+func bar() {}"},
			idx:   0,
			want:  false,
		},
		{
			name:  "both additions",
			lines: []string{"+line1", "+line2"},
			idx:   0,
			want:  false,
		},
		{
			name:  "last line",
			lines: []string{"-only line"},
			idx:   0,
			want:  false,
		},
		{
			name:  "empty trimmed content",
			lines: []string{"-  ", "+  "},
			idx:   0,
			want:  false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := isPureMovement(tt.lines, tt.idx)
			if got != tt.want {
				t.Errorf("isPureMovement(%v, %d) = %v, want %v",
					tt.lines, tt.idx, got, tt.want)
			}
		})
	}
}

func TestIsBinary(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		data []byte
		want bool
	}{
		{"text content", []byte("hello world\nfoo bar"), false},
		{"go source", []byte("package main\nfunc main() {}"), false},
		{"PNG header", []byte("\x89PNG\r\n\x1a\n"), true},
		{"empty is not binary", []byte(""), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := isBinary(tt.data)
			if got != tt.want {
				t.Errorf("isBinary(%v) = %v, want %v", tt.name, got, tt.want)
			}
		})
	}
}

// --- Integration tests (require temp git repos) ---

func initTestRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	repo, err := gogit.PlainInit(dir, false)
	if err != nil {
		t.Fatal(err)
	}

	// Create an initial file and commit
	testFile := filepath.Join(dir, "README.md")
	if err := os.WriteFile(testFile, []byte("# Test\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	wt, err := repo.Worktree()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := wt.Add("README.md"); err != nil {
		t.Fatal(err)
	}
	_, err = wt.Commit("initial commit", &gogit.CommitOptions{
		Author: &object.Signature{
			Name:  "Test",
			Email: "test@example.com",
			When:  time.Now(),
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	return dir
}

// Integration tests use os.Chdir which is process-global,
// so they cannot run in parallel.

func TestIsGitRepository_Integration(t *testing.T) {
	t.Run("valid repo root", func(t *testing.T) {
		dir := initTestRepo(t)
		origDir, _ := os.Getwd()
		if err := os.Chdir(dir); err != nil {
			t.Fatal(err)
		}
		defer os.Chdir(origDir)

		if !IsGitRepository(context.Background()) {
			t.Error("expected true for valid git repo")
		}
	})

	t.Run("subdirectory of repo", func(t *testing.T) {
		dir := initTestRepo(t)
		subdir := filepath.Join(dir, "sub", "deep")
		if err := os.MkdirAll(subdir, 0o755); err != nil {
			t.Fatal(err)
		}

		origDir, _ := os.Getwd()
		if err := os.Chdir(subdir); err != nil {
			t.Fatal(err)
		}
		defer os.Chdir(origDir)

		if !IsGitRepository(context.Background()) {
			t.Error("expected true for subdirectory of git repo (issue #4 fix)")
		}
	})

	t.Run("non-repo directory", func(t *testing.T) {
		dir := t.TempDir()
		origDir, _ := os.Getwd()
		if err := os.Chdir(dir); err != nil {
			t.Fatal(err)
		}
		defer os.Chdir(origDir)

		if IsGitRepository(context.Background()) {
			t.Error("expected false for non-git directory")
		}
	})
}

func TestGetHeadCommitMessage_Integration(t *testing.T) {
	dir := initTestRepo(t)
	origDir, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(origDir)

	msg, err := GetHeadCommitMessage(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if msg != "initial commit" {
		t.Errorf("got %q, want 'initial commit'", msg)
	}
}

func TestGetCurrentBranch_Integration(t *testing.T) {
	dir := initTestRepo(t)
	origDir, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(origDir)

	branch, err := GetCurrentBranch(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	// Default branch could be master or main depending on git config
	if branch != "master" && branch != "main" {
		t.Errorf("got branch %q, expected master or main", branch)
	}
}

func TestCommitChanges_Integration(t *testing.T) {
	dir := initTestRepo(t)
	origDir, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(origDir)

	// Stage a new file
	newFile := filepath.Join(dir, "new.txt")
	if err := os.WriteFile(newFile, []byte("new content"), 0o644); err != nil {
		t.Fatal(err)
	}

	repo, err := gogit.PlainOpen(dir)
	if err != nil {
		t.Fatal(err)
	}
	wt, err := repo.Worktree()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := wt.Add("new.txt"); err != nil {
		t.Fatal(err)
	}

	err = CommitChanges(context.Background(), "feat: add new file")
	if err != nil {
		t.Fatal(err)
	}

	msg, err := GetHeadCommitMessage(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if msg != "feat: add new file" {
		t.Errorf("got %q, want 'feat: add new file'", msg)
	}
}
