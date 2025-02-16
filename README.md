# AI-Commit

**AI-Commit** is a powerful CLI tool designed to revolutionize your Git workflow by leveraging AI for two key tasks: generating commit messages and providing basic code reviews. By integrating cutting-edge AI models, AI-Commit helps you create meaningful, Conventional Commits-compliant messages and get quick feedback on your code changes, right from your terminal.

It supports:
- **OpenAI**
- **Google Gemini**
- **Anthropic Claude**
- **DeepSeek**

Say goodbye to tedious commit message writing and superficial code reviews. AI-Commit automates these processes, saving you valuable time and ensuring consistency and quality in your development lifecycle.

[https://github.com/renatogalera/ai-commit](https://github.com/renatogalera/ai-commit)

---

## ✨ Key Features

- **AI-Powered Commit Messages**: Automatically generate insightful and Conventional Commits-compliant messages using top AI providers (OpenAI, Gemini, Anthropic, DeepSeek).
- **AI Code Reviewer (Subcommand)**: Get basic, AI-driven code reviews directly in your terminal. Identify potential style issues, refactoring opportunities, and basic security concerns before committing. Use the `ai-commit review` subcommand to analyze your staged changes.
- **Interactive TUI**: Refine commit messages in an enhanced, user-friendly Text User Interface. Regenerate messages, change commit types, edit prompts, and even view the full diff—all within the TUI.
- **Non-Interactive Mode (`--force`)**: Automate commit message generation in scripts or workflows, bypassing the TUI for quick, direct commits.
- **Semantic Release (`--semantic-release`)**: Streamline your release process with AI-assisted semantic version bumping. Choose between AI-suggested version updates or manual version selection via TUI (`--manual-semver`).
- **Interactive Commit Splitting (`--interactive-split`)**: Gain granular control over your commits with chunk-based staging and commit message generation for partial commits.
- **Emoji Support (`--emoji`)**: Add a touch of visual flair to your commit history with automatically включены emojis based on commit types.
- **Customizable Templates (`--template`)**: Tailor commit messages to your team's style with custom templates, incorporating dynamic values like branch names.
- **Multi-Provider AI Support**: Choose the best AI for each task by switching seamlessly between OpenAI, Gemini, Anthropic, and DeepSeek.
- **Configurable and Filterable**: Adapt AI-Commit to your projects with customizable commit types and prompt templates. Filter lock file diffs for cleaner, AI-focused message generation and reviews.
- **Diff View in TUI**: Inspect complete Git diffs within the TUI (`l` key) for thorough pre-commit reviews.
- **Enhanced Splitter UI**: Benefit from improved interactive splitting with chunk selection inversion and clear status updates.

---

## 🛠️ Installation

```bash
git clone https://github.com/renatogalera/ai-commit.git
cd ai-commit
go build -o ai-commit ./cmd/ai-commit
# Optionally move it into your PATH for global access
sudo mv ai-commit /usr/local/bin/
```

---

## ⚙️ Configuration

AI-Commit automatically creates a `config.yaml` file at `~/.config/ai-commit/config.yaml` upon first run. This file lets you personalize default settings:

```yaml
provider: "openai"
openAiApiKey: "sk-YOUR-OPENAI-KEY"
openaiModel: "gpt-4o-latest"

geminiApiKey: "YOUR-GEMINI-KEY"
geminiModel: "models/gemini-2.0-flash"

anthropicApiKey: "sk-YOUR-ANTHROPIC-KEY"
anthropicModel: "claude-3-5-sonnet-latest"

deepseekApiKey: "YOUR-DEEPSEEK-KEY"
deepseekModel: "deepseek-chat"

semanticRelease: false
interactiveSplit: false
enableEmoji: false
template: ""
promptTemplate: "" # Customize the AI prompt template for commit messages and reviews
commitTypes: # Define your project's accepted commit types
  - "feat"
  - "fix"
  - "docs"
  - "style"
  - "refactor"
  - "test"
  - "chore"
  - "perf"
  - "build"
  - "ci"
lockFiles: # Specify lock files to be ignored in diffs for commit messages and reviews
  - "go.mod"
  - "go.sum"
```

> **Note**: Command-line flags override config file settings. API keys can be set via environment variables or within `config.yaml`.

API Keys via Environment Variables:

- `OPENAI_API_KEY`
- `GEMINI_API_KEY`
- `ANTHROPIC_API_KEY`
- `DEEPSEEK_API_KEY`

---

## 🚀 Basic Usage

1.  **Stage your changes**:
    ```bash
    git add .
    ```

2.  **Run AI-Commit for Commit Message**:
    ```bash
    ai-commit 
    ```
    Initiates the interactive TUI to generate, review, and commit your message.

3.  **Run AI-Commit for Code Review**:
    ```bash
    ai-commit review
    ```
    Triggers an AI-powered code review of your staged changes, displaying suggestions in the terminal.

4.  **Interactive TUI Options**: When the TUI is active (after running `ai-commit` without `review`), utilize these options:

    -   **Confirm Commit**: `Enter` or `y` to commit with the generated message.
    -   **Regenerate Message**: `r` to generate a new commit message. Track regen attempts in the UI.
    -   **Change Commit Type**: `t` to select a different commit type, influencing AI generation.
    -   **Edit Message**: `e` to manually edit the commit message in the TUI (`Ctrl+s` to save, `Esc` to cancel).
    -   **Edit Prompt**: `p` to customize the AI prompt text for specific instructions (`Ctrl+s` to apply, `Esc` to cancel).
    -   **View Diff**: `l` to review the full Git diff within the TUI (`Esc` or `q` to return).
    -   **Help**: `?` to toggle help text showing keybindings.
    -   **Quit**: `q`, `Esc`, or `Ctrl+C` to exit without commit.

---

## 🎛️ Command-Line Flags

**Main Flags**:

*   `--provider`: AI provider selection (`openai`, `gemini`, `anthropic`, `deepseek`).
*   `--model`: Specific model choice per provider (e.g., `gpt-4`, `models/gemini-2.0-flash`).
*   `--apiKey`, `--geminiApiKey`, `--anthropicApiKey`, `--deepseekApiKey`: API keys for each provider.
*   `--commit-type`: Force a commit type (e.g., `fix`, `feat`) for non-interactive use or AI guidance.
*   `--template`: Custom template for commit messages, wrapping AI output.
*   `--prompt` *(Deprecated)*: Use `promptTemplate` in `config.yaml` for persistent prompt customization instead.

**Workflow Control Flags**:

*   `--force`: Non-interactive commit; skips TUI and commits directly.
*   `--semantic-release`: Enables semantic versioning; suggests/creates version tags post-commit.
*   `--manual-semver`: With `--semantic-release`, manually select version type in TUI.
*   `--interactive-split`: Launches chunk-based commit splitting TUI.
*   `--emoji`: Adds emojis to commit messages based on type.

**Subcommand**:

*   `review`: Trigger AI-powered code review of staged changes: 
    ```bash
    ai-commit review 
    ```

---

## ✍️ Examples

1.  **Interactive Commit**:
    ```bash
    ai-commit
    ```
    Starts the TUI for commit message review and commit.

2.  **Force Commit**:
    ```bash
    ai-commit --force
    ```
    Directly commits staged changes using AI message generation.

3.  **AI-Powered Code Review**:
    ```bash
    ai-commit review
    ```
    Analyzes staged code changes and outputs AI-generated review suggestions in the terminal.

4.  **Semantic Release with Manual Versioning**:
    ```bash
    ai-commit --semantic-release --manual-semver
    ```
    Combines commit, AI-suggested next version, and manual version selection in TUI.

5.  **Provider/Model Options**:
    ```bash
    ai-commit --provider=openai --model=gpt-4 --apiKey=sk-...
    ai-commit --provider=gemini --model=models/gemini-2.0-flash --geminiApiKey=... 
    ai-commit --provider=anthropic --model=claude-3-sonnet --anthropicApiKey=...
    ai-commit --provider=deepseek --model=deepseek-chat --deepseekApiKey=...
    ```
    Demonstrates setting provider, model, and API key via flags.

6.  **Interactive Split for Partial Commits**:
    ```bash
    ai-commit --interactive-split
    ```
    Initiates TUI for chunk selection and partial commit generation.


---

## 🚀 Get Started

Elevate your Git workflow today! Install AI-Commit to generate smarter commit messages and gain AI-driven insights into your code changes.

```bash
git clone https://github.com/renatogalera/ai-commit.git
cd ai-commit
go build -o ai-commit ./cmd/ai-commit
sudo mv ai-commit /usr/local/bin/
```
---