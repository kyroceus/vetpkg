#!/usr/bin/env bash
set -euo pipefail

VETPKG_DIR="$(cd "$(dirname "$0")" && pwd)"
BINARY_NAME="vetpkg"
INSTALL_DIR="${HOME}/.local/bin"
CONFIG_DIR="${HOME}/.config/vetpkg"
CONFIG_FILE="${CONFIG_DIR}/config.json"

# --- clean --------------------------------------------------------------------
echo "Cleaning.."
rm -rf "$VETPKG_DIR/vetpkg-bin"

# ── build the binary ──────────────────────────────────────────────────────────
echo "Building vetpkg..."
mkdir "$VETPKG_DIR/vetpkg-bin"
cd "$VETPKG_DIR"
go build -o "$VETPKG_DIR/vetpkg-bin" ./...

# ── install ────────────────────────────────────────────────────────────────────
mkdir -p "$INSTALL_DIR"
cp -r "$VETPKG_DIR/vetpkg-bin/$BINARY_NAME" "$INSTALL_DIR/$BINARY_NAME"
chmod +x "$INSTALL_DIR/$BINARY_NAME"
echo "Installed to $INSTALL_DIR/$BINARY_NAME"

case ":$PATH:" in
    *":$INSTALL_DIR:"*) ;;
    *)
        echo ""
        echo "NOTE: $INSTALL_DIR is not in your PATH."
        echo "Add this line to your shell profile (~/.bashrc, ~/.zshrc, etc.):"
        echo "  export PATH=\"\$HOME/.local/bin:\$PATH\""
        echo ""
        ;;
esac

# ── write config if not present ───────────────────────────────────────────────
mkdir -p "$CONFIG_DIR"
if [[ ! -f "$CONFIG_FILE" ]]; then
    cat > "$CONFIG_FILE" <<EOF
{
  "analyzer": {
    "backend": "claude"
  },
  "claude": {
    "model": "claude-sonnet-4-6"
  },
  "ollama": {
    "endpoint": "http://localhost:11434",
    "model": "llama3.1"
  },
  "general": {
    "makepkg_path": "",
    "auto_approve_low_risk": false
  }
}
EOF
    echo "Created config at $CONFIG_FILE"
else
    echo "Config already exists at $CONFIG_FILE"
fi

echo ""
echo "Installation complete."
echo "Usage: run 'vetpkg' instead of 'makepkg' inside an AUR package directory"
echo "(it reviews the PKGBUILD, then execs the real makepkg with the same args)."
echo ""
echo "Set ANTHROPIC_API_KEY in your environment for Claude analysis."
echo "Or edit $CONFIG_FILE to switch to backend \"ollama\" or \"none\"."
