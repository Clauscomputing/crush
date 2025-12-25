# Crush - AI Coding Assistant

Crush is a terminal-based AI assistant for software development, built by Charm. It provides an interactive TUI for working with LLMs to assist with coding tasks.

## Architecture Overview

```
main.go                     Entry point
internal/
  cmd/                      CLI commands (root, run, projects, logs, etc.)
  app/                      Application orchestration and lifecycle
  agent/                    LLM agent implementation with tools
  config/                   Configuration loading and management
  db/                       SQLite database with migrations
  tui/                      Terminal UI (BubbleTea)
  session/                  Session management
  message/                  Conversation history
  permission/               Tool execution permission system
  shell/                    Cross-platform shell execution
  lsp/                      Language Server Protocol client
  security/                 OpenBSD pledge/unveil sandboxing
```

## Security Model (OpenBSD)

On OpenBSD, Crush uses pledge(2) and unveil(2) to sandbox the application:

### pledge(2) - System Call Restrictions

The application requests these promises at startup:
- `stdio` - Standard I/O operations
- `rpath` - Read filesystem
- `wpath` - Write filesystem
- `cpath` - Create/delete files
- `dpath` - Create/delete directories
- `flock` - File locking (SQLite)
- `fattr` - File attributes
- `proc` - Process operations (fork/exec for LSP, shell)
- `exec` - Execute programs
- `inet` - Network (LLM APIs)
- `dns` - DNS resolution
- `unix` - Unix sockets
- `tty` - Terminal control
- `getpw` - Password database access (user lookup)

At shutdown, promises are reduced to `stdio` only.

### unveil(2) - Filesystem Restrictions

Only these paths are accessible after initialization:

**System (read-only):**
- `/usr/lib`, `/usr/local/lib` - Libraries
- `/usr/bin`, `/usr/local/bin`, `/bin` - Executables
- `/etc/ssl/cert.pem`, `/etc/ssl/certs` - TLS certificates
- `/etc/resolv.conf`, `/etc/hosts` - DNS resolution
- `/etc/passwd` - User database
- `/usr/share/zoneinfo`, `/etc/localtime` - Timezone
- `/dev/null`, `/dev/tty`, `/dev/urandom` - Devices

**User directories (read/write):**
- Data directory (`.crush/`) - Database, logs, config
- Working directory - Project files being edited
- `~/.config` - XDG config directory
- `~/.local/share` - XDG data directory
- `~/.gitconfig` - Git config (read-only)
- `~/go`, `~/.cache/go-build` - Go toolchain cache
- `/tmp/crush-{uid}/` - Temporary files

### Security Initialization

```
1. Load configuration (needs filesystem access)
2. Call security.Init(dataDir, workingDir)
   a. unveil() all required paths
   b. unveil(NULL, NULL) to lock filesystem access
   c. pledge() with all required promises
3. Connect to database
4. Start application
```

The security package is in `internal/security/`:
- `security_openbsd.go` - Real implementation
- `security_stub.go` - No-op for other platforms

### Adding New Paths

If Crush needs access to additional paths, update `setupUnveil()` in `security_openbsd.go`. Common reasons:
- New tool that reads/writes specific locations
- LSP server cache directories
- Additional config file locations

### Debugging

Run with `-d` flag to see debug logs including unveil paths and pledge promises. If the app crashes with `SIGABRT`, check which syscall or path was denied.
