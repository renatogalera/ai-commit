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
    - [OpenAI API Key](#openai-api-key)
    - [Gemini API Key](#gemini-api-key)
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
   Split large diffs into multiple smaller commits via an interactive TUI that shows each “chunk” of changes. Each partial commit can also be assigned an AI-generated commit message.

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

### OpenAI API Key

Set your API key either via a command-line flag or an environment variable:

- **Command-Line Flag:** `--apiKey YOUR_API_KEY`
- **Environment Variable:**  

  ```bash
  export OPENAI_API_KEY=YOUR_API_KEY
  ```

### Gemini API Key

If you prefer to use the new Gemini model, you must provide your Gemini API key via:

- **Command-Line Flag:** `--geminiApiKey YOUR_GEMINI_API_KEY`
- **Environment Variable:**  

  ```bash
  export GEMINI_API_KEY=YOUR_GEMINI_API_KEY
  ```

Then specify the Gemini provider using the `--model` flag (see below).

### Custom Templates

Use the `--template` flag to supply a commit message template with placeholders:

- **`{COMMIT_MESSAGE}`**  
  The AI-generated commit message.
- **`{GIT_BRANCH}`**  
  The name of the current Git branch.

For example:

```bash
ai-commit --template "Branch: {GIT_BRANCH}\nCommit: {COMMIT_MESSAGE}"
```

### Semantic Release

1. Use `--semantic-release` when running AI-Commit. This will:
   - Parse your current version (from the latest `vX.Y.Z` Git tag).
   - Consult the AI provider to determine if you need a MAJOR, MINOR, or PATCH bump.
   
2. *(Optional)* Add the `--manual-semver` flag to pick the next version manually (major/minor/patch) instead of using the AI suggestion.

---

## Usage

Run **ai-commit** inside a Git repository with staged changes.

### Command-Line Flags

- **`--apiKey`**  
  Your OpenAI API key. If not set, the tool looks for `OPENAI_API_KEY`.
- **`--geminiApiKey`**  
  Your Gemini API key. If not set, the tool looks for `GEMINI_API_KEY`. Use this when you wish to select Gemini as your AI provider.
- **`--model`**  
  Select the AI model to use. Acceptable values are `openai` (default) or `gemini`.
- **`--commit-type`**  
  Specify a commit type (`feat`, `fix`, `docs`, etc.). Otherwise the tool may infer it.
- **`--template`**  
  Custom template for your commit message.
- **`--force`**  
  Automatically commit without interactive confirmation.
- **`--language`**  
  Language used in AI generation (default `english`).
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
   Ensures you’re in a valid Git repository and there are staged changes.

2. **Retrieve Diff & Filter Lock Files**  
   Pulls the staged diff and removes lock files (`go.mod`, `go.sum`, etc.) from analysis.

3. **Generate AI Prompt**  
   Assembles a request including your diff, commit type, and desired language.

4. **AI Request**  
   Calls the selected AI provider's chat completion endpoint (either OpenAI or Gemini) with the prompt, retrieving a commit message.

5. **Sanitize & Format**  
   Applies Conventional Commits formatting, optionally adds an emoji prefix if requested, and substitutes into any template you provide.

6. **Interactive UI**  
   Presents a TUI to review and optionally regenerate the commit message unless `--force` is used.

7. **Commit & (Optional) Semantic Release**  
   Commits your changes. If `--semantic-release` is enabled, AI-Commit:
   - Reads the current version tag.
   - Generates a recommended next version (either via AI suggestion or manually via TUI).
   - Creates a new Git tag.

8. **Interactive Commit Splitting (optional)**  
   If `--interactive-split` is used, AI-Commit shows a separate TUI that breaks the diff into chunks, letting you select what belongs in each commit. Each partial commit is also assigned an AI-generated commit message.

---

## Examples

### 1. Standard Interactive Commit

```bash
ai-commit --apiKey YOUR_API_KEY
```

1. Stage changes with `git add .`
2. Let AI-Commit generate a message and show you an interactive UI:
   - **Confirm** (`y` / `enter`)
   - **Regenerate** (`r`)
   - **Select Type** (`t`)
   - **Quit** (`q` / `ctrl+c`)

### 2. Force Commit (Non-Interactive)

```bash
ai-commit --apiKey YOUR_API_KEY --force
```

Bypasses the interactive UI and commits immediately.

### 3. Semantic Release

```bash
ai-commit --apiKey YOUR_API_KEY --semantic-release
```

1. Generates the commit message (interactive or forced).  
2. After committing, it reads your latest version tag, queries the AI provider for a suggested version increment, and tags your repository.

### 4. Custom Template

```bash
ai-commit --template "Branch: {GIT_BRANCH}\nCommit: {COMMIT_MESSAGE}"
```

Inserts the current branch name and AI-generated message into the final commit log.

### 5. Interactive Commit Splitting (Partial Commits)

```bash
ai-commit --interactive-split
```

1. Stage your changes with `git add .`
2. AI-Commit shows a TUI listing each diff chunk or “hunk.”
3. Use **Space** to toggle which chunks you want included in a partial commit.
4. Press **C** to commit the selected chunks. AI will generate the message.
5. Repeat or exit once you have committed all the chunks you need.

### 6. Optional Emoji Prefix

```bash
ai-commit --emoji
```

Adds a relevant emoji to the beginning of your commit message if a recognized commit type (e.g. `feat`, `fix`) is found or inferred.

### 7. Semantic Release with Manual Version Bump

```bash
ai-commit --semantic-release --manual-semver
```

In this mode, after the commit is created, a TUI will prompt you to manually select the next version (major, minor, or patch) instead of using an AI-generated suggestion. This gives you more control over your versioning process.

### 8. Using Gemini as AI Provider

To use Gemini instead of OpenAI, run the tool with the following flags:

```bash
ai-commit --model gemini --geminiApiKey YOUR_GEMINI_API_KEY
```

- **`--model gemini`**  
  Selects the Gemini provider.
- **`--geminiApiKey`**  
  Supplies your Gemini API key. Alternatively, set the environment variable:

  ```bash
  export GEMINI_API_KEY=YOUR_GEMINI_API_KEY
  ```

Gemini integrates with AI-Commit in the same way as OpenAI, using the unified `AIClient` interface. All other flags and the interactive TUI flow remain the same.

---

## License

This project is released under the [MIT License](LICENSE.md). Please see the LICENSE file for details.
```
