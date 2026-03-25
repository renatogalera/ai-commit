package changelog

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	gogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	gogitobj "github.com/go-git/go-git/v5/plumbing/object"
	"golang.org/x/mod/semver"

	"github.com/renatogalera/ai-commit/pkg/ai"
	"github.com/renatogalera/ai-commit/pkg/config"
	"github.com/renatogalera/ai-commit/pkg/prompt"
)

// Options controls changelog generation behavior.
type Options struct {
	FromRef string // e.g. "v0.10.0"
	ToRef   string // e.g. "v0.11.0"
	Since   string // e.g. "2 weeks ago"
}

// Generate produces a markdown changelog for commits in the given range.
func Generate(ctx context.Context, aiClient ai.AIClient, cfg *config.Config, language string, opts Options) (string, error) {
	repo, err := gogit.PlainOpenWithOptions(".", &gogit.PlainOpenOptions{DetectDotGit: true})
	if err != nil {
		return "", fmt.Errorf("failed to open repository: %w", err)
	}

	fromRef, toRef, err := resolveRange(repo, opts)
	if err != nil {
		return "", err
	}

	var commits []*gogitobj.Commit
	if opts.Since != "" {
		sinceTime, err := ParseSince(opts.Since)
		if err != nil {
			return "", err
		}
		commits, err = collectCommitsSince(repo, sinceTime)
		if err != nil {
			return "", err
		}
	} else {
		toHash, err := resolveRef(repo, toRef)
		if err != nil {
			return "", fmt.Errorf("cannot resolve %q: %w", toRef, err)
		}
		fromHash, err := resolveRef(repo, fromRef)
		if err != nil {
			return "", fmt.Errorf("cannot resolve %q: %w", fromRef, err)
		}
		commits, err = collectCommitsBetween(repo, fromHash, toHash)
		if err != nil {
			return "", err
		}
	}

	if len(commits) == 0 {
		return "", fmt.Errorf("no commits found in range %s..%s", fromRef, toRef)
	}

	grouped := GroupCommitsByType(commits)
	commitData := formatGroupedCommits(grouped)

	changelogPrompt := prompt.BuildChangelogPrompt(commitData, fromRef, toRef, language, cfg.PromptTemplate)
	if cfg.Limits.Prompt.Enabled && cfg.Limits.Prompt.MaxChars > 0 {
		if len(changelogPrompt) > cfg.Limits.Prompt.MaxChars {
			limit := cfg.Limits.Prompt.MaxChars
			if limit > 3 {
				limit -= 3
			}
			changelogPrompt = changelogPrompt[:limit] + "..."
		}
	}

	result, err := aiClient.GetCommitMessage(ctx, changelogPrompt)
	if err != nil {
		return "", fmt.Errorf("AI changelog generation failed: %w", err)
	}
	result = aiClient.SanitizeResponse(result, "")
	return strings.TrimSpace(result), nil
}

// resolveRange determines the from/to refs based on options.
func resolveRange(repo *gogit.Repository, opts Options) (string, string, error) {
	if opts.Since != "" {
		return "", "HEAD", nil
	}
	if opts.FromRef != "" && opts.ToRef != "" {
		return opts.FromRef, opts.ToRef, nil
	}
	// Auto-detect from last two tags
	from, to, err := getLastTwoTags(repo)
	if err != nil {
		return "", "", fmt.Errorf("cannot auto-detect range: %w (provide explicit refs)", err)
	}
	return from, to, nil
}

func resolveRef(repo *gogit.Repository, ref string) (plumbing.Hash, error) {
	hash, err := repo.ResolveRevision(plumbing.Revision(ref))
	if err == nil {
		return *hash, nil
	}
	tagRef, err := repo.Tag(ref)
	if err == nil {
		return tagRef.Hash(), nil
	}
	return plumbing.ZeroHash, fmt.Errorf("cannot resolve ref %q", ref)
}

func getLastTwoTags(repo *gogit.Repository) (string, string, error) {
	tagIter, err := repo.Tags()
	if err != nil {
		return "", "", err
	}
	var tags []string
	err = tagIter.ForEach(func(ref *plumbing.Reference) error {
		name := ref.Name().Short()
		if strings.HasPrefix(name, "v") && semver.IsValid(name) {
			tags = append(tags, name)
		}
		return nil
	})
	if err != nil {
		return "", "", err
	}
	sort.Slice(tags, func(i, j int) bool {
		return semver.Compare(tags[i], tags[j]) < 0
	})
	if len(tags) < 2 {
		return "", "", fmt.Errorf("need at least 2 semver tags, found %d", len(tags))
	}
	return tags[len(tags)-2], tags[len(tags)-1], nil
}

func collectCommitsBetween(repo *gogit.Repository, fromHash, toHash plumbing.Hash) ([]*gogitobj.Commit, error) {
	iter, err := repo.Log(&gogit.LogOptions{From: toHash})
	if err != nil {
		return nil, fmt.Errorf("failed to get commit log: %w", err)
	}
	defer iter.Close()

	var commits []*gogitobj.Commit
	err = iter.ForEach(func(c *gogitobj.Commit) error {
		if c.Hash == fromHash {
			return fmt.Errorf("stop") // sentinel to stop iteration
		}
		commits = append(commits, c)
		return nil
	})
	// The "stop" sentinel is expected, not an error
	if err != nil && err.Error() != "stop" {
		return nil, err
	}
	return commits, nil
}

func collectCommitsSince(repo *gogit.Repository, since time.Time) ([]*gogitobj.Commit, error) {
	head, err := repo.Head()
	if err != nil {
		return nil, fmt.Errorf("failed to get HEAD: %w", err)
	}
	iter, err := repo.Log(&gogit.LogOptions{From: head.Hash(), Since: &since})
	if err != nil {
		return nil, fmt.Errorf("failed to get commit log: %w", err)
	}
	defer iter.Close()

	var commits []*gogitobj.Commit
	err = iter.ForEach(func(c *gogitobj.Commit) error {
		commits = append(commits, c)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return commits, nil
}

// GroupCommitsByType groups commits by their conventional commit type.
func GroupCommitsByType(commits []*gogitobj.Commit) map[string][]*gogitobj.Commit {
	typeRegex := regexp.MustCompile(`^(\w+)(\([^)]*\))?:\s*`)
	grouped := make(map[string][]*gogitobj.Commit)
	for _, c := range commits {
		firstLine := strings.SplitN(c.Message, "\n", 2)[0]
		match := typeRegex.FindStringSubmatch(firstLine)
		commitType := "other"
		if len(match) > 1 {
			commitType = strings.ToLower(match[1])
		}
		grouped[commitType] = append(grouped[commitType], c)
	}
	return grouped
}

func formatGroupedCommits(grouped map[string][]*gogitobj.Commit) string {
	// Deterministic order: known types first, then "other"
	order := []string{"feat", "fix", "perf", "refactor", "docs", "test", "chore", "build", "ci", "style", "other"}
	var sb strings.Builder

	for _, typ := range order {
		commits, ok := grouped[typ]
		if !ok || len(commits) == 0 {
			continue
		}
		sb.WriteString(fmt.Sprintf("### %s\n", typ))
		for _, c := range commits {
			firstLine := strings.SplitN(c.Message, "\n", 2)[0]
			shortHash := c.Hash.String()[:7]
			sb.WriteString(fmt.Sprintf("- %s %s\n", shortHash, firstLine))
		}
		sb.WriteString("\n")
	}

	// Any types not in the predefined order
	for typ, commits := range grouped {
		found := false
		for _, o := range order {
			if o == typ {
				found = true
				break
			}
		}
		if found {
			continue
		}
		sb.WriteString(fmt.Sprintf("### %s\n", typ))
		for _, c := range commits {
			firstLine := strings.SplitN(c.Message, "\n", 2)[0]
			shortHash := c.Hash.String()[:7]
			sb.WriteString(fmt.Sprintf("- %s %s\n", shortHash, firstLine))
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

// ParseSince parses a human-readable time string like "2 weeks ago" into a time.Time.
func ParseSince(since string) (time.Time, error) {
	re := regexp.MustCompile(`(\d+)\s+(second|minute|hour|day|week|month|year)s?\s+ago`)
	match := re.FindStringSubmatch(strings.ToLower(since))
	if len(match) < 3 {
		return time.Time{}, fmt.Errorf("invalid --since format: %q (use e.g. '2 weeks ago')", since)
	}
	n, _ := strconv.Atoi(match[1])
	now := time.Now()
	switch match[2] {
	case "second":
		return now.Add(-time.Duration(n) * time.Second), nil
	case "minute":
		return now.Add(-time.Duration(n) * time.Minute), nil
	case "hour":
		return now.Add(-time.Duration(n) * time.Hour), nil
	case "day":
		return now.AddDate(0, 0, -n), nil
	case "week":
		return now.AddDate(0, 0, -7*n), nil
	case "month":
		return now.AddDate(0, -n, 0), nil
	case "year":
		return now.AddDate(-n, 0, 0), nil
	}
	return time.Time{}, fmt.Errorf("unsupported time unit in --since: %q", since)
}
