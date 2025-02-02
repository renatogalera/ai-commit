# AI-Commit

**AI-Commit** is an AI-powered tool that automatically generates [Conventional Commits](https://www.conventionalcommits.org/) based on your staged changes. It leverages [OpenAI](https://openai.com/) to produce concise, readable commit messages and now includes an **experimental semantic release** feature for automated versioning and **interactive split** mode for partial commits. Inspired by [insulineru/ai-commit](https://github.com/insulineru/ai-commit).

---

## Table of Contents

- [AI-Commit](#ai-commit)
  - [Table of Contents](#table-of-contents)
  - [Key Features](#key-features)
  - [Prerequisites](#prerequisites)
  - [Installation](#installation)
  - [Configuration](#configuration)
    - [OpenAI API Key](#openai-api-key)
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
  - [License](#license)

---

## Key Features

1. **AI-Powered Commit Messages**  
   Generates helpful commit messages by analyzing your staged diff and prompting OpenAI.

2. **Conventional Commits Compliance**  
   Ensures messages follow [Conventional Commits](https://www.conventionalcommits.org/) for a cleaner, interpretable commit history.

3. **Interactive or Non-Interactive**  
   Choose between a friendly TUI for confirming commits or a `--force` mode to skip prompts.

4. **Customizable Commit Types**  
   Specify a commit type (e.g., `feat`, `fix`, `docs`) or let the tool infer it automatically.

5. **Custom Templates**  
   Dynamically insert the AI-generated commit into custom templates with placeholders (e.g., branch name).

6. **Semantic Release (Experimental)**  
   Automatically suggests a new semantic version tag (`MAJOR.MINOR.PATCH`) based on commit content, optionally creates and pushes a new git tag, and invokes [GoReleaser](https://goreleaser.com/) to publish release artifacts.

7. **Interactive Commit Splitting**  
   Split large diffs into multiple smaller commits via an interactive TUI that shows each “chunk” of changes. Select which lines or hunks you want in each commit. Each partial commit can also be assigned an AI-generated commit message.

8. **Optional Emoji Prefix**  
   Add an emoji prefix to your commit message if desired.

---

## Prerequisites

- **Git**: AI-Commit operates on your local Git repository and requires Git installed.
- **Go**: Needed to build AI-Commit from source.
- **OpenAI API Key**: Sign up at [https://openai.com/](https://openai.com/) to get your API key.

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

To automatically suggest the next version and optionally create & push a Git tag plus run [GoReleaser](https://goreleaser.com/):

1. Install GoReleaser if you haven’t already:
   ```bash
   brew install goreleaser/tap/goreleaser
   ```
   or see [official installation docs](https://goreleaser.com/install/).

2. Use `--semantic-release` when running AI-Commit. This will:
   - Parse your current version (from the latest `vX.Y.Z` Git tag).
   - Consult OpenAI to determine if you need a MAJOR, MINOR, or PATCH bump.
   - Automatically tag your repository, push the new tag, and run `goreleaser release`.

---

## Usage

Run **ai-commit** inside a Git repository with staged changes.

### Command-Line Flags

- **`--apiKey`**  
  Your OpenAI API key. If not set, the tool looks for `OPENAI_API_KEY`.
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

4. **OpenAI Request**  
   Calls OpenAI’s chat completion endpoint with the prompt, retrieving a commit message.

5. **Sanitize & Format**  
   Applies Conventional Commits formatting, optionally adds an emoji prefix if requested, and substitutes into any template you provide.

6. **Interactive UI**  
   Presents a TUI to review and optionally regenerate the commit message unless `--force` is used.

7. **Commit & (Optional) Semantic Release**  
   Commits your changes. If `--semantic-release` is enabled, AI-Commit:
   - Reads the current version tag.
   - Generates a recommended next version (MAJOR, MINOR, or PATCH).
   - Creates and pushes a new Git tag.
   - Invokes GoReleaser to build and publish your release artifacts.

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
2. After committing, it reads your latest version tag, queries OpenAI for a suggested version increment, tags your repository, and runs GoReleaser.

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

Adds a relevant emoji to the beginning of your commit message if a recognized commit type (e.g. `feat`, `fix`) is found or inferred. If the commit type is unrecognized or you do not use `--emoji`, it won't add an emoji prefix.

---

## License

This project is released under the [MIT License](LICENSE.md). Please see the LICENSE file for details.
```

**Notes**  
1. The new `--emoji` flag is entirely optional.  
2. If you don’t pass `--emoji`, no emoji prefix is added to your commit messages.  