#!/bin/bash
set -euo pipefail

if command -v go >/dev/null 2>&1; then
    echo "Go is already installed; skipping."
else
    echo "Installing Go..."
    brew install go
fi

if docker desktop version >/dev/null 2>&1 || [[ -d /Applications/Docker.app || -d "$HOME/Applications/Docker.app" ]]; then
    echo "Docker Desktop is already installed; skipping."
else
    echo "Installing Docker Desktop..."
    brew install --cask docker-desktop
fi

if command -v node >/dev/null 2>&1; then
    echo "Node.js is already installed; skipping."
else
    echo "Installing Node.js..."
    brew install node
fi

echo "Go, Docker Desktop, and Node.js are installed."
echo "Docker Desktop is optional for the current local app; open it to complete first-run setup if needed."
echo "From the repository root, run: make dev up"
