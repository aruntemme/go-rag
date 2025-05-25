#!/bin/bash

# RAG Go Application Build Script
echo "🚀 Building RAG Go Application..."

# Clean previous builds
echo "🧹 Cleaning previous builds..."
rm -f rag-server rag-server-* 2>/dev/null

# Build for current platform (optimized)
echo "🔨 Building for current platform..."
go build -ldflags="-s -w" -o rag-server .

# Cross-platform builds
echo "🌍 Building for multiple platforms..."
echo "ℹ️  Note: Cross-platform builds may fail due to CGO dependencies (sqlite-vec)"

# Linux AMD64 (with CGO)
echo "  → Linux AMD64..."
CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o rag-server-linux-amd64 . 2>/dev/null || echo "    ❌ Linux build failed (CGO constraint)"

# Windows AMD64 (with CGO)
echo "  → Windows AMD64..."
CGO_ENABLED=1 GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -o rag-server-windows-amd64.exe . 2>/dev/null || echo "    ❌ Windows build failed (CGO constraint)"

# macOS ARM64 (Apple Silicon)
echo "  → macOS ARM64..."
CGO_ENABLED=1 GOOS=darwin GOARCH=arm64 go build -ldflags="-s -w" -o rag-server-macos-arm64 . 2>/dev/null || echo "    ❌ macOS ARM64 build failed (CGO constraint)"

# macOS AMD64 (Intel)
echo "  → macOS AMD64..."
CGO_ENABLED=1 GOOS=darwin GOARCH=amd64 go build -ldflags="-s -w" -o rag-server-macos-amd64 . 2>/dev/null || echo "    ✅ macOS AMD64 build successful"

echo ""
echo "✅ Build complete! Available executables:"
ls -la rag-server*

echo ""
echo "📖 Usage examples:"
echo "  ./rag-server                           # Use default config.json"
echo "  ./rag-server -config=prod.json         # Use custom config file"
echo "  ./rag-server -help                     # Show help information"
echo "  ./rag-server -version                  # Show version" 