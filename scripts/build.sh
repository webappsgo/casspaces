#!/bin/bash
set -e

echo "🏗️  Building CasjayDev Workspaces..."

# Build for current platform
go build -ldflags="-w -s" -o casspaces ./cmd/casspaces

echo "✅ Build complete: casspaces"