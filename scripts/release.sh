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

# === SCRIPT LOGIC ===

# --- Initial Health Checks ---
if ! command -v gh >/dev/null 2>&1; then
    echo "❌ GitHub CLI (gh) is required but not found. Please install it: https://cli.github.com/"
    exit 1
fi

# Verify GitHub CLI is authenticated
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
if ! GIT_TERMINAL_PROMPT=0 git pull origin "$MAIN_BRANCH"; then
    echo "❌ Git Error: Failed to pull from origin. Please ensure your git credentials are configured correctly."
    exit 1
fi
echo "🔄 Pushing '$MAIN_BRANCH' to origin to ensure it is up-to-date..."
if ! GIT_TERMINAL_PROMPT=0 git push origin "$MAIN_BRANCH"; then
    echo "❌ Git Error: Failed to push to origin. Please check your permissions and credentials."
    exit 1
fi
echo "🔄 Fetching and pruning all tags from the 'origin' remote..."
if ! GIT_TERMINAL_PROMPT=0 git fetch origin --prune --prune-tags; then
    echo "❌ Git Error: Failed to fetch and prune tags. Please ensure your git credentials are configured correctly."
    exit 1
fi

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
if ! go test ./...; then
    echo "❌ Tests failed. Fix the failing tests before creating a release."
    exit 1
fi

# --- Argument Parsing ---
BUMP="patch"
VERSION=""
NOTES=""
while [[ $# -gt 0 ]]; do
  case "$1" in
    v*.*.*) 
      # Validate version format strictly
      if ! [[ "$1" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
        echo "❌ Invalid version format: '$1'. Must be in format vX.Y.Z with numeric components."
        exit 1
      fi
      VERSION="$1"; 
      shift 
      ;;
    --major) BUMP="major"; shift ;;
    --minor) BUMP="minor"; shift ;;
    --patch) BUMP="patch"; shift ;;
    -m|--message) 
      NOTES="$2"; 
      if [[ -z "$NOTES" ]]; then
        echo "❌ Release notes cannot be empty when using -m/--message."
        exit 1
      fi
      shift 2 
      ;;
    *) echo "❌ Unknown argument: $1"; exit 1 ;;
  esac
done

# --- Detect and Calculate Version ---
if [[ -z "$VERSION" ]]; then
    debug "No version specified, will calculate based on latest tag"
    LATEST_TAG_RAW=$(git describe --tags --abbrev=0 2>/dev/null || echo "")
    debug "Raw tag from git: '$LATEST_TAG_RAW'"

    if [[ -n "$LATEST_TAG_RAW" ]]; then
        LATEST_TAG=$(echo "$LATEST_TAG_RAW" | tr -d '[:space:]')
        echo "🔍 Latest tag found: $LATEST_TAG"
        debug "Processing tag: '$LATEST_TAG'"
        
        # More robust version parsing using bash regex
        if [[ "$LATEST_TAG" =~ ^v([0-9]+)\.([0-9]+)\.([0-9]+)$ ]]; then
            MAJOR="${BASH_REMATCH[1]}"
            MINOR="${BASH_REMATCH[2]}"
            PATCH="${BASH_REMATCH[3]}"
            
            debug "Parsed version components - MAJOR: $MAJOR, MINOR: $MINOR, PATCH: $PATCH"
            
            # Use expr for more reliable arithmetic
            case "$BUMP" in
                major)
                    debug "Applying MAJOR bump"
                    MAJOR=$(expr "$MAJOR" + 1)
                    MINOR=0
                    PATCH=0
                    ;;
                minor)
                    debug "Applying MINOR bump"
                    MINOR=$(expr "$MINOR" + 1)
                    PATCH=0
                    ;;
                patch)
                    debug "Applying PATCH bump"
                    PATCH=$(expr "$PATCH" + 1)
                    ;;
            esac
            
            VERSION="v$MAJOR.$MINOR.$PATCH"
            debug "Calculated new version: $VERSION"
        else
            echo "❌ Invalid latest tag format: '$LATEST_TAG'. Expected format: vX.Y.Z"
            exit 1
        fi
    else
        echo "🔍 No existing tags found. Creating initial release."
        VERSION="$INITIAL_VERSION"
        debug "Using initial version: $VERSION"
    fi
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

# --- Check for tag collision ---
if git rev-parse "$VERSION" >/dev/null 2>&1; then
    echo "❌ Tag $VERSION already exists. Please choose a different version."
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

# Verify artifacts exist
if [[ ! -d "dist" ]]; then
    echo "❌ Error: 'dist' directory not found after build."
    echo "   To recover, you may want to delete the tag:"
    echo "   git tag -d $VERSION && git push --delete origin $VERSION"
    exit 1
fi

# Count files in dist directory
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

# More reliable proxy notification with timeout and error checking
PROXY_TIMEOUT=60  # seconds
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