  OpenBSD Security Sandboxing Support

  This PR adds pledge/unveil sandboxing for OpenBSD, restricting the application's filesystem access and syscalls.

  Changes

  New package: internal/security/
  - doc.go - Package documentation
  - security_openbsd.go - pledge/unveil implementation for OpenBSD
  - security_stub.go - No-op stubs for other platforms
  - security_test.go - Basic tests

  internal/cmd/root.go
  - Call security.Init() after config is loaded but before DB connection
  - Abort on security initialization failure

  internal/db/connect.go
  - Switch from ncruces/go-sqlite3 (WASM) to mattn/go-sqlite3 (CGO)
  - The WASM driver's path resolution walked the entire filesystem, incompatible with unveil

  internal/shell/coreutils.go
  - Enable Go coreutils on OpenBSD (not just Windows)
  - Unveil restrictions aren't inherited through exec(2), so in-process execution
is required

  go.mod / go.sum
  - Replace ncruces/go-sqlite3 with mattn/go-sqlite3
  - Add golang.org/x/sys as direct dependency (for unix.Pledge/Unveil)

  Security Model

  On OpenBSD:
  1. unveil(2) restricts filesystem to: working directory, data directory, system
paths (libs, certs, DNS config), temp directory
  2. pledge(2) restricts syscalls to: stdio, file ops, network, process management, tty
  3. Verification check attempts to read ~/.ssh/config and aborts if accessible

  Testing

  Tested on OpenBSD 7.8:
  - App starts and connects to database
  - File operations outside unveiled paths are blocked
  - Shell commands respect sandbox (via in-process coreutils)

