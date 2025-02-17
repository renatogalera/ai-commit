#!/bin/bash
# install_ai_commit.sh
# This script automatically downloads the latest release of ai-commit from GitHub,
# detects your OS and CPU architecture, selects the matching asset,
# downloads it, extracts it if needed, makes it executable, and installs it to /usr/local/bin.
#
# Requirements: curl, jq, tar, and sudo (if not running as root).

set -euo pipefail

###########################################
# Function: error_exit
# Prints an error message and exits.
###########################################
error_exit() {
    echo "Error: $1" >&2
    exit 1
}

###########################################
# Check for required commands: curl, jq, and tar
###########################################
for cmd in curl jq tar; do
    if ! command -v "$cmd" &>/dev/null; then
        error_exit "$cmd is not installed. Please install $cmd."
    fi
done

###########################################
# Hard-coded GitHub repository details
###########################################
GITHUB_OWNER="renatogalera"
GITHUB_REPO="ai-commit"
API_URL="https://api.github.com/repos/${GITHUB_OWNER}/${GITHUB_REPO}/releases/latest"

echo "Fetching latest release information for ${GITHUB_OWNER}/${GITHUB_REPO}..."

###########################################
# Retrieve the latest release data
###########################################
API_RESPONSE=$(curl -sL "$API_URL")
if echo "$API_RESPONSE" | jq -e 'has("message") and .message == "Not Found"' >/dev/null; then
    error_exit "Repository ${GITHUB_OWNER}/${GITHUB_REPO} not found or no releases available."
fi

# Extract release information for logging
RELEASE_TAG=$(echo "$API_RESPONSE" | jq -r '.tag_name')
RELEASE_NAME=$(echo "$API_RESPONSE" | jq -r '.name')
echo "Latest release: ${RELEASE_TAG} - ${RELEASE_NAME}"

###########################################
# Detect OS and CPU architecture
###########################################
OS_TYPE=$(uname -s)
case "$OS_TYPE" in
    Linux*)   os="linux" ;;
    Darwin*)  os="darwin" ;;
    CYGWIN*|MINGW*|MSYS*) os="windows" ;;
    *)        error_exit "Unsupported OS: $OS_TYPE" ;;
esac

ARCH_TYPE=$(uname -m)
case "$ARCH_TYPE" in
    x86_64)   arch="amd64" ;;
    aarch64|arm64) arch="arm64" ;;
    armv7l)   arch="armv7" ;;
    *)        error_exit "Unsupported architecture: $ARCH_TYPE" ;;
esac

echo "Detected system: OS=${os}, Architecture=${arch}"

###########################################
# Find the asset matching the system details
# (case-insensitive search)
###########################################
ASSET_INFO=$(echo "$API_RESPONSE" | jq --arg os "$os" --arg arch "$arch" '
    .assets[] | select(
        (.name | ascii_downcase | contains($os)) and
        (.name | ascii_downcase | contains($arch))
    )
')

if [ -z "$ASSET_INFO" ]; then
    error_exit "No asset found for OS '${os}' and architecture '${arch}'."
fi

# Ensure there is only one matching asset
MATCH_COUNT=$(echo "$API_RESPONSE" | jq --arg os "$os" --arg arch "$arch" '
    [.assets[] | select(
        (.name | ascii_downcase | contains($os)) and
        (.name | ascii_downcase | contains($arch))
    )] | length
')
if [ "$MATCH_COUNT" -gt 1 ]; then
    error_exit "Multiple assets found for OS '${os}' and architecture '${arch}'. Please refine your asset naming."
fi

ASSET_URL=$(echo "$ASSET_INFO" | jq -r '.browser_download_url')
ASSET_NAME=$(echo "$ASSET_INFO" | jq -r '.name')

echo "Selected asset: ${ASSET_NAME}"
echo "Downloading asset from: ${ASSET_URL}..."

###########################################
# Download the asset to a temporary file
###########################################
TMP_FILE=$(mktemp /tmp/"${ASSET_NAME}".XXXXXX) || error_exit "Failed to create a temporary file."

HTTP_STATUS=$(curl -L -o "$TMP_FILE" -w "%{http_code}" "$ASSET_URL")
if [ "$HTTP_STATUS" -ne 200 ]; then
    rm -f "$TMP_FILE"
    error_exit "Download failed. HTTP status code: $HTTP_STATUS"
fi

echo "Download completed."

###########################################
# Check if the asset is a tar.gz (or .tgz) archive.
# If so, extract it.
###########################################
BINARY_PATH=""
if [[ "$ASSET_NAME" =~ \.(tar\.gz|tgz)$ ]]; then
    echo "Asset is an archive. Extracting..."
    TMP_DIR=$(mktemp -d /tmp/ai-commit-extract.XXXXXX) || error_exit "Failed to create temporary extraction directory."
    tar -xzf "$TMP_FILE" -C "$TMP_DIR"
    # Look for the 'ai-commit' binary (assumed to be at the root or inside one directory)
    BINARY_PATH=$(find "$TMP_DIR" -type f -name "ai-commit" -perm /111 | head -n 1)
    if [ -z "$BINARY_PATH" ]; then
        rm -rf "$TMP_DIR" "$TMP_FILE"
        error_exit "ai-commit binary not found inside the archive."
    fi
    echo "Extracted binary: $BINARY_PATH"
else
    # Asset is assumed to be a binary directly.
    BINARY_PATH="$TMP_FILE"
fi

###########################################
# Validate the binary and set permissions
###########################################
if [ ! -s "$BINARY_PATH" ]; then
    rm -f "$BINARY_PATH" "$TMP_FILE"
    error_exit "Downloaded binary is empty."
fi

chmod +x "$BINARY_PATH" || error_exit "Failed to set execute permission on the binary."

###########################################
# Install the binary to /usr/local/bin
###########################################
DESTINATION="/usr/local/bin/ai-commit"
echo "Installing to ${DESTINATION}..."

if [ "$EUID" -ne 0 ]; then
    sudo mv "$BINARY_PATH" "$DESTINATION" || { rm -f "$BINARY_PATH"; error_exit "Failed to move file to ${DESTINATION}."; }
else
    mv "$BINARY_PATH" "$DESTINATION" || { rm -f "$BINARY_PATH"; error_exit "Failed to move file to ${DESTINATION}."; }
fi

# Clean up temporary files and directories
rm -f "$TMP_FILE"
if [ -n "${TMP_DIR:-}" ]; then
    rm -rf "$TMP_DIR"
fi

echo "Installation complete. 'ai-commit' is now available in /usr/local/bin."
