// Package security provides OpenBSD pledge/unveil sandboxing for Crush.
//
// On OpenBSD, this package restricts the application's capabilities using:
//
//   - pledge(2): Restricts syscalls. We request all needed promises upfront
//     since pledge can only become MORE restrictive, never less.
//
//   - unveil(2): Restricts filesystem access. We reveal all needed paths
//     before calling unveil(NULL, NULL) to lock the restrictions.
//
// # Pledge Promises
//
// The following pledge promises are requested:
//
//   - stdio:   Basic I/O operations (read, write, close, etc.)
//   - rpath:   Read-only filesystem access (open for reading, stat, readdir)
//   - wpath:   Write access to files (open for writing, chmod, utime)
//   - cpath:   Create/delete files (mkdir, unlink, rename)
//   - dpath:   Create device nodes (needed by some temp file operations)
//   - flock:   File locking (used by SQLite WAL mode)
//   - fattr:   Change file attributes (chmod, chown, utimes)
//   - proc:    Process operations (fork, wait, kill for subprocess management)
//   - exec:    Execute programs (running shell commands, git, etc.)
//   - inet:    Internet socket operations (HTTPS API calls)
//   - dns:     DNS resolution (hostname lookups for API endpoints)
//   - unix:    Unix domain sockets (local IPC, some network libraries)
//   - tty:     Terminal operations (interactive prompts, colored output)
//   - getpw:   Access user database (os.UserHomeDir, username lookups)
//
// # Unveiled Paths
//
// System paths (read-only unless noted):
//
//   - /usr/lib, /usr/local/lib:       Shared libraries for exec'd programs
//   - /usr/bin, /usr/local/bin, /bin: Executables (read+execute)
//   - /sbin:                          System binaries
//   - /etc/ssl/cert.pem, /etc/ssl/certs: TLS certificates for HTTPS
//   - /etc/resolv.conf, /etc/hosts:   DNS and hostname resolution
//   - /etc/passwd:                    User database for os.UserHomeDir
//   - /usr/share/zoneinfo, /etc/localtime: Timezone data
//   - /dev/null, /dev/tty:            Device access (read+write)
//   - /dev/urandom:                   Random number generation
//
// User paths (read+write+create):
//
//   - dataDir:           Application data, database, logs (from config)
//   - workingDir:        Current project directory (read+write+create+execute)
//   - ~/.crush:          Default config location
//   - ~/.config:         XDG config directory
//   - ~/.local/share:    XDG data directory
//   - ~/.gitconfig:      Git configuration (read-only)
//   - $GOPATH:           Go workspace (if set in environment)
//   - $GOCACHE:          Go build cache (if set in environment)
//   - /tmp/crush-$UID:   User-specific temp directory
//
// # Usage
//
//   - Init: Must be called once at startup with dataDir and workingDir.
//     After this call, only the specified paths are accessible.
//
// Note: Shell commands use in-process Go coreutils on OpenBSD because
// unveil restrictions are not inherited through exec(2) - child processes
// start with full filesystem access unless they call unveil themselves.
//
// On non-OpenBSD platforms, these functions are no-ops for compatibility.
package security
