# AI-Commit

**AI-Commit** is a tool that generates [Conventional Commits](https://www.conventionalcommits.org/) using AI. 

It supports:
- **OpenAI**
- **Google Gemini**
- **Anthropic Claude**.

It can also optionally perform an AI-assisted semantic release or split staged changes into multiple partial commits.

---

## Features

- **AI-Powered Commit Messages** (OpenAI, Gemini, or Anthropic)
- **Conventional Commits** enforced
- **Interactive** or **Non-Interactive** (`--force`)
- **Semantic Release** with optional manual or AI-driven version bumps
- **Interactive Commit Splitting** (partial commits)
- **Emoji Support** (`--emoji`)
- **Custom Templates** (e.g. `Branch: {GIT_BRANCH}\n{COMMIT_MESSAGE}`)

---

## Installation

```bash
git clone https://github.com/renatogalera/ai-commit.git
cd ai-commit
go build -o ai-commit ./cmd/ai-commit
# Optionally move it into your PATH
sudo mv ai-commit /usr/local/bin/
```

---

## Configuration

A `config.yaml` is auto-created at `~/.config/ai-commit/config.yaml` with defaults:

```yaml
provider: "openai"
openAiApiKey: "sk-YOUR-OPENAI-KEY"
openaiModel: "gpt-4"

geminiApiKey: "YOUR-GEMINI-KEY"
geminiModel: "models/gemini-2.0-flash"

anthropicApiKey: "sk-YOUR-ANTHROPIC-KEY"
anthropicModel: "claude-2"

semanticRelease: false
interactiveSplit: false
enableEmoji: false
commitType: ""
template: ""
prompt: ""

authorName:  "ai-commit"
authorEmail: "ai-commit@example.com"
```

> **Note**: Any CLI flags override these config values.

You can also specify keys via environment variables:
- `OPENAI_API_KEY`
- `GEMINI_API_KEY`
- `ANTHROPIC_API_KEY`

---

## Basic Usage

1. **Stage your changes**:
   ```bash
   git add .
   ```
2. **Run AI-Commit**:
   ```bash
   ai-commit
   ```
3. **Choose** whether to:
   - Confirm the commit (`Enter` / `y`)
   - Regenerate the message (`r`)
   - Change commit type (`t`)
   - Edit the message (`e`)
   - Add custom prompt text (`p`)
   - Quit without committing (`q` / `Ctrl+C`)

---

## Command-Line Flags

**Key flags**:
- `--provider`  
  *Which AI provider to use?*  
  Valid: `openai`, `gemini`, `anthropic`  
  (Default from config.yaml)

- `--model`  
  *Which sub-model to use within that provider?*  
  e.g., `gpt-4`, `models/gemini-2.0-flash`, `claude-2`

- `--apiKey`  
  *OpenAI key.*  
  (For Gemini or Anthropic, use `--geminiApiKey` or `--anthropicApiKey`.)

- `--commit-type fix|feat|docs|refactor|test|perf|build|ci|chore`
  *Specify commit type if desired.*

- `--template "Branch: {GIT_BRANCH}\n{COMMIT_MESSAGE}"`  
  *Commit template to wrap the AI's result.*

- `--force`
  *Commit without TUI confirmation.*

- `--semantic-release`
  *Perform optional AI-based version bump or manual version bump (`--manual-semver`).*

- `--interactive-split`
  *Launch a chunk-based partial commit TUI.*

- `--emoji`
  *Include emoji in the commit (e.g. `✨ feat: ...`).*

---

## Examples

1. **Simple**:
   ```bash
   ai-commit
   ```
2. **Force Commit**:
   ```bash
   ai-commit --force
   ```
3. **Semantic Release**:
   ```bash
   ai-commit --semantic-release
   ```
4. **Provider + Model**:
   ```bash
   ai-commit --provider=openai --model=gpt-4 --apiKey=sk-OPENAI_KEY
   ai-commit --provider=gemini --model=models/gemini-2.0-flash --geminiApiKey=YOUR_GEMINI_KEY
   ai-commit --provider=anthropic --model=claude-2 --anthropicApiKey=YOUR_ANTHROPIC_KEY
   ```
5. **Interactive Split**:
   ```bash
   ai-commit --interactive-split
   ```
6. **Manual SemVer**:
   ```bash
   ai-commit --semantic-release --manual-semver
   ```