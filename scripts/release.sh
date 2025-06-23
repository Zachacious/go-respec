#!/usr/bin/env bash

# Enable strict error handling
set -euo pipefail

# Add error trapping to show line numbers where failures occur
trap 'echo "❌ ERROR: Command failed at line $LINENO"; exit 1' ERR

# === CONFIGURATION ===
REPO="github.com/Zachacious/go-respec"
MAIN_BRANCH="main"
INITIAL_VERSION="v0.1.0"
DEBUG=true  # Set to false to disable debug output

# Function to print debug messages
debug() {
    if [[ "$DEBUG" == "true" ]]; then
        echo "🔧 DEBUG: $1"
    fi
}

# Function to show help message
show_help() {
    cat <<EOF
📦 Usage: $(basename "$0") [version] [--patch|--minor|--major] [-m "message"]

Options:
  vX.Y.Z          Explicitly set the release version (e.g., v1.2.3).
  --patch         Bump patch version (default if no version specified).
  --minor         Bump minor version.
  --major         Bump major version.
  -m, --message   Provide release notes inline.
  -h, --help      Show this help message and exit.

Examples:
  ./release.sh v1.3.0 -m "Add feature X and fix bug Y"
      → Tag v1.3.0 explicitly, with release notes.

  ./release.sh --minor -m "Add feature X"
      → Auto-detect latest tag and bump MINOR version.

  ./release.sh
      → Auto-bump patch version and prompt for release notes.

Notes:
- If the version already exists, you'll be prompted to overwrite it.
- You must have a clean git working directory before releasing.
EOF
}

# --- Initial Health Checks ---
if ! command -v gh >/dev/null 2>&1; then
    echo "❌ GitHub CLI (gh) is required but not found. Please install it: https://cli.github.com/"
    exit 1
fi

if ! gh auth status >/dev/null 2>&1; then
    echo "❌ GitHub CLI is not authenticated. Please run 'gh auth login' first."
    exit 1
fi

echo "🔍 Checking for uncommitted changes..."
if ! git diff-index --quiet HEAD --; then
    echo "❌ Uncommitted changes detected. Please commit or stash them before running a release."
    git status --short
    exit 1
fi

# --- Git Synchronization ---
echo "🔄 Switching to '$MAIN_BRANCH' and pulling latest changes..."
git checkout "$MAIN_BRANCH"
GIT_TERMINAL_PROMPT=0 git pull origin "$MAIN_BRANCH"

echo "🔄 Pushing '$MAIN_BRANCH' to origin to ensure it is up-to-date..."
GIT_TERMINAL_PROMPT=0 git push origin "$MAIN_BRANCH"

echo "🔄 Fetching and pruning all tags from the 'origin' remote..."
GIT_TERMINAL_PROMPT=0 git fetch origin --prune --prune-tags

# --- Go Module Verification ---
echo "🔍 Verifying Go module setup..."
if [[ ! -f "go.mod" ]]; then
    echo "❌ go.mod file not found. This doesn't appear to be a Go module."
    exit 1
fi

MOD_NAME=$(grep "^module" go.mod | awk '{print $2}')
if [[ -z "$MOD_NAME" ]]; then
    echo "❌ Could not determine module name from go.mod file."
    exit 1
fi
debug "Go module name: $MOD_NAME"

# --- Run Tests ---
echo "🧪 Running tests..."
go test ./...

# --- Argument Parsing ---
BUMP="patch"
VERSION=""
NOTES=""
while [[ $# -gt 0 ]]; do
  case "$1" in
    -h|--help)
      show_help
      exit 0
      ;;
    v*.*.*)
      if ! [[ "$1" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
        echo "❌ Invalid version format: '$1'. Must be in format vX.Y.Z"
        exit 1
      fi
      VERSION="$1"
      shift
      ;;
    --major) BUMP="major"; shift ;;
    --minor) BUMP="minor"; shift ;;
    --patch) BUMP="patch"; shift ;;
    -m|--message)
      NOTES="$2"
      if [[ -z "$NOTES" ]]; then
        echo "❌ Release notes cannot be empty when using -m/--message."
        exit 1
      fi
      shift 2
      ;;
    *)
      echo "❌ Unknown argument: $1"
      echo "   Use --help for usage."
      exit 1
      ;;
  esac
done

# --- Detect and Calculate Version ---
if [[ -n "$VERSION" ]]; then
    debug "Explicit version provided: $VERSION"
    if git rev-parse "$VERSION" >/dev/null 2>&1; then
        echo "⚠️ Tag $VERSION already exists."
        read -p "   Do you want to overwrite it? This will delete and recreate the tag. (y/N) " -n 1 -r
        echo
        if [[ ! $REPLY =~ ^[Yy]$ ]]; then
            echo "🛑 Release cancelled."
            exit 1
        fi
        git tag -d "$VERSION" || true
        git push --delete origin "$VERSION" || true
    fi
else
    debug "No version specified, will calculate based on latest tag"
    LATEST_TAG_RAW=$(git describe --tags --abbrev=0 2>/dev/null || echo "")
    debug "Raw tag from git: '$LATEST_TAG_RAW'"

    if [[ -n "$LATEST_TAG_RAW" ]]; then
        LATEST_TAG=$(echo "$LATEST_TAG_RAW" | tr -d '[:space:]')
        echo "🔍 Latest tag found: $LATEST_TAG"
        debug "Processing tag: '$LATEST_TAG'"

        if [[ "$LATEST_TAG" =~ ^v([0-9]+)\.([0-9]+)\.([0-9]+)$ ]]; then
            MAJOR="${BASH_REMATCH[1]}"
            MINOR="${BASH_REMATCH[2]}"
            PATCH="${BASH_REMATCH[3]}"

            case "$BUMP" in
                major)
                    MAJOR=$((MAJOR + 1))
                    MINOR=0
                    PATCH=0
                    ;;
                minor)
                    MINOR=$((MINOR + 1))
                    PATCH=0
                    ;;
                patch)
                    PATCH=$((PATCH + 1))
                    ;;
            esac

            VERSION="v$MAJOR.$MINOR.$PATCH"
        else
            echo "❌ Invalid latest tag format: '$LATEST_TAG'. Expected format: vX.Y.Z"
            exit 1
        fi
    else
        echo "🔍 No existing tags found. Creating initial release."
        VERSION="$INITIAL_VERSION"
    fi
    debug "Calculated version: $VERSION"
fi

# --- Confirmation Step ---
echo "✅ New version will be: $VERSION"
read -p "   Are you sure you want to proceed with tagging? (y/N) " -n 1 -r
echo
if [[ ! $REPLY =~ ^[Yy]$ ]]; then
    echo "🛑 Release cancelled."
    exit 1
fi

if [[ -z "$NOTES" ]]; then
    echo "✏️ Please enter the release notes. End with Ctrl+D."
    NOTES=$(</dev/stdin)
fi

if [[ -z "$NOTES" ]]; then
    echo "❌ Release notes cannot be empty."
    exit 1
fi

# --- Execution Step ---
echo "1. Tagging version $VERSION..."
git tag -a "$VERSION" -m "Release $VERSION"

echo "2. Pushing tag to GitHub..."
if ! GIT_TERMINAL_PROMPT=0 git push origin "$VERSION"; then
    echo "❌ Git Error: Failed to push the new tag. Please check your permissions and credentials."
    git tag -d "$VERSION"
    exit 1
fi

echo "3. Building release artifacts using 'make'..."
if ! make release; then
    echo "❌ Build Error: 'make release' command failed."
    echo "   To recover, you may want to delete the tag:"
    echo "   git tag -d $VERSION && git push --delete origin $VERSION"
    exit 1
fi

if [[ ! -d "dist" ]]; then
    echo "❌ Error: 'dist' directory not found after build."
    echo "   To recover, you may want to delete the tag:"
    echo "   git tag -d $VERSION && git push --delete origin $VERSION"
    exit 1
fi

DIST_FILES=$(find dist -type f | wc -l)
if [[ "$DIST_FILES" -eq 0 ]]; then
    echo "❌ Error: No artifacts found in 'dist' directory after build."
    echo "   To recover, you may want to delete the tag:"
    echo "   git tag -d $VERSION && git push --delete origin $VERSION"
    exit 1
fi

echo "4. Creating GitHub Release..."
if ! gh release create "$VERSION" dist/* \
    --title "respec $VERSION" \
    --notes "$NOTES"; then
    echo "❌ Error: Failed to create GitHub release."
    echo "   The tag has been pushed, but the release wasn't created."
    echo "   To recover, you may want to delete the tag:"
    echo "   git tag -d $VERSION && git push --delete origin $VERSION"
    exit 1
fi

echo "5. Notifying Go proxy..."
echo "   Waiting for Go proxy to acknowledge the new version..."

PROXY_TIMEOUT=60
PROXY_START_TIME=$(date +%s)
PROXY_NOTIFIED=false

while [[ "$(date +%s)" -lt "$((PROXY_START_TIME + PROXY_TIMEOUT))" ]]; do
    if GOPROXY=proxy.golang.org go list -m "$REPO@$VERSION" >/dev/null 2>&1; then
        PROXY_NOTIFIED=true
        echo "   ✅ Go proxy successfully updated!"
        break
    fi
    echo "   Waiting for Go proxy to update (retrying in 5 seconds)..."
    sleep 5
done

if [[ "$PROXY_NOTIFIED" != "true" ]]; then
    echo "   ⚠️ Warning: Timed out waiting for Go proxy to acknowledge the version."
    echo "   This doesn't affect the release, but users might have to wait a bit longer before installation."
fi

echo ""
echo "✅ Release $VERSION completed successfully!"
echo "📦 Users can install with: go install $REPO@$VERSION"
