# AI-Commit

![commit](img/2025-09-03_03-10-55.gif)

**AI-Commit** AI-Commit streamlines your Git workflow with AI—focused on three core tasks that matter

* Generate **Conventional Commits**-compliant messages
* Run a quick **AI code review** on staged diffs
* **Review/enforce commit message style** (clarity, informativeness, consistency)

It supports multiple providers:

* **OpenAI** (default)
* **Google Gemini**
* **Anthropic Claude**
* **DeepSeek**
* **Ollama** (local models)
* **OpenRouter** (multi-model gateway)

Focus on better commits, consistent standards, and reduced toil—right from your terminal.

Repository: [https://github.com/renatogalera/ai-commit](https://github.com/renatogalera/ai-commit)

---

## Installation

### One-liner script

The script detects your OS/arch, downloads the latest release, makes it executable, and installs to `/usr/local/bin` (using `sudo` when needed).

```bash
curl -sL https://raw.githubusercontent.com/renatogalera/ai-commit/main/scripts/install_ai_commit.sh | bash
```

> If you’re not root, it’ll prompt for `sudo` to move the binary.

### Build from source

```bash
git clone https://github.com/renatogalera/ai-commit.git
cd ai-commit
go build -o ai-commit ./cmd/ai-commit
sudo mv ai-commit /usr/local/bin/
```

---

## Features (overview)

* **AI-powered commit messages** that follow Conventional Commits.
* **AI code review** (`ai-commit review`) on staged diffs (style, refactor hints, simple risks).
* **Commit message style review** (`--review-message`) to enforce clarity & quality.
* **Interactive TUI** to refine messages, switch types, view full diff, and (where supported) stream AI output.
* **Non-interactive mode** (`--force`) for scripts/CI.
* **Semantic release assist** (`--semantic-release`, with optional `--manual-semver`).
* **Interactive split commits** (`--interactive-split`) with chunk selection/inversion.
* **Emoji support** (`--emoji`) mapped to commit types.
* **Custom templates** (`--template`) and **prompt template** (`promptTemplate` in config).
* **Changelog generation** (`ai-commit changelog`) between tags or time ranges.
* **Ticket auto-detection** from branch names (JIRA, GitHub, Linear) via `{TICKET_ID}` template placeholder.
* **Git hook integration** (`ai-commit hook install`) for automatic commit message generation.
* **Scope auto-suggestion** from changed file paths to guide the AI.
* **Diff/prompt limits** to bound payload sizes.
* **Lock file filtering** for cleaner AI context.

---

## Configuration

On first run, AI-Commit creates:

```
~/.config/ai-commit/config.yaml
```

> Path is derived from the binary name; if you rename the binary, the folder matches the new name.

### Example `config.yaml`

```yaml
authorName: "Your Name"
authorEmail: "youremail@example.com"

provider: "openai"       # default provider if no CLI flag is given

providers:
  openai:
    apiKey: ""
    model: "chatgpt-4o-latest"
    baseURL: "https://api.openai.com/v1"
  google:
    apiKey: ""
    model: "gemini-2.5-flash"
    baseURL: ""
  anthropic:
    apiKey: ""
    model: "claude-3-7-sonnet-latest"
    baseURL: "https://api.anthropic.com"
  deepseek:
    apiKey: ""
    model: "deepseek-chat"
    baseURL: "https://api.deepseek.com/v1"
  openrouter:
    apiKey: ""
    model: "openrouter/auto"
    baseURL: "https://openrouter.ai/api/v1"
  ollama:
    apiKey: ""           # Not required
    model: "llama2"
    baseURL: "http://localhost:11434"

limits:
  diff:
    enabled: false
    maxChars: 0
  prompt:
    enabled: false
    maxChars: 0

semanticRelease: false
interactiveSplit: false
enableEmoji: false

commitType: ""           # Optional default
template: ""             # Optional commit message template; can use {COMMIT_MESSAGE}, {GIT_BRANCH}, and {TICKET_ID}
promptTemplate: ""       # Optional global prompt template for AI prompts
ticketPattern: ""        # Custom regex for ticket extraction from branch names (default: auto-detect JIRA, GitHub, Linear)

commitTypes:
  - type: "feat"     emoji: "✨"
  - type: "fix"      emoji: "🐛"
  - type: "docs"     emoji: "📚"
  - type: "style"    emoji: "💎"
  - type: "refactor" emoji: "♻️"
  - type: "test"     emoji: "🧪"
  - type: "chore"    emoji: "🔧"
  - type: "perf"     emoji: "🚀"
  - type: "build"    emoji: "📦"
  - type: "ci"       emoji: "👷"

lockFiles:
  - "go.mod"
  - "go.sum"
```

**Notes**

* Command-line flags override config values.
* `authorName`/`authorEmail` are used for `git` authoring by `CommitChanges`. Set these to your identity (the tool does *not* read your git config).
* `promptTemplate` influences the prompts for message generation, code reviews, and style checks.
* `limits.diff/prompt` allow truncation/summarization before sending to providers.

### Environment variables

For each provider, the code observes:

* `${PROVIDER}_API_KEY` (e.g., `OPENAI_API_KEY`, `GOOGLE_API_KEY`, `ANTHROPIC_API_KEY`, `DEEPSEEK_API_KEY`, `OPENROUTER_API_KEY`)
* `${PROVIDER}_BASE_URL` (e.g., `OPENAI_BASE_URL`, `GOOGLE_BASE_URL`, …, `OLLAMA_BASE_URL`)

---

## Usage

1. **Stage changes**

```bash
git add .
```

2. **Generate a commit message (interactive)**

```bash
ai-commit
```

**TUI keybindings**

* Confirm commit: `Enter` or `y`
* Regenerate: `r` (limited attempts)
* Change commit type: `t`
* Edit commit message: `e` (save with `Ctrl+s`, cancel `Esc`)
* Edit extra prompt text: `p` (save with `Ctrl+s`, cancel `Esc`)
* View full diff: `l` (exit diff with `Esc` or `q`)
* Toggle help: `?`
* Quit: `q` / `Esc` / `Ctrl+C`

> **Style review display:**
>
> * If the selected provider **does not stream**, style-review suggestions (when `--review-message` is enabled) are shown in the TUI.
> * For **streaming** providers, style review is not yet shown live inside the TUI.

---

## CLI Reference

```
ai-commit [flags]
ai-commit review
ai-commit summarize
ai-commit changelog [fromRef..toRef]
ai-commit hook install|uninstall
```

### Main flags

* `--provider` — one of: `openai`, `google`, `anthropic`, `deepseek`, `ollama`, `openrouter`
* `--model` — overrides `providers.<name>.model`
* `--apiKey` — overrides `providers.<name>.apiKey` or `${PROVIDER}_API_KEY`
* `--baseURL` — overrides `providers.<name>.baseURL` or `${PROVIDER}_BASE_URL`
* `--language` — language for prompts/responses (default: `english`)
* `--commit-type` — force a Conventional Commit type (`feat`, `fix`, …)
* `--template` — apply a template to the final message (supports `{COMMIT_MESSAGE}`, `{GIT_BRANCH}`, and `{TICKET_ID}`)
* `--review-message` — run AI style review on the generated commit message
* `--msg-only` — generate commit message and print to stdout (used by git hooks)

### Workflow control

* `--force` — non-interactive; prints style feedback (if any) then commits immediately
* `--semantic-release` — compute next version from latest commit and create a tag
* `--manual-semver` — with `--semantic-release`, choose version via TUI
* `--interactive-split` — open the chunk-based split TUI

### Subcommands

* `review` — AI code review of staged changes

  ```bash
  ai-commit review
  ```

* `summarize` — pick a commit via an in-terminal fuzzy finder and generate an AI summary

  ```bash
  ai-commit summarize
  ```

* `changelog` — generate an AI-powered changelog between two refs

  ```bash
  ai-commit changelog v0.10.0..v0.11.0
  ai-commit changelog --since=”2 weeks ago”
  ai-commit changelog --output CHANGELOG.md
  ai-commit changelog                          # auto-detect: last two tags
  ```

* `hook install` / `hook uninstall` — manage the `prepare-commit-msg` git hook

  ```bash
  ai-commit hook install           # install hook
  ai-commit hook install --force   # overwrite existing hook
  ai-commit hook uninstall         # remove hook
  ```

> The “fuzzy finder” is embedded via a Go library; no external `fzf` binary is required.

---

## Examples

**Interactive, English, OpenAI**

```bash
ai-commit --provider=openai --model=chatgpt-4o-latest --language=english
```

**Force commit + style review (non-interactive)**

```bash
ai-commit --force --review-message
```

**Interactive with style review (non-streaming providers show feedback in TUI)**

```bash
ai-commit --provider=google --model=models/gemini-2.5-flash --review-message
```

**Use Anthropic via env vars**

```bash
export ANTHROPIC_API_KEY=sk-...
ai-commit --provider=anthropic --model=claude-3-7-sonnet-latest
```

**DeepSeek with explicit base URL**

```bash
ai-commit --provider=deepseek --model=deepseek-chat --baseURL=https://api.deepseek.com/v1 --apiKey=sk-...
```

**Ollama (local)**

```bash
ai-commit --provider=ollama --model=llama2 --baseURL=http://localhost:11434
```

**OpenRouter**

```bash
ai-commit --provider=openrouter --model=openrouter/auto --apiKey=sk-...
```

**Interactive split**

```bash
ai-commit --interactive-split
```

**Semantic release (manual selection)**

```bash
ai-commit --semantic-release --manual-semver
```

**Generate changelog between releases**

```bash
ai-commit changelog v0.10.0..v0.11.0
ai-commit changelog --since="2 weeks ago" --output CHANGELOG.md
```

**Use ticket ID from branch in template**

```yaml
# config.yaml
template: "{COMMIT_MESSAGE}\n\nRefs: {TICKET_ID}"
```

```bash
# On branch feature/PROJ-456-add-login:
ai-commit
# Result: "feat(auth): add login\n\nRefs: PROJ-456"
```

**Install as git hook**

```bash
ai-commit hook install
# Now 'git commit' auto-generates AI messages
```

---

## Provider matrix

| Provider   | API key required | Default model (example)    | Base URL (example)                          | Streaming in code |
| ---------- | ---------------- | -------------------------- | ------------------------------------------- | ----------------- |
| OpenAI     | Yes              | `chatgpt-4o-latest`        | `https://api.openai.com/v1`                 | Yes               |
| Google     | Yes              | `gemini-2.5-flash`         | (default)                                   | No                |
| Anthropic  | Yes              | `claude-3-7-sonnet-latest` | `https://api.anthropic.com`                 | Yes               |
| DeepSeek   | Yes              | `deepseek-chat`            | `https://api.deepseek.com/v1`               | Yes               |
| OpenRouter | Yes              | `openrouter/auto`          | `https://openrouter.ai/api/v1`              | Yes               |
| Ollama     | No               | `llama2`                   | `http://localhost:11434`                    | No                |

> **Env vars:** `${PROVIDER}_API_KEY` and `${PROVIDER}_BASE_URL` (uppercase provider name).

---

## TUI details

* **Streaming**: If the provider implements streaming, the TUI streams completion tokens while showing a progress pulse.
* **Diff view**: Press `l` to inspect the full Git diff inside the TUI.
* **Commit type guess**: If not forced, the UI guesses a type from the first line and lets you override with `t`.
* **Regeneration limit**: Default max of 3 successive regenerations per run (see UI label).

---

## Style review behavior (`--review-message`)

* **Non-interactive (`--force`)**: style feedback prints to the terminal before the commit. If issues are found, it prints a short, styled block; if not, it remains quiet or shows “No issues found.”
* **Interactive**:

  * **Non-streaming providers**: style feedback appears in the TUI alongside the generated message.
  * **Streaming providers**: style feedback inside the TUI is not yet implemented.

---

## Commit templates

You can wrap the final AI message with a template, e.g.:

```yaml
template: |
  {COMMIT_MESSAGE}

  Branch: {GIT_BRANCH}
  Refs: {TICKET_ID}
```

Placeholders:

* `{COMMIT_MESSAGE}` — replaced with the AI-generated (and type-prefixed) message
* `{GIT_BRANCH}` — resolved via `git` at runtime
* `{TICKET_ID}` — auto-extracted from the branch name (supports JIRA `PROJ-123`, GitHub `#42`/`GH-42`, Linear `ENG-456`). Configure a custom regex with `ticketPattern` in config.

---

## Limits & filtering

* **Lock files**: diffs for paths listed in `lockFiles` are filtered out from the AI prompt to reduce noise.
* **Limits**:

  * `limits.diff`: truncate/summarize diffs before prompting
  * `limits.prompt`: hard cap the final prompt size (truncated with `...`)

---

## Troubleshooting

* **Empty commit message**: if AI returns an empty string, the tool aborts (non-interactive) or stays in the UI. Try regenerating or inspecting the diff.
* **“API key required” errors**: ensure either `--apiKey`, the `${PROVIDER}_API_KEY` environment variable, or a non-empty `providers.<name>.apiKey` is set.
* **Ollama base URL**: must be valid. If you run into connectivity or 4xx from a provider, confirm your endpoint and headers (especially for self-hosted gateways).
* **Author identity**: set `authorName`/`authorEmail` in `config.yaml` to avoid commits with default values.

---

## Security & privacy

* AI-Commit sends your **diffs/prompts** to the configured provider(s). Review your provider’s data retention and privacy policies. For highly sensitive repos, prefer **local** providers (e.g., **Ollama**) or configure strict limits.

---

## License

MIT (project’s chart mentions MIT; keep the repo’s `LICENSE` in sync with public statements).

---

## Changelog / Roadmap (short)

* ✅ Split-commit TUI with chunk selection/inversion
* ✅ Summarize commits with fuzzy finder
* ✅ AI-powered changelog generation (`ai-commit changelog`)
* ✅ Ticket auto-detection from branch names (`{TICKET_ID}`)
* ✅ Git hook integration (`ai-commit hook install`)
* ✅ Scope auto-suggestion from changed file paths
* ✅ CI pipeline with tests and linting
* ✅ Comprehensive test suite (150+ test cases)
* 🔜 In-TUI style review for streaming providers

---

