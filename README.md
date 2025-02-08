# AI-Commit

**AI-Commit** is an AI-powered tool that automatically generates [Conventional Commits](https://www.conventionalcommits.org/) based on your staged changes. It leverages [OpenAI](https://openai.com/) to produce concise, readable commit messages and now includes an experimental semantic release feature for automated versioning, interactive commit splitting for partial commits, and a new **Gemini** integration for those who wish to use Google's Gemini API as an alternative AI provider. Inspired by [insulineru/ai-commit](https://github.com/insulineru/ai-commit).

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
  - [License](#license)

---

## Key Features

1. **AI-Powered Commit Messages**  
   Generates helpful commit messages by analyzing your staged diff and prompting OpenAI (or Gemini) for a commit message.

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
   In addition to OpenAI, you can now choose to use Google's Gemini API as your AI provider. Gemini works seamlessly with the same unified interface and TUI flow.

---

## Prerequisites

- **Git**: AI-Commit operates on your local Git repository and requires Git installed.
- **Go**: Needed to build AI-Commit from source.
- **OpenAI API Key**: Sign up at [https://openai.com/](https://openai.com/) to get your API key.
- **Gemini API Key (optional)**: If you wish to use Gemini, sign up at [Google Gemini](https://developers.google.com/gemini) and obtain an API key.

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

The `config.yaml` file holds default settings used by AI-Commit when generating commit messages. It is typically located at `~/.config/ai-commit/config.yaml` (the directory name is based on the binary name). The file includes the following options:

- **`modelName`**  
  **Description:** Specifies which AI model provider to use.  
  **Valid Options:** `"openai"` or `"gemini"`  
  **Example:**  
  ```yaml
  modelName: "openai"
  ```  
  This option determines whether AI-Commit uses OpenAI's API or Google's Gemini API to generate commit messages.

- **`openaiModel`**  
  **Description:** Defines the specific OpenAI model to be used when `modelName` is set to `"openai"`.  
  **Example:**  
  ```yaml
  openaiModel: "gpt-4olatest"
  ```  
  Make sure that the specified model is supported by your OpenAI account.

- **`geminiModel`**  
  **Description:** Defines the specific Gemini model to be used when `modelName` is set to `"gemini"`.  
  **Example:**  
  ```yaml
  geminiModel: "models/gemini-2.0-flash"
  ```  
  Verify that your Gemini account supports the specified model.

- **`openAiApiKey`**  
  **Description:** Your API key for OpenAI.  
  **Note:** This value is overridden by the `--apiKey` flag or the `OPENAI_API_KEY` environment variable if provided at runtime.  
  **Example:**  
  ```yaml
  openAiApiKey: "sk-your-openai-key"
  ```

- **`geminiApiKey`**  
  **Description:** Your API key for Gemini.  
  **Note:** This value is overridden by the `--geminiApiKey` flag or the `GEMINI_API_KEY` environment variable if provided at runtime.  
  **Example:**  
  ```yaml
  geminiApiKey: ""
  ```

- **`commitType`**  
  **Description:** Sets the default commit type if none is provided via the command line. Common commit types include `"feat"`, `"fix"`, `"docs"`, etc.  
  **Example:**  
  ```yaml
  commitType: "feat"
  ```

- **`template`**  
  **Description:** Provides a default commit message template. You can include placeholders such as `{GIT_BRANCH}` and `{COMMIT_MESSAGE}` which will be dynamically replaced at commit time.  
  **Example:**  
  ```yaml
  template: ""
  ```

- **`prompt`**  
  **Description:** A default prompt seed that influences the AI-generated commit message. Typically, this prompt is built automatically from your staged diff, but you can add extra context here if desired.  
  **Example:**  
  ```yaml
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

If you prefer to use the Gemini model, supply your Gemini API key via:

- **Command-Line Flag:** `--geminiApiKey YOUR_GEMINI_API_KEY`
- **Environment Variable:**  

  ```bash
  export GEMINI_API_KEY=YOUR_GEMINI_API_KEY
  ```

### AI Model Selection

You can choose the AI provider through both the configuration file and command-line flags:

- **In config.yaml:**  
  Set the `modelName` field to either `"openai"` or `"gemini"`.  
  Additionally, specify the desired model in `openaiModel` or `geminiModel` accordingly.

- **Via Command-Line Flags (which override config.yaml):**  
  - **`--model`**: Specifies the AI provider (`openai` or `gemini`).  
  - **`--openai-model`**: Sets the OpenAI model (overrides `openaiModel` in config.yaml).  
  - **`--gemini-model`**: Sets the Gemini model (overrides `geminiModel` in config.yaml).

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
  Your Gemini API key. If not set, the tool looks for `GEMINI_API_KEY`. Use this when you wish to select Gemini as your AI provider.

- **`--model`**  
  Select the AI model provider to use. Valid options are `openai` (default) or `gemini`. This flag overrides the `modelName` setting in config.yaml.

- **`--openai-model`**  
  Specify the OpenAI model to be used (e.g., `gpt-4` or `gpt-4olatest`). Overrides the `openaiModel` setting in config.yaml.

- **`--gemini-model`**  
  Specify the Gemini model to be used (e.g., `models/gemini-2.0-flash`). Overrides the `geminiModel` setting in config.yaml.

- **`--commit-type`**  
  Specify a commit type (`feat`, `fix`, `docs`, etc.). Otherwise the tool may infer it.

- **`--template`**  
  Custom template for your commit message.

- **`--force`**  
  Automatically commit without interactive confirmation.

- **`--language`**  
  Language used in AI generation (default is `english`).

- **`--semantic-release`**  
  Triggers AI-assisted version bumping and release tasks (see [Semantic Release](#semantic-release)).

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
   Calls the chosen AI provider’s chat completion endpoint (either OpenAI or Gemini) to obtain a commit message.

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

All other flags and UI flows remain identical.

---

## License

This project is released under the [MIT License](LICENSE.md). Please see the LICENSE file for details.