# AI-Commit

![commit](img/commit.png)

**AI-Commit** is a powerful CLI tool designed to revolutionize your Git workflow by leveraging AI for three key tasks: generating commit messages, providing basic code reviews, and enforcing commit message style guides. By integrating cutting-edge AI models, AI-Commit helps you create meaningful, Conventional Commits-compliant messages, get quick feedback on your code changes, and ensure your commit messages adhere to a high standard of clarity and informativeness—all right from your terminal.

It supports:

- **Phind** (current model is free, enjoy!)
- **OpenAI**
- **Google Gemini**
- **Anthropic Claude**
- **DeepSeek**
- **Ollama** (local AI models)
- **OpenRouter** (access to multiple open-source models)

Boost your commit quality, enforce standards, and save valuable time with AI-Commit, your all-in-one AI assistant for Git workflows.

[https://github.com/renatogalera/ai-commit](https://github.com/renatogalera/ai-commit)

---

## 🛠️ Installation

You can install **AI-Commit** using one of two methods: via our automated installation script or by building from source.

### Automated Installation via Script

The installation script will:

- **Detect** your operating system and CPU architecture.
- **Fetch** the latest release of AI-Commit from GitHub.
- **Download** the appropriate binary asset.
- **Set** the executable permission.
- **Install** the binary to `/usr/local/bin` (using `sudo` if required).

To install via the script, run the following commands in your terminal:

```bash
curl -sL https://raw.githubusercontent.com/renatogalera/ai-commit/main/scripts/install_ai_commit.sh | bash
```

*Note*: If you are not running as root, the script will prompt for your password to use `sudo` when moving the binary.

### Building from Source

If you prefer to build AI-Commit manually from source, follow these steps:

```bash
git clone https://github.com/renatogalera/ai-commit.git
cd ai-commit
go build -o ai-commit ./cmd/ai-commit
# Optionally, move the binary into your PATH for global access:
sudo mv ai-commit /usr/local/bin/
```

---

## ✨ Key Features

- **AI-Powered Commit Messages**: Automatically generate insightful and Conventional Commits-compliant messages using top AI providers (OpenAI, Google, Anthropic, DeepSeek, and now Phind).
- **AI Code Reviewer (Subcommand)**: Get basic, AI-driven code reviews directly in your terminal. Identify potential style issues, refactoring opportunities, and basic security concerns before committing. Use the `ai-commit review` subcommand to analyze your staged changes.
- **AI Commit Message Style Guide Enforcer (`--review-message`)**: Automatically review and enforce your commit message style using AI. Get feedback on clarity, informativeness, and overall quality to ensure consistently excellent commit messages. Enable with the `--review-message` flag during commit message generation.
- **Interactive TUI**: Refine commit messages in an enhanced, user-friendly Text User Interface. Regenerate messages, change commit types, edit prompts, view the full diff, and now also benefit from AI-driven style review feedback—all within the TUI.
- **Non-Interactive Mode (`--force`)**: Automate commit message generation and style enforcement in scripts or workflows, bypassing the TUI for quick, direct commits.
- **Semantic Release (`--semantic-release`)**: Streamline your release process with AI-assisted semantic version bumping. Choose between AI-suggested version updates or manual version selection via TUI (`--manual-semver`).
- **Interactive Commit Splitting (`--interactive-split`)**: Gain granular control over your commits with chunk-based staging and commit message generation for partial commits.
- **Emoji Support (`--emoji`)**: Add a touch of visual flair to your commit history with automatically included emojis based on commit types.
- **Customizable Templates (`--template`)**: Tailor commit messages to your team's style with custom templates, incorporating dynamic values like branch names.
- **Multi-Provider AI Support**: Choose the best AI for each task by switching seamlessly between OpenAI, Google, Anthropic, DeepSeek, OpenRouter, and Phind.
- **Configurable and Filterable**: Adapt AI-Commit to your projects with customizable commit types and prompt templates. Filter lock file diffs for cleaner, AI-focused message generation and reviews.
- **Diff View in TUI**: Inspect complete Git diffs within the TUI (`l` key) for thorough pre-commit reviews.
- **Enhanced Splitter UI**: Benefit from improved interactive splitting with chunk selection inversion and clear status updates.

---

## ⚙️ Configuration

AI-Commit automatically creates a `config.yaml` file at `~/.config/ai-commit/config.yaml` upon first run. This file lets you personalize default settings:

```yaml
# Your name and email address for git commits.
authorName: "Your Name"
authorEmail: "youremail@example.com"

provider: "phind"
limits:
  diff:
    enabled: false
    maxChars: 0
  prompt:
    enabled: false
    maxChars: 0

# Preferred provider config.
providers:
  phind:
    apiKey: ""             # Phind does not require an API key by default
    model: "Phind-70B"
    baseURL: "https://https.extension.phind.com/agent/"
  openai:
    apiKey: "sk-YOUR-OPENAI-KEY"
    model: "gpt-5-nano"
    baseURL: "https://api.openai.com/v1"
  google:
    apiKey: "YOUR-GOOGLE-KEY"
    model: "models/gemini-2.5-flash"
    baseURL: "https://generativelanguage.googleapis.com"
  anthropic:
    apiKey: "sk-YOUR-ANTHROPIC-KEY"
    model: "claude-sonnet-4-20250514"
    baseURL: "https://api.anthropic.com/v1"
  deepseek:
    apiKey: "YOUR-DEEPSEEK-KEY"
    model: "deepseek-chat"
    baseURL: "https://api.deepseek.com/v1"
  openrouter:
    apiKey: "YOUR-OPENROUTER-KEY"
    model: "openrouter/auto"
    baseURL: "https://openrouter.ai/api/v1"
  ollama:
    apiKey: ""             # Ollama does not require an API key by default
    model: "llama3"
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
commitType: ""
template: ""
promptTemplate: "" # Customize the AI prompt template for commit messages, reviews, and style checks
commitTypes:
  - type: "feat"
    emoji: "✨"
  - type: "fix"
    emoji: "🐛"
  - type: "docs"
    emoji: "📖"
  - type: "style"
    emoji: "🎨"
  - type: "refactor"
    emoji: "♻️"
  - type: "test"
    emoji: "🧪"
  - type: "chore"
    emoji: "🔧"
  - type: "perf"
    emoji: "⚡"
  - type: "build"
    emoji: "📦"
  - type: "ci"
    emoji: "🚀"

lockFiles: # Specify lock files to be ignored in diffs for commit messages and reviews
  - "go.mod"
  - "go.sum"
```

> **Note**: Command-line flags always take precedence over configuration file values. API keys can be set via environment variables or within `config.yaml`. You can now also customize the `promptTemplate` in this file to adjust the behavior of both commit message generation and code reviews.

Environment Overrides 

- `${PROVIDER}_API_KEY` and `${PROVIDER}_BASE_URL`, with `PROVIDER` in uppercase.
  Examples: `OPENAI_API_KEY`, `OPENAI_BASE_URL`; `GOOGLE_API_KEY`, `GOOGLE_BASE_URL`; `ANTHROPIC_API_KEY`, `ANTHROPIC_BASE_URL`; `DEEPSEEK_API_KEY`, `DEEPSEEK_BASE_URL`; `PHIND_API_KEY`, `PHIND_BASE_URL`; `OLLAMA_BASE_URL`.

## 🚀 Basic Usage

1.  **Stage your changes**:
    ```bash
    git add .
    ```

2. **Commit Message Generation**     
    ```bash
    ai-commit
    ```
    Standard interactive commit message generation, without style review.

    -   **Confirm Commit**: `Enter` or `y` to commit with the generated message.
    -   **Regenerate Message**: `r` to generate a new commit message. Track regen attempts in the UI.
    -   **Change Commit Type**: `t` to select a different commit type, influencing AI generation.
    -   **Edit Message**: `e` to manually edit the commit message in the TUI (`Ctrl+s` to save, `Esc` to cancel).
    -   **Edit Prompt**: `p` to customize the AI prompt text for specific instructions (`Ctrl+s` to apply, `Esc` to cancel).
    -   **View Diff**: `l` to review the full Git diff within the TUI (`Esc` or `q` to return).
    -   **Help**: `?` to toggle help text showing keybindings.
    -   **Quit**: `q`, `Esc`, or `Ctrl+C` to exit without commit.
    -  **AI Style Review Feedback**: When using `--review-message`, any style feedback from the AI will also be displayed in the TUI (feature to be implemented in future versions for interactive feedback). For now, feedback is shown in the terminal output before the TUI.

---

## 🎛️ Command-Line Flags

**Main Flags**:

*   `--provider`: AI provider selection (`openai`, `google`, `anthropic`, `deepseek`, `phind`, `ollama`, `openrouter`, `your_provider`).
*   `--model`: Specific model choice for the selected provider (overrides `providers.<name>.model`).
*   `--apiKey`: API key for the selected provider (overrides `providers.<name>.apiKey` or env `${PROVIDER}_API_KEY`).
*   `--baseURL`: Base URL for the selected provider (overrides `providers.<name>.baseURL` or env `${PROVIDER}_BASE_URL`).
*   Limits (config.yaml):
    - `limits.diff.enabled` + `limits.diff.maxChars` to summarize/truncate large diffs before sending to AI.
    - `limits.prompt.enabled` + `limits.prompt.maxChars` to hard-limit total prompt size.
*   `--commit-type`: Force a commit type (e.g., `fix`, `feat`) for non-interactive use or AI guidance.
*   `--template`: Custom template for commit messages, wrapping AI output.
*   `--prompt` *(Deprecated)*: Use `promptTemplate` in `config.yaml` for persistent prompt customization instead.

**Workflow Control Flags**:

*   `--force`: Non-interactive commit; skips TUI and commits directly.
*   `--semantic-release`: Enables semantic versioning; suggests/creates version tags post-commit.
*   `--manual-semver`: With `--semantic-release`, manually select version type in TUI.
*   `--interactive-split`: Launches chunk-based commit splitting TUI.
*   `--emoji`: Adds emojis to commit messages based on type.
*   `--review-message`: Enable AI-powered commit message style review. After generating a commit message, AI-Commit sends it to AI for a style review. Feedback is provided in the terminal output, ensuring commit messages are clear, informative, and adhere to best practices.

**Subcommand**:

*   `review`: Trigger AI-powered code review of staged changes:
    ```bash
    ai-commit review
    ```
*   `summarize`: **NEW** - Summarize a selected commit using AI. Uses `fzf` to pick a commit from the commit history, then displays an AI-generated summary of that commit.
     ```bash
     ai-commit summarize
     ```

---

## ✍️ More Examples


1.  **Summarize a Commit**:
    ```bash
    ai-commit summarize
    ```
    Lists commits with `fzf`, and after you pick one, shows an AI-generated summary in the terminal.

2.  **Interactive Commit with Style Review**:
    ```bash
    ai-commit --review-message
    ```
    Launches the interactive TUI after generating and AI-reviewing the commit message style.

3.  **Force Commit with Style Review (Non-Interactive)**:
    ```bash
    ai-commit --force --review-message
    ```
    Directly commits staged changes after generating and AI-reviewing the commit message style, skipping the TUI. Style review feedback is printed to the terminal before commit.

4.  **AI-Powered Code Review**:
    ```bash
    ai-commit review
    ```
    Executes AI code review and outputs suggestions to the terminal.

5.  **Semantic Release (Manual Version)**:
    ```bash
    ai-commit --semantic-release --manual-semver
    ```
    Semantic release with manual version selection TUI.

6.  **Provider and Model Selection**:
    ```bash
    ai-commit --provider=openai --model=gpt-4 --apiKey=sk-...
    ai-commit --provider=google --model=models/google-2.0-flash --googleApiKey=...
    ai-commit --provider=anthropic --model=claude-3-sonnet --anthropicApiKey=...
    ai-commit --provider=deepseek --model=deepseek-chat --deepseekApiKey=...
    ai-commit --provider=phind --model=Phind-70B           # Phind model is currently free; API key is optional
    ai-commit --provider=ollama --model=llama2 --ollamaBaseURL=http://localhost:11434  # Use local Ollama instance
    ai-commit --provider=openrouter --model=openrouter/auto --openrouterApiKey=...
    ```

7.  **Interactive Split Commit**:
    ```bash
    ai-commit --interactive-split
    ```
---
