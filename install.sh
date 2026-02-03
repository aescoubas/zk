#!/bin/bash
set -e

# Default install prefix
PREFIX="${1:-/usr/local}"
BIN_DIR="$PREFIX/bin"

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

# ---------------------------------------------------------
# Ollama Setup (Semantic Features)
# ---------------------------------------------------------
echo -e "${GREEN}Checking Semantic Search Dependencies (Ollama)...${NC}"

if ! command -v ollama &> /dev/null; then
    echo "Ollama not found. Installing..."
    if curl -fsSL https://ollama.com/install.sh | sh; then
        echo "Ollama installed successfully."
    else
        echo "Failed to install Ollama. Semantic features (zk ask/embed) will not work."
        echo "Please install manually: curl -fsSL https://ollama.com/install.sh | sh"
        # We don't exit here, just warn, so standard zk functions still work
    fi
else
    echo "Ollama is already installed."
fi

if command -v ollama &> /dev/null; then
    # Check if running
    if ! curl -s localhost:11434 > /dev/null; then
        echo "Ollama server is not running."
        # Try systemd first
        if command -v systemctl &> /dev/null; then
            echo "Attempting to start via systemctl..."
            sudo systemctl start ollama || true
        fi
        
        # Check again
        sleep 2
        if ! curl -s localhost:11434 > /dev/null; then
             echo "Starting Ollama in background..."
             ollama serve &> /dev/null &
             
             # Wait loop
             echo -n "Waiting for Ollama to initialize..."
             for i in {1..10}; do
                if curl -s localhost:11434 > /dev/null; then
                    echo " Done."
                    break
                fi
                echo -n "."
                sleep 1
             done
        fi
    fi

    # Pull Model if server is up
    if curl -s localhost:11434 > /dev/null; then
        echo "Ensuring embedding model 'nomic-embed-text' is available..."
        ollama pull nomic-embed-text
    else
        echo "Warning: Could not start Ollama. You may need to run 'ollama serve' manually."
    fi
fi
# ---------------------------------------------------------

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
