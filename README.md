# AI-Commit

**AI-Commit** is a powerful tool that generates [Conventional Commits](https://www.conventionalcommits.org/) using AI (OpenAI, Google Gemini, Anthropic Claude, DeepSeek). *Automatically generate commit messages for Git, saving you time and improving your commit history.*

It supports:
- **OpenAI**
- **Google Gemini**
- **Anthropic Claude**
- **DeepSeek**

Writing good commit messages can be time-consuming. AI-Commit solves this problem by automatically generating clear, concise, and consistent commit messages, enforcing the Conventional Commits standard and leveraging the power of leading AI models.  Whether you're using GPT-4, Gemini, or Claude, AI-Commit streamlines your workflow.

[https://github.com/renatogalera/ai-commit](https://github.com/renatogalera/ai-commit)

---

## Features

- **AI-Powered Commit Messages:** Generate commit messages with AI (OpenAI, Gemini, Anthropic, and DeepSeek).
- **Conventional Commits:** Enforces the Conventional Commits standard for consistency.
- **Interactive TUI**:  Engage with an enhanced interactive Text User Interface to review, regenerate, and customize commit messages with improved feedback and help.
- **Non-Interactive Mode**: Bypass the interactive TUI and commit directly using the `--force` flag for automated workflows.
- **Semantic Release:**  Perform AI-assisted semantic version bumps or manually select version increments (`--manual-semver`).
- **Interactive Commit Splitting**: Utilize a chunk-based partial commit TUI with chunk selection and selection inversion for fine-grained control over commits.
- **Emoji Support:**  Add emojis to your commit messages with the `--emoji` flag.
- **Custom Templates:**  Create and apply custom commit message templates (e.g., `Branch: {GIT_BRANCH}\n{COMMIT_MESSAGE}`).
- **Multiple AI Providers**: Seamlessly switch between OpenAI, Google Gemini, Anthropic Claude, and DeepSeek to utilize the best AI for your needs.
- **Configurable Commit Types**: Tailor AI-Commit to your project's commit conventions by customizing the accepted commit types in the configuration file.
- **Lock File Filtering**: Automatically filter out diffs from lock files (`go.mod`, `package-lock.json`, etc.) to focus AI generation on relevant code changes.
- **Configurable Prompt**: Customize the AI prompt template used to generate commit messages for advanced control over AI behavior.
- **View Diff in TUI**: Review the full Git diff directly within the interactive TUI before committing, using the 'l' key.
- **Enhanced Splitter UI**: The interactive split feature now includes chunk selection inversion and improved status feedback in the TUI.

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

A `config.yaml` is auto-created at `~/.config/ai-commit/config.yaml` with default settings that you can customize:

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
promptTemplate: "" # Customize the AI prompt template here
commitTypes: # Customize accepted commit types for your project
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
lockFiles: # Specify lock files to filter from diff
  - "go.mod"
  - "go.sum"
```

> **Note**: Command-line flags always take precedence over configuration file values. API keys can also be provided through environment variables.

API keys can be specified via environment variables:
- `OPENAI_API_KEY`
- `GEMINI_API_KEY`
- `ANTHROPIC_API_KEY`
- `DEEPSEEK_API_KEY`

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
3.  **Interactive TUI**:  The TUI will launch, presenting you with the AI-generated commit message and several options:

    *   **Confirm Commit**: Press `Enter` or `y` to accept the message and create the commit.
    *   **Regenerate**: Press `r` to generate a new commit message using AI. Track regeneration attempts displayed in the UI.
    *   **Change Commit Type**: Press `t` to select a different commit type from a list, influencing the next AI message generation.
    *   **Edit Message**: Press `e` to manually edit the commit message within the TUI. Save with `Ctrl+s` and cancel with `Esc`.
    *   **Edit Prompt**: Press `p` to customize the prompt text sent to the AI for message generation, allowing for very specific instructions. Apply with `Ctrl+s` and cancel with `Esc`.
    *   **View Diff**: Press `l` to inspect the complete Git diff directly in the TUI before committing. Press `Esc` or `q` to return.
    *   **Help**: Press `?` to toggle the help view, displaying available keybindings for easy navigation and action.
    *   **Quit**: Press `q`, `Esc`, or `Ctrl+C` to exit AI-Commit without committing.

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
    *OpenAI API key.*  
    (For Gemini, Anthropic, or DeepSeek, use `--geminiApiKey`, `--anthropicApiKey`, or `--deepseekApiKey` respectively.)

*   `--commit-type fix|feat|docs|refactor|test|perf|build|ci|chore`  
    *Specify commit type if desired. Useful for guiding the AI or for non-interactive usage.*

*   `--template "Branch: {GIT_BRANCH}\n{COMMIT_MESSAGE}"`  
    *Commit template to wrap the AI's generated message with extra information or formatting.*

*   `--prompt`
    *[Deprecated - use promptTemplate in config.yaml]* - *[Flag retained for backward compatibility, but configuring `promptTemplate` in `config.yaml` is now recommended for persistent custom prompts]* Commit prompt to be passed to the AI model. Use for advanced prompt engineering or specific instructions.

*   `--force`
    *Bypass the interactive TUI and commit directly using the AI-generated message. Ideal for automated scripts and workflows.*

*   `--semantic-release`
    *Enable semantic release functionality to automatically suggest and create a version tag based on the commit message, following semantic versioning conventions. Use with `--manual-semver` for manual version selection in TUI.*

*   `--manual-semver`
    *When used with `--semantic-release`, launches a TUI to manually select the semantic version bump (Major, Minor, Patch) instead of AI-driven suggestion.*

*   `--interactive-split`
    *Launch the interactive commit splitting TUI. Allows for selecting specific diff chunks to stage and commit separately, generating commit messages for each partial commit.*

*   `--emoji`
    *Include an emoji prefix in the commit message based on the commit type (e.g., `✨ feat: ...`).*

---

## Examples

1.  **Simple Interactive Usage**:
    ```bash
    ai-commit
    ```
    Launches the interactive TUI to review and commit a message.

2.  **Force Commit (Non-Interactive)**:
    ```bash
    ai-commit --force
    ```
    Commits staged changes immediately using the AI-generated message, skipping the TUI.

3.  **Semantic Release (AI-Driven Version Bump)**:
    ```bash
    ai-commit --semantic-release
    ```
    Performs a commit and then suggests and creates a version tag based on the commit message.

4.  **Semantic Release (Manual Version Selection)**:
    ```bash
    ai-commit --semantic-release --manual-semver
    ```
    Launches a TUI to manually select Major, Minor, or Patch version bump after the commit.

5.  **Provider and Model Selection**:
    ```bash
    ai-commit --provider=openai --model=gpt-4 --apiKey=sk-OPENAI_KEY
    ai-commit --provider=gemini --model=models/gemini-2.0-flash --geminiApiKey=YOUR_GEMINI_KEY
    ai-commit --provider=anthropic --model=claude-3-sonnet --anthropicApiKey=YOUR_ANTHROPIC_KEY
    ai-commit --provider=deepseek --model=deepseek-chat --deepseekApiKey=YOUR_DEEPSEEK_KEY
    ```
    Examples of specifying different AI providers and models with API keys directly via flags.

6.  **Interactive Split Commit**:
    ```bash
    ai-commit --interactive-split
    ```
    Starts the interactive split TUI to select and commit partial changes.

---

## Get Started

Ready to improve your commit messages and streamline your Git workflow with AI? Install AI-Commit today and start generating better, more consistent commits!
```bash
git clone https://github.com/renatogalera/ai-commit.git
cd ai-commit
go build -o ai-commit ./cmd/ai-commit
sudo mv ai-commit /usr/local/bin/
```

---