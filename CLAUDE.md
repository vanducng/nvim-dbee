# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

nvim-dbee is a database client plugin for NeoVim with a hybrid architecture:
- **Backend**: Go (handles database connections and query execution)
- **Frontend**: Lua (provides NeoVim integration and UI)
- **Communication**: Event-driven architecture between Go backend and Lua frontend

## Essential Commands

### Building and Testing

**Go Backend (run from `dbee/` directory):**
```bash
# Run all Go tests
go test ./...

# Build the binary
go build -o ~/.local/share/nvim/dbee/bin/dbee

# Cross-platform build (uses Zig for CGO support)
./ci/build.sh -o linux -a amd64 -e true -p output_binary
```

**Lua Frontend:**
```bash
# Lint Lua code
luacheck lua/

# Format Lua code (check only)
stylua --check lua/

# Format Lua code (apply changes)
stylua lua/
```

**Markdown:**
```bash
# Check markdown formatting
mdformat --number --wrap 100 --check README.md ARCHITECTURE.md
```

### Development Workflow

1. The plugin binary must be built before use. Install methods:
   - `require("dbee").install()` - Auto-detects best method
   - `require("dbee").install("go")` - Build from source
   - Manual build: `cd dbee && go build`

2. Test changes:
   - Go changes: Run `go test ./...` from `dbee/` directory
   - Lua changes: Use luacheck for linting
   - Integration: Rebuild binary and restart NeoVim

## Architecture Highlights

### Backend Structure (Go)
- `adapters/`: Database-specific implementations (11 databases supported)
  - Each adapter implements common interfaces
  - Uses native Go drivers, no CLI dependencies
- `core/`: Core business logic
  - `builders/`: Convenience builders for common constructs
  - `format/`: Output formatters (CSV, JSON)
  - `mock/`: Testing utilities
- `handler/`: Event handling and communication with frontend
- `plugin/`: NeoVim plugin integration layer

### Frontend Structure (Lua)
- `api/`: Clean separation between UI and core functionality
  - `core.lua`: Backend interaction
  - `ui.lua`: UI manipulation
  - `state.lua`: State management
- `ui/`: Component-based UI system
  - `drawer/`: Tree view for connections/scratchpads
  - `editor/`: Query editor
  - `result/`: Results display with pagination
  - `call_log/`: Query history
- `sources.lua`: Pluggable connection sources (Memory, File, Env)
- `layouts/`: Configurable window management

### Key Design Patterns
- **Adapter Pattern**: Database implementations are pluggable
- **Iterator Pattern**: Results are streamed for performance
- **Event-Driven**: Non-blocking query execution
- **Source Pattern**: Multiple ways to load connections

## Database Support

Supported databases (with build constraints for platform compatibility):
- PostgreSQL, MySQL, SQLite, Oracle, SQL Server
- MongoDB, Redis, ClickHouse
- BigQuery, DuckDB, Redshift

## Important Notes

- The plugin uses a separate binary for database operations
- Cross-platform builds use Zig as a C compiler for CGO
- Pre-built binaries are stored in the nvim-dbee-bucket repository
- Connection strings support templating for secrets: `{{ env "VAR" }}`, `{{ exec "cmd" }}`
- Results are paginated (default 100 rows per page)
- All database operations are non-blocking