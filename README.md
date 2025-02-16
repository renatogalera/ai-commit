# AI-Commit

**AI-Commit** is a powerful tool that generates [Conventional Commits](https://www.conventionalcommits.org/) using AI (OpenAI, Google Gemini, Anthropic Claude, DeepSeek). *Automatically generate commit messages for Git, saving you time and improving your commit history.*

It supports:
- **OpenAI**
- **Google Gemini**
- **Anthropic Claude**
- **DeepSeek**

Writing good commit messages can be time-consuming. AI-Commit solves this problem by automatically generating clear, concise, and consistent commit messages, enforcing the Conventional Commits standard and leveraging the power of leading AI models.  Whether you're using GPT-4, Gemini, or Claude, AI-Commit streamlines your workflow.

---

## Features

- **AI-Powered Commit Messages:** Generate commit messages with AI (OpenAI, Gemini, or Anthropic).
- **Conventional Commits:** Enforces the Conventional Commits standard for consistency.
- **Interactive or Non-Interactive:** Use the interactive TUI or bypass it with the `--force` flag.
- **Semantic Release:**  Perform AI-assisted or manual semantic version bumps (major/minor/patch).
- **Interactive Commit Splitting:**  Easily create partial commits with an interactive chunk-based TUI.
- **Emoji Support:**  Add emojis to your commit messages with the `--emoji` flag.
- **Custom Templates:**  Create custom commit message templates (e.g., `Branch: {GIT_BRANCH}\n{COMMIT_MESSAGE}`).
- **Multiple AI Providers:** Seamlessly switch between OpenAI, Google Gemini, and Anthropic Claude.
- **Smart Commit Message Generation:** Uses advanced AI models for intelligent and context-aware commit messages.

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
openaiModel: "chatgpt-4o-latest"

geminiApiKey: "YOUR-GEMINI-KEY"
geminiModel: "models/gemini-2.0-flash"

anthropicApiKey: "sk-YOUR-ANTHROPIC-KEY"
anthropicModel: "claude-3-5-sonnet-latest"

deepseekApiKey: "YOUR-DEEPSEEK-KEY"
deepseekModel: "deepseek-chat"


semanticRelease: false
interactiveSplit: false
enableEmoji: false
commitType: ""
template: ""
prompt: ""
```

> **Note**: Any CLI flags override these config values.

You can also specify API keys via environment variables:
- `OPENAI_API_KEY`
- `GEMINI_API_KEY`
- `ANTHROPIC_API_KEY`

---

## Basic Usage

1.  **Stage your changes**:
    ```bash
    git add .
    ```
2.  **Run AI-Commit**:
    ```bash
    ai-commit
    ```
3.  **Choose** whether to:
    *   Confirm the commit (`Enter` / `y`)
    *   Regenerate the message (`r`)
    *   Change commit type (`t`)
    *   Edit the message (`e`)
    *   Add custom prompt text (`p`)
    *   Quit without committing (`q` / `Ctrl+C`)

---

## Command-Line Flags

**Key flags**:

*   `--provider`  
    *Which AI provider to use?*  
    Valid: `openai`, `gemini`, `anthropic`, `deepseek`  
    (Default from config.yaml)

*   `--model`  
    *Which sub-model to use within that provider?*  
    e.g., `gpt-4`, `models/gemini-2.0-flash`, `claude-2`

*   `--apiKey`  
    *OpenAI key.*  
    (For Gemini or Anthropic or DeepSeek, use `--geminiApiKey` or `--anthropicApiKey` or `--deepseekApiKey`.)

*   `--commit-type fix|feat|docs|refactor|test|perf|build|ci|chore`  
    *Specify commit type if desired (e.g., for automatic commit type detection).*

*   `--template "Branch: {GIT_BRANCH}\n{COMMIT_MESSAGE}"`  
    *Commit template to wrap the AI's generated message.*

*   `--force`
    *Bypass the interactive TUI and commit directly (for automated workflows).*

*   `--semantic-release`
    *Perform an optional AI-based version bump or manual version bump (`--manual-semver`).

*   `--interactive-split`
    *Launch a chunk-based partial commit TUI for fine-grained control over your commits.*

*   `--emoji`
    *Include an emoji prefix in the commit message (e.g., `✨ feat: ...`).*

---

## Examples

1.  **Simple**:
    ```bash
    ai-commit
    ```
2.  **Force Commit**:
    ```bash
    ai-commit --force
    ```
3.  **Semantic Release**:
    ```bash
    ai-commit --semantic-release
    ```
4.  **Provider + Model**:
    ```bash
    ai-commit --provider=openai --model=gpt-4 --apiKey=sk-OPENAI_KEY
    ai-commit --provider=gemini --model=models/gemini-2.0-flash --geminiApiKey=YOUR_GEMINI_KEY
    ai-commit --provider=anthropic --model=claude-2 --anthropicApiKey=YOUR_ANTHROPIC_KEY
    ai-commit --provider=deepseek --model=deepseek-chat --deepseekApiKey=YOUR_DEEPSEEK_KEY
    ```
5.  **Interactive Split**:
    ```bash
    ai-commit --interactive-split
    ```
6. **Manual SemVer**:
    ```bash
    ai-commit --semantic-release --manual-semver
    ```

---

## Get Started

Ready to improve your commit messages with AI? Install AI-Commit today and start generating better, more consistent commits!
```bash
git clone https://github.com/renatogalera/ai-commit.git
cd ai-commit
go build -o ai-commit ./cmd/ai-commit
sudo mv ai-commit /usr/local/bin/
```

---