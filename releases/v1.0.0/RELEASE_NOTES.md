# RAG Go Application v1.0.0 🚀

## 🎉 Major Release: Command-Line Interface & Enhanced Build System

This release introduces comprehensive command-line argument support and a robust cross-platform build system, making the RAG application much more flexible for deployment and configuration management.

## ✨ New Features

### 🔧 Command-Line Interface
- **Config Path Arguments**: Use `-config=path/to/config.json` to specify custom configuration files
- **Help System**: Run `./rag-server -help` for comprehensive usage information  
- **Version Display**: Use `./rag-server -version` to show application version
- **Backward Compatibility**: Still works with default `config.json` when no arguments provided

### 🚀 Enhanced Build System
- **Cross-Platform Build Script**: New `build.sh` script for building on multiple platforms
- **Optimized Builds**: Production builds with `-ldflags="-s -w"` for smaller executables
- **CGO Handling**: Proper CGO configuration for sqlite-vec dependencies
- **Multiple Architectures**: Support for macOS (Intel/ARM), Linux, and Windows

### 📚 Comprehensive Documentation
- **Updated README**: Complete CLI documentation with examples
- **Build Instructions**: Detailed build and deployment guide
- **Docker Support**: Updated Dockerfile with CLI argument support
- **Environment Configs**: Examples for dev/staging/production deployments

## 🔧 Technical Improvements

### Configuration Management
- Removed automatic config loading from `init()` function
- Added command-line flag parsing with `flag` package
- Enhanced error handling for missing config files
- Support for absolute and relative config paths

### Build & Deployment
- Single-command multi-platform builds
- Graceful handling of CGO constraints
- Environment-specific configuration examples
- Docker integration with volume mounting support

## 📦 Release Assets

This release includes the following pre-built binaries:

- **`rag-server-darwin-amd64`**: macOS Intel (x86_64)
- **`rag-server-macos-amd64`**: macOS Intel (alternative)
- **`build.sh`**: Cross-platform build script
- **`config-example.json`**: Example configuration file

## 🚀 Quick Start

```bash
# Download the binary for your platform
chmod +x rag-server-*

# Run with default config
./rag-server-darwin-amd64

# Run with custom config  
./rag-server-darwin-amd64 -config=my-config.json

# Show help
./rag-server-darwin-amd64 -help
```

## 🔄 Migration Guide

### From Previous Versions
No breaking changes - existing `config.json` files continue to work without modification.

### New Deployments
1. Download the appropriate binary for your platform
2. Copy `config-example.json` to `config.json` and customize
3. Run `./rag-server -config=config.json`

## 🛠️ Development

### Building from Source
```bash
git clone https://github.com/aruntemme/go-rag.git
cd go-rag
go mod tidy

# Quick build
go build -ldflags="-s -w" -o rag-server .

# Multi-platform build
chmod +x build.sh && ./build.sh
```

## 📋 System Requirements

- **Runtime**: No additional dependencies (statically linked)
- **LLM Service**: OpenAI-compatible API (LlamaCPP, Ollama, OpenAI, etc.)
- **Platforms**: macOS, Linux, Windows (x86_64)

## 🔗 Advanced Features

All existing RAG features remain available:
- ✅ Intelligent adaptive chunking (6 strategies)
- ✅ Resume and biblical text processing
- ✅ Parent-child chunk relationships  
- ✅ Semantic search with metadata filtering
- ✅ Query expansion and re-ranking
- ✅ SQLite-vec vector storage
- ✅ RESTful API with comprehensive endpoints

## 🐛 Bug Fixes

- Fixed config loading order in main.go
- Improved error handling for missing configuration files
- Enhanced build script error reporting
- Updated Docker configuration for CGO dependencies

## 📞 Support

- 📖 **Documentation**: [README.md](README.md) | [API Reference](API_REFERENCE.md)
- 🐛 **Issues**: [GitHub Issues](https://github.com/aruntemme/go-rag/issues)
- 💬 **Discussions**: [GitHub Discussions](https://github.com/aruntemme/go-rag/discussions)

---

**Full Changelog**: https://github.com/aruntemme/go-rag/compare/v0.9.0...v1.0.0