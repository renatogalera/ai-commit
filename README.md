# AI-Commit

**AI-Commit** is a tool that generates [Conventional Commits](https://www.conventionalcommits.org/) using AI. 

It supports   
- **OpenAI**
- **Google Gemini**
- **Anthropic Claude**. 
   
It can also optionally perform an AI-assisted semantic release or split staged changes into multiple partial commits.

## Features

- **AI-Powered Commit Messages** (OpenAI, Gemini, or Anthropic)
- **Conventional Commits** enforced
- **Interactive** or **Non-Interactive** (`--force`)
- **Semantic Release** with optional manual or AI-driven version bumps
- **Interactive Commit Splitting** (partial commits)
- **Emoji Support** with `--emoji`
- **Custom Templates** (e.g. `Branch: {GIT_BRANCH}\n{COMMIT_MESSAGE}`)

## Installation

```bash
git clone https://github.com/renatogalera/ai-commit.git
cd ai-commit
go build -o ai-commit ./cmd/ai-commit
# Optionally move it into your PATH
sudo mv ai-commit /usr/local/bin/
```

## Configuration

A `config.yaml` is auto-created at `~/.config/ai-commit/config.yaml` with defaults:

```yaml
modelName: "openai"
openAiApiKey: "sk-YOUR-OPENAI-KEY"
openaiModel: "gpt-4"
geminiApiKey: ""
geminiModel: "models/gemini-2.0-flash"
anthropicApiKey: ""
anthropicModel: "claude-3-5-sonnet-latest"
commitType: ""
template: ""
semanticRelease: false
interactiveSplit: false
enableEmoji: false
prompt: ""
authorName: "Your Name"
authorEmail: "youremail@example.com"
```

*Any CLI flags override these config values.* You can also specify keys via environment variables:
- `OPENAI_API_KEY`
- `GEMINI_API_KEY`
- `ANTHROPIC_API_KEY`

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

## Command-Line Flags

Common flags:
- `--model openai|gemini|anthropic`  
- `--apiKey YOUR_OPENAI_KEY` (or use `--geminiApiKey` / `--anthropicApiKey`)
- `--commit-type fix|feat|docs|...`
- `--template "Branch: {GIT_BRANCH}\n{COMMIT_MESSAGE}"`
- `--force` (bypass interactive UI)
- `--semantic-release` (auto version bump)
- `--manual-semver` (choose major/minor/patch in a TUI)
- `--interactive-split` (chunk-based partial commits)
- `--emoji` (emoji prefix on commit)

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
4. **Anthropic**:
   ```bash
   ai-commit --model anthropic --anthropicApiKey YOUR_CLAUDE_KEY
   ```
5. **Interactive Split**:
   ```bash
   ai-commit --interactive-split
   ```
---