#!/bin/bash
set -euo pipefail

usage() {
    cat <<EOF
Usage: ./install.sh [--prefix PATH] [--data-dir PATH]

Options:
  --prefix PATH    Install prefix for the zk binary and completions (default: $HOME/.local)
  --data-dir PATH  Persist the Zettelkasten data root to ~/.config/zk/root
  -h, --help       Show this help message

You can also provide the data directory via ZK_DATA_DIR.
EOF
}

PREFIX="$HOME/.local"
DATA_DIR="${ZK_DATA_DIR:-}"

while [ $# -gt 0 ]; do
    case "$1" in
        --prefix)
            if [ $# -lt 2 ]; then
                echo "Error: --prefix requires a value." >&2
                exit 1
            fi
            PREFIX="$2"
            shift 2
            ;;
        --data-dir)
            if [ $# -lt 2 ]; then
                echo "Error: --data-dir requires a value." >&2
                exit 1
            fi
            DATA_DIR="$2"
            shift 2
            ;;
        -h|--help)
            usage
            exit 0
            ;;
        -*)
            echo "Error: unknown option: $1" >&2
            usage >&2
            exit 1
            ;;
        *)
            if [ "$PREFIX" = "$HOME/.local" ]; then
                PREFIX="$1"
                shift
                continue
            fi
            echo "Error: unexpected argument: $1" >&2
            usage >&2
            exit 1
            ;;
    esac
done

BIN_DIR="$PREFIX/bin"
REPO_ROOT="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
CONFIG_HOME="${XDG_CONFIG_HOME:-$HOME/.config}"
CONFIG_DIR="$CONFIG_HOME/zk"
CONFIG_FILE="$CONFIG_DIR/root"

# Ensure bin directory exists
if [ ! -d "$BIN_DIR" ]; then
    mkdir -p "$BIN_DIR" 2>/dev/null || sudo mkdir -p "$BIN_DIR"
fi

# ANSI colors
GREEN='\033[0;32m'
NC='\033[0m' # No Color

echo -e "${GREEN}Installing Zettelkasten CLI (zk) to $PREFIX...${NC}"

# Check for Go
if ! command -v go &> /dev/null; then
    echo "Error: 'go' is not installed or not in PATH."
    exit 1
fi

# 1. Build
echo "Building binary from source..."
mkdir -p "$REPO_ROOT/bin"
cd "$REPO_ROOT"
if go build -tags fts5 -ldflags "-s -w" -o "$REPO_ROOT/bin/zk" ./cmd/zk; then
    echo "Build successful."
else
    echo "Build failed."
    exit 1
fi

# 2. Install Binary
echo "Installing binary to $BIN_DIR/zk..."
TARGET_BIN="$BIN_DIR/zk"
TEMP_BIN="$BIN_DIR/.zk.tmp.$$"

# Check write permissions
if [ ! -w "$BIN_DIR" ]; then
    echo "Elevated privileges required to write to $BIN_DIR."
    sudo cp "$REPO_ROOT/bin/zk" "$TEMP_BIN"
    sudo chmod 755 "$TEMP_BIN"
    sudo mv -f "$TEMP_BIN" "$TARGET_BIN"
else
    cp "$REPO_ROOT/bin/zk" "$TEMP_BIN"
    chmod 755 "$TEMP_BIN"
    mv -f "$TEMP_BIN" "$TARGET_BIN"
fi

if [ -n "$DATA_DIR" ]; then
    mkdir -p "$CONFIG_DIR"
    printf '%s\n' "$DATA_DIR" > "$CONFIG_FILE"
    echo "Configured Zettelkasten data root in $CONFIG_FILE"
else
    echo "Set --data-dir or ZK_DATA_DIR during installation, or configure the data repo later via ZK_PATH or $CONFIG_FILE"
fi

# 3. Install Completions
echo "Generating shell completions..."

# Helper to write completion file
install_completion() {
    local shell=$1
    local dir=$2
    local file=$3
    local zk_cmd="$BIN_DIR/zk"

    if [ -d "$dir" ]; then
        echo "Installing $shell completion to $dir/$file..."
        
        if [ ! -w "$dir" ]; then
             sudo sh -c "\"$zk_cmd\" completion $shell > \"$dir/$file\""
        else
             "$zk_cmd" completion "$shell" > "$dir/$file"
        fi
    fi
}

# User-level completion paths (if they exist)
# Bash
install_completion "bash" "$PREFIX/share/bash-completion/completions" "zk"
install_completion "bash" "$HOME/.local/share/bash-completion/completions" "zk"

# Zsh
install_completion "zsh" "$PREFIX/share/zsh/site-functions" "_zk"
install_completion "zsh" "$HOME/.zsh/completions" "_zk"

# Fish
install_completion "fish" "$PREFIX/share/fish/vendor_completions.d" "zk.fish"
install_completion "fish" "$HOME/.config/fish/completions" "zk.fish"

echo -e "${GREEN}Installation complete!${NC}"
echo "Run 'zk help' to verify."
echo "Semantic commands such as 'zk ask' and 'zk embed' require a separately managed Ollama setup."
