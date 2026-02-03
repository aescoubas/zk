#!/bin/bash
set -e

# Default install prefix
PREFIX="${1:-/home/escoubas/.local}"
BIN_DIR="$PREFIX/bin"

# Ensure bin directory exists
mkdir -p "$BIN_DIR"

# ANSI colors
GREEN='\033[0;32m'
NC='\033[0m' # No Color

echo -e "${GREEN}Installing Zettelkasten CLI (zk) to $PREFIX...${NC}"

# Check for Go
if ! command -v go &> /dev/null; then
    echo "Error: 'go' is not installed or not in PATH."
    exit 1
fi

# Kill running instances
echo "Stopping running 'zk' processes..."
pkill -x zk || true

# 1. Build
echo "Building binary from source..."
cd tools/zk-go
if go build -tags fts5 -ldflags "-s -w" -o ../../bin/zk ./cmd/zk; then
    echo "Build successful."
else
    echo "Build failed."
    exit 1
fi
cd ../..

# 2. Install Binary
echo "Installing binary to $BIN_DIR/zk..."

install_cmd="cp bin/zk $BIN_DIR/zk"

# Check write permissions
if [ ! -w "$BIN_DIR" ]; then
    echo "Elevated privileges required to write to $BIN_DIR."
    sudo $install_cmd
    sudo chmod 755 "$BIN_DIR/zk"
else
    $install_cmd
    chmod 755 "$BIN_DIR/zk"
fi

# 3. Install Completions
echo "Generating shell completions..."

# Helper to write completion file
install_completion() {
    local shell=$1
    local dir=$2
    local file=$3

    if [ -d "$dir" ]; then
        echo "Installing $shell completion to $dir/$file..."
        local cmd="./bin/zk completion $shell"
        
        if [ ! -w "$dir" ]; then
             sudo sh -c "$cmd > $dir/$file"
        else
             $cmd > "$dir/$file"
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
