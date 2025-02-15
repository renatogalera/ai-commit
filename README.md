# AI-Commit

**AI-Commit** is an AI-powered tool that automatically generates [Conventional Commits](https://www.conventionalcommits.org/) based on your staged changes. It leverages [OpenAI](https://openai.com/), [Google Gemini](https://developers.google.com/gemini) and now also [Anthropic Claude](https://www.anthropic.com/) to produce concise, readable commit messages. The tool also features an experimental semantic release process for automated versioning and interactive commit splitting for partial commits. It is inspired by [insulineru/ai-commit](https://github.com/insulineru/ai-commit).

---

## Table of Contents

- [AI-Commit](#ai-commit)
  - [Table of Contents](#table-of-contents)
  - [Key Features](#key-features)
  - [Prerequisites](#prerequisites)
  - [Installation](#installation)
  - [Configuration](#configuration)
    - [Configuration File (config.yaml)](#configuration-file-configyaml)
    - [OpenAI API Key](#openai-api-key)
    - [Gemini API Key](#gemini-api-key)
    - [Anthropic API Key](#anthropic-api-key)
    - [AI Model Selection](#ai-model-selection)
    - [Custom Templates](#custom-templates)
    - [Semantic Release](#semantic-release)
  - [Usage](#usage)
    - [Command-Line Flags](#command-line-flags)
  - [How it Works](#how-it-works)
  - [Examples](#examples)
    - [1. Standard Interactive Commit](#1-standard-interactive-commit)
    - [2. Force Commit (Non-Interactive)](#2-force-commit-non-interactive)
    - [3. Semantic Release](#3-semantic-release)
    - [4. Custom Template](#4-custom-template)
    - [5. Interactive Commit Splitting (Partial Commits)](#5-interactive-commit-splitting-partial-commits)
    - [6. Optional Emoji Prefix](#6-optional-emoji-prefix)
    - [7. Semantic Release with Manual Version Bump](#7-semantic-release-with-manual-version-bump)
    - [8. Using Gemini as AI Provider](#8-using-gemini-as-ai-provider)
    - [9. Using Anthropic as AI Provider](#9-using-anthropic-as-ai-provider)
  - [License](#license)

---

## Key Features

1. **AI-Powered Commit Messages**  
   Generates helpful commit messages by analyzing your staged diff and prompting the configured AI provider for a commit message.

2. **Conventional Commits Compliance**  
   Ensures messages follow [Conventional Commits](https://www.conventionalcommits.org/) for a cleaner, interpretable commit history.

3. **Interactive or Non-Interactive**  
   Choose between a friendly TUI for confirming commits or a `--force` mode to skip prompts.

4. **Customizable Commit Types**  
   Specify a commit type (e.g., `feat`, `fix`, `docs`) or let the tool infer it automatically.

5. **Custom Templates**  
   Dynamically insert the AI-generated commit into custom templates with placeholders (e.g., branch name).

6. **Semantic Release (Experimental)**  
   Automatically suggests a new semantic version tag (`MAJOR.MINOR.PATCH`) based on commit content and optionally creates a new Git tag.

7. **Interactive Commit Splitting**  
   Split large diffs into multiple smaller commits via an interactive TUI that shows each “chunk” of changes. Each partial commit is also assigned an AI-generated commit message.

8. **Optional Emoji Prefix**  
   Add an emoji prefix to your commit message if desired.

9. **Gemini Integration (Experimental)**  
   In addition to OpenAI, you can choose to use Google's Gemini API as your AI provider with a unified interface and UI flow.

10. **Anthropic Integration (Experimental)**  
    Now you can also choose Anthropic's Claude models by setting the provider to `"anthropic"`. This allows you to generate commit messages using Anthropic's safety-first language model, Claude.

---

## Prerequisites

- **Git**: AI-Commit operates on your local Git repository and requires Git installed.
- **Go**: Needed to build AI-Commit from source.
- **OpenAI API Key**: Sign up at [https://openai.com/](https://openai.com/) to get your API key.
- **Gemini API Key (optional)**: If you wish to use Gemini, sign up at [Google Gemini](https://developers.google.com/gemini) and obtain an API key.
- **Anthropic API Key (optional)**: If you wish to use Anthropic's Claude, sign up at [Anthropic](https://www.anthropic.com/) and obtain an API key.

---

## Installation

1. **Clone the Repository**

   ```bash
   git clone https://github.com/renatogalera/ai-commit.git
   cd ai-commit
   ```

2. **Build the Application**

   ```bash
   go build -o ai-commit ./cmd/ai-commit
   ```

3. **(Optional) Install Globally**

   ```bash
   sudo mv ai-commit /usr/local/bin/
   ```

   Now `ai-commit` is accessible from anywhere in your terminal.

---

## Configuration

### Configuration File (config.yaml)

The `config.yaml` file holds default settings used by AI-Commit when generating commit messages. It is typically located at `~/.config/ai-commit/config.yaml` (the directory name is based on the binary name). An updated example configuration file with Anthropic support is shown below:

```yaml
# config.yaml

# Which AI model provider to use. Valid options: "openai", "gemini", or "anthropic"
modelName: "openai"

# OpenAI model to use.
openaiModel: "gpt-4olatest"

# Gemini model to use.
geminiModel: "models/gemini-2.0-flash"

# Anthropic model to use.
anthropicModel: "claude-3-5-sonnet-20241022"

# API key for OpenAI. Overridden by the --apiKey flag or the OPENAI_API_KEY environment variable.
openAiApiKey: "sk-your-openai-key"

# API key for Gemini. Overridden by the --geminiApiKey flag or the GEMINI_API_KEY environment variable.
geminiApiKey: ""

# API key for Anthropic. Overridden by the --anthropicApiKey flag or the ANTHROPIC_API_KEY environment variable.
anthropicApiKey: ""

# Default commit type (e.g. feat, fix, docs, etc.). Overridden by --commit-type flag.
commitType: "feat"

# A default commit message template, e.g. "Modified {GIT_BRANCH} | {COMMIT_MESSAGE}".
template: ""

# A default prompt seed. Typically built automatically from the staged diff, but you can add extra text here.
prompt: ""
```

### OpenAI API Key

Set your API key either via a command-line flag or an environment variable:

- **Command-Line Flag:** `--apiKey YOUR_API_KEY`
- **Environment Variable:**  
  ```bash
  export OPENAI_API_KEY=YOUR_API_KEY
  ```

### Gemini API Key

Supply your Gemini API key via:

- **Command-Line Flag:** `--geminiApiKey YOUR_GEMINI_API_KEY`
- **Environment Variable:**  
  ```bash
  export GEMINI_API_KEY=YOUR_GEMINI_API_KEY
  ```

### Anthropic API Key

Supply your Anthropic API key via:

- **Command-Line Flag:** `--anthropicApiKey YOUR_ANTHROPIC_API_KEY`
- **Environment Variable:**  
  ```bash
  export ANTHROPIC_API_KEY=YOUR_ANTHROPIC_API_KEY
  ```

### AI Model Selection

You can choose the AI provider using both the configuration file and command-line flags:

- **In config.yaml:**  
  Set the `modelName` field to either `"openai"`, `"gemini"`, or `"anthropic"`.  
  Additionally, specify the desired model in `openaiModel`, `geminiModel`, or `anthropicModel`.

- **Via Command-Line Flags (which override config.yaml):**  
  - **`--model`**: Specifies the AI provider (`openai`, `gemini`, or `anthropic`).
  - **`--openai-model`**: Sets the OpenAI model (overrides `openaiModel` in config.yaml).
  - **`--gemini-model`**: Sets the Gemini model (overrides `geminiModel` in config.yaml).
  - **`--anthropic-model`**: Sets the Anthropic model (overrides `anthropicModel` in config.yaml).

### Custom Templates

You can use a commit message template to format the final output. Placeholders include:
- **`{COMMIT_MESSAGE}`** – The AI-generated commit message.
- **`{GIT_BRANCH}`** – The current Git branch name.

Example:
```bash
ai-commit --template "Branch: {GIT_BRANCH}\nCommit: {COMMIT_MESSAGE}"
```

### Semantic Release

1. Use the `--semantic-release` flag when running AI-Commit. This will:
   - Retrieve your current version (from the latest `vX.Y.Z` Git tag).
   - Generate a recommended next version bump based on the commit content.
   
2. Optionally, add the `--manual-semver` flag to manually select the next version (major/minor/patch) via a TUI rather than relying on AI suggestions.

---

## Usage

Run **ai-commit** inside a Git repository with staged changes.

### Command-Line Flags

- **`--apiKey`**  
  Your OpenAI API key. If not set, the tool looks for `OPENAI_API_KEY`.

- **`--geminiApiKey`**  
  Your Gemini API key. If not set, the tool looks for `GEMINI_API_KEY`.

- **`--anthropicApiKey`**  
  Your Anthropic API key. If not set, the tool looks for `ANTHROPIC_API_KEY`.

- **`--model`**  
  Select the AI model provider to use. Valid options are `openai`, `gemini`, or `anthropic`. This flag overrides the `modelName` setting in config.yaml.

- **`--openai-model`**  
  Specify the OpenAI model to be used (e.g., `gpt-4` or `gpt-4olatest`). Overrides the `openaiModel` setting in config.yaml.

- **`--gemini-model`**  
  Specify the Gemini model to be used (e.g., `models/gemini-2.0-flash`). Overrides the `geminiModel` setting in config.yaml.

- **`--anthropic-model`**  
  Specify the Anthropic model to be used (e.g., `claude-3-5-sonnet-20241022`). Overrides the `anthropicModel` setting in config.yaml.

- **`--commit-type`**  
  Specify a commit type (`feat`, `fix`, `docs`, etc.). Otherwise the tool may infer it.

- **`--template`**  
  Custom template for your commit message.

- **`--force`**  
  Automatically commit without interactive confirmation.

- **`--language`**  
  Language used in AI generation (default is `english`).

- **`--semantic-release`**  
  Triggers AI-assisted version bumping and release tasks.

- **`--manual-semver`**  
  When used with `--semantic-release`, launches a TUI to manually select the next version (major/minor/patch).

- **`--interactive-split`**  
  Launches an interactive TUI to split staged changes into multiple partial commits.

- **`--emoji`**  
  Includes an emoji prefix in your commit message if specified.

---

## How it Works

1. **Check Git**  
   Ensures you’re in a valid Git repository and that there are staged changes.

2. **Retrieve Diff & Filter Lock Files**  
   Retrieves the staged diff and filters out lock files (e.g., `go.mod`, `go.sum`) from analysis.

3. **Generate AI Prompt**  
   Builds a prompt including your diff, commit type, and any additional context to send to the AI provider.

4. **AI Request**  
   Calls the chosen AI provider’s chat completion endpoint (OpenAI, Gemini, or Anthropic) to obtain a commit message.

5. **Sanitize & Format**  
   Applies Conventional Commits formatting, optionally adds an emoji prefix, and inserts the result into a custom template if provided.

6. **Interactive UI**  
   Displays a TUI for you to review, regenerate, or modify the commit message (unless `--force` is used).

7. **Commit & (Optional) Semantic Release**  
   Commits your changes and, if enabled, uses semantic release to generate a new version tag based on the commit message.

8. **Interactive Commit Splitting (optional)**  
   If enabled with `--interactive-split`, the tool lets you select diff chunks for partial commits, generating separate commit messages for each.

---

## Examples

### 1. Standard Interactive Commit

```bash
ai-commit --apiKey YOUR_API_KEY
```

1. Stage your changes using `git add .`
2. AI-Commit generates a commit message and presents an interactive UI:
   - **Confirm** (`y` / `enter`)
   - **Regenerate** (`r`)
   - **Select Type** (`t`)
   - **Quit** (`q` / `ctrl+c`)

### 2. Force Commit (Non-Interactive)

```bash
ai-commit --apiKey YOUR_API_KEY --force
```

This bypasses the interactive UI and immediately commits your changes.

### 3. Semantic Release

```bash
ai-commit --apiKey YOUR_API_KEY --semantic-release
```

After generating and applying the commit message, AI-Commit retrieves your current version tag, suggests a version bump, and creates a new Git tag.

### 4. Custom Template

```bash
ai-commit --template "Branch: {GIT_BRANCH}\nCommit: {COMMIT_MESSAGE}"
```

This inserts the current branch name and the AI-generated commit message into the final commit.

### 5. Interactive Commit Splitting (Partial Commits)

```bash
ai-commit --interactive-split
```

1. Stage your changes using `git add .`
2. AI-Commit displays a TUI listing diff chunks.
3. Use **Space** to toggle which chunks to include.
4. Press **C** to commit the selected chunks with an AI-generated message.
5. Repeat or exit as needed.

### 6. Optional Emoji Prefix

```bash
ai-commit --emoji
```

This adds a relevant emoji to the beginning of the commit message if a recognized commit type (e.g., `feat`, `fix`) is inferred.

### 7. Semantic Release with Manual Version Bump

```bash
ai-commit --semantic-release --manual-semver
```

After committing, a TUI will prompt you to manually select the next version (major, minor, or patch).

### 8. Using Gemini as AI Provider

```bash
ai-commit --model gemini --geminiApiKey YOUR_GEMINI_API_KEY
```

- **`--model gemini`**  
  Selects the Gemini provider (overriding `modelName` in config.yaml).
- **`--geminiApiKey`**  
  Supplies your Gemini API key.
- **`--gemini-model`**  
  (Optional) Specifies the Gemini model to use; otherwise, the default in config.yaml is applied.

### 9. Using Anthropic as AI Provider

```bash
ai-commit --model anthropic --anthropicApiKey YOUR_ANTHROPIC_API_KEY --anthropic-model claude-3-5-sonnet-20241022
```

- **`--model anthropic`**  
  Selects the Anthropic provider (overriding `modelName` in config.yaml).
- **`--anthropicApiKey`**  
  Supplies your Anthropic API key.
- **`--anthropic-model`**  
  (Optional) Specifies the Anthropic model to use; otherwise, the default in config.yaml is applied.

All other flags and UI flows remain identical regardless of the chosen provider.

---

## License

This project is released under the [MIT License](LICENSE.md). Please see the LICENSE file for details.
