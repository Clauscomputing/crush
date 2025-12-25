// Package shell provides cross-platform shell execution capabilities.
//
// This package provides Shell instances for executing commands with their own
// working directory and environment. Each shell execution is independent.
//
// WINDOWS COMPATIBILITY:
// This implementation provides POSIX shell emulation (mvdan.cc/sh/v3) even on
// Windows. Commands should use forward slashes (/) as path separators to work
// correctly on all platforms.
package shell

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	"github.com/charmbracelet/x/exp/slice"
	"mvdan.cc/sh/moreinterp/coreutils"
	"mvdan.cc/sh/v3/expand"
	"mvdan.cc/sh/v3/interp"
	"mvdan.cc/sh/v3/syntax"
)

// ShellType represents the type of shell to use
type ShellType int

const (
	ShellTypePOSIX ShellType = iota
	ShellTypeCmd
	ShellTypePowerShell
)

// Logger interface for optional logging
type Logger interface {
	InfoPersist(msg string, keysAndValues ...any)
}

// noopLogger is a logger that does nothing
type noopLogger struct{}

func (noopLogger) InfoPersist(msg string, keysAndValues ...any) {}

// BlockFunc is a function that determines if a command should be blocked
type BlockFunc func(args []string) bool

// Shell provides cross-platform shell execution with optional state persistence
type Shell struct {
	env        []string
	cwd        string
	mu         sync.Mutex
	logger     Logger
	blockFuncs []BlockFunc
}

// Options for creating a new shell
type Options struct {
	WorkingDir string
	Env        []string
	Logger     Logger
	BlockFuncs []BlockFunc
}

// NewShell creates a new shell instance with the given options
func NewShell(opts *Options) *Shell {
	if opts == nil {
		opts = &Options{}
	}

	cwd := opts.WorkingDir
	if cwd == "" {
		cwd, _ = os.Getwd()
	}

	env := opts.Env
	if env == nil {
		env = os.Environ()
	}

	logger := opts.Logger
	if logger == nil {
		logger = noopLogger{}
	}

	return &Shell{
		cwd:        cwd,
		env:        env,
		logger:     logger,
		blockFuncs: opts.BlockFuncs,
	}
}

// Exec executes a command in the shell
func (s *Shell) Exec(ctx context.Context, command string) (string, string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.exec(ctx, command)
}

// ExecStream executes a command in the shell with streaming output to provided writers
func (s *Shell) ExecStream(ctx context.Context, command string, stdout, stderr io.Writer) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.execStream(ctx, command, stdout, stderr)
}

// GetWorkingDir returns the current working directory
func (s *Shell) GetWorkingDir() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.cwd
}

// SetWorkingDir sets the working directory
func (s *Shell) SetWorkingDir(dir string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Verify the directory exists
	if _, err := os.Stat(dir); err != nil {
		return fmt.Errorf("directory does not exist: %w", err)
	}

	s.cwd = dir
	return nil
}

// GetEnv returns a copy of the environment variables
func (s *Shell) GetEnv() []string {
	s.mu.Lock()
	defer s.mu.Unlock()

	env := make([]string, len(s.env))
	copy(env, s.env)
	return env
}

// SetEnv sets an environment variable
func (s *Shell) SetEnv(key, value string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Update or add the environment variable
	keyPrefix := key + "="
	for i, env := range s.env {
		if strings.HasPrefix(env, keyPrefix) {
			s.env[i] = keyPrefix + value
			return
		}
	}
	s.env = append(s.env, keyPrefix+value)
}

// SetBlockFuncs sets the command block functions for the shell
func (s *Shell) SetBlockFuncs(blockFuncs []BlockFunc) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.blockFuncs = blockFuncs
}

// CommandsBlocker creates a BlockFunc that blocks exact command matches
func CommandsBlocker(cmds []string) BlockFunc {
	bannedSet := make(map[string]struct{})
	for _, cmd := range cmds {
		bannedSet[cmd] = struct{}{}
	}

	return func(args []string) bool {
		if len(args) == 0 {
			return false
		}
		_, ok := bannedSet[args[0]]
		return ok
	}
}

// ArgumentsBlocker creates a BlockFunc that blocks specific subcommand
func ArgumentsBlocker(cmd string, args []string, flags []string) BlockFunc {
	return func(parts []string) bool {
		if len(parts) == 0 || parts[0] != cmd {
			return false
		}

		argParts, flagParts := splitArgsFlags(parts[1:])
		if len(argParts) < len(args) || len(flagParts) < len(flags) {
			return false
		}

		argsMatch := slices.Equal(argParts[:len(args)], args)
		flagsMatch := slice.IsSubset(flags, flagParts)

		return argsMatch && flagsMatch
	}
}

func splitArgsFlags(parts []string) (args []string, flags []string) {
	args = make([]string, 0, len(parts))
	flags = make([]string, 0, len(parts))
	for _, part := range parts {
		if strings.HasPrefix(part, "-") {
			// Extract flag name before '=' if present
			flag := part
			if idx := strings.IndexByte(part, '='); idx != -1 {
				flag = part[:idx]
			}
			flags = append(flags, flag)
		} else {
			args = append(args, part)
		}
	}
	return args, flags
}

func (s *Shell) blockHandler() func(next interp.ExecHandlerFunc) interp.ExecHandlerFunc {
	return func(next interp.ExecHandlerFunc) interp.ExecHandlerFunc {
		return func(ctx context.Context, args []string) error {
			if len(args) == 0 {
				return next(ctx, args)
			}

			for _, blockFunc := range s.blockFuncs {
				if blockFunc(args) {
					return fmt.Errorf("command is not allowed for security reasons: %s", strings.Join(args, " "))
				}
			}

			return next(ctx, args)
		}
	}
}

// shHandler intercepts execution of sh/bash and runs them in-process
// to ensure they stay within the application's security sandbox.
func (s *Shell) shHandler() func(next interp.ExecHandlerFunc) interp.ExecHandlerFunc {
	return func(next interp.ExecHandlerFunc) interp.ExecHandlerFunc {
		return func(ctx context.Context, args []string) error {
			if len(args) == 0 {
				return next(ctx, args)
			}

			cmd := filepath.Base(args[0])
			if cmd == "sh" || cmd == "bash" {
				var scriptFile string
				var scriptContent string
				var params []string

				// Naive argument parsing for -c or script file
				for i := 1; i < len(args); i++ {
					if args[i] == "-c" {
						if i+1 < len(args) {
							scriptContent = args[i+1]
							// args for -c command start after the content string
							if i+2 < len(args) {
								params = args[i+2:]
							}
							break
						}
						return fmt.Errorf("%s: -c requires an argument", cmd)
					}
					// Stop at first non-flag argument (the script file)
					if !strings.HasPrefix(args[i], "-") {
						scriptFile = args[i]
						// args including scriptFile and subsequent args are params
						params = args[i:]
						break
					}
				}

				hc := interp.HandlerCtx(ctx)
				env := hc.Env
				stdout := hc.Stdout
				stderr := hc.Stderr
				cwd := hc.Dir

				if scriptContent != "" {
					return s.runScript(ctx, scriptContent, params, env, cwd, stdout, stderr)
				}

				if scriptFile != "" {
					content, err := os.ReadFile(scriptFile)
					if err != nil {
						return err
					}
					return s.runScript(ctx, string(content), params, env, cwd, stdout, stderr)
				}

				return fmt.Errorf("%s: interactive mode not supported", cmd)
			}

			return next(ctx, args)
		}
	}
}

// scriptExecutionHandler checks if the command being executed is a shell script
// and if so, runs it in-process to ensure sandbox safety.
func (s *Shell) scriptExecutionHandler() func(next interp.ExecHandlerFunc) interp.ExecHandlerFunc {
	return func(next interp.ExecHandlerFunc) interp.ExecHandlerFunc {
		return func(ctx context.Context, args []string) error {
			if len(args) == 0 {
				return next(ctx, args)
			}

			// Skip explicit "sh" and "bash" commands (handled by shHandler)
			base := filepath.Base(args[0])
			if base == "sh" || base == "bash" {
				return next(ctx, args)
			}

			path := args[0]
			hc := interp.HandlerCtx(ctx)

			// Resolve path if it's not absolute or relative (simple name like "script.sh")
			if !strings.Contains(path, string(os.PathSeparator)) {
				pathEnv := ""
				hc.Env.Each(func(k string, v expand.Variable) bool {
					if k == "PATH" {
						pathEnv = v.Str
						return false
					}
					return true
				})

				if found, err := lookPath(path, pathEnv); err == nil {
					path = found
				} else {
					// Can't resolve, let next handler deal with it (likely fail)
					return next(ctx, args)
				}
			}

			// Check if it's a shell script
			if isShellScript(path) {
				content, err := os.ReadFile(path)
				if err != nil {
					return err
				}

				// Run in-process. args are passed as params ($0, $1, ...).
				return s.runScript(ctx, string(content), args, hc.Env, hc.Dir, hc.Stdout, hc.Stderr)
			}

			return next(ctx, args)
		}
	}
}

func isShellScript(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()

	buf := make([]byte, 128)
	n, err := f.Read(buf)
	if err != nil && err != io.EOF {
		return false
	}
	header := string(buf[:n])

	// Check for standard shebangs
	return strings.HasPrefix(header, "#!") &&
		(strings.Contains(header, "/bin/sh") ||
			strings.Contains(header, "/bin/bash") ||
			strings.Contains(header, " env sh") ||
			strings.Contains(header, " env bash"))
}

func lookPath(file string, pathEnv string) (string, error) {
	if pathEnv == "" {
		return "", os.ErrNotExist
	}
	for _, dir := range filepath.SplitList(pathEnv) {
		if dir == "" {
			dir = "."
		}
		path := filepath.Join(dir, file)
		if info, err := os.Stat(path); err == nil {
			if !info.IsDir() && info.Mode()&0111 != 0 {
				return path, nil
			}
		}
	}
	return "", os.ErrNotExist
}

// runScript runs a shell command/script in a new in-process runner
func (s *Shell) runScript(ctx context.Context, content string, params []string, env expand.Environ, dir string, stdout, stderr io.Writer) error {
	line, err := syntax.NewParser().Parse(strings.NewReader(content), "")
	if err != nil {
		return fmt.Errorf("could not parse script: %w", err)
	}

	runner, err := interp.New(
		interp.StdIO(nil, stdout, stderr),
		interp.Interactive(false),
		interp.Env(env),
		interp.Dir(dir),
		interp.Params(params...),
		interp.ExecHandlers(s.execHandlers()...),
	)
	if err != nil {
		return fmt.Errorf("could not run command: %w", err)
	}

	return runner.Run(ctx, line)
}

// newInterp creates a new interpreter with the current shell state
func (s *Shell) newInterp(stdout, stderr io.Writer) (*interp.Runner, error) {
	return interp.New(
		interp.StdIO(nil, stdout, stderr),
		interp.Interactive(false),
		interp.Env(expand.ListEnviron(s.env...)),
		interp.Dir(s.cwd),
		interp.ExecHandlers(s.execHandlers()...),
	)
}

// updateShellFromRunner updates the shell from the interpreter after execution
func (s *Shell) updateShellFromRunner(runner *interp.Runner) {
	s.cwd = runner.Dir
	s.env = nil
	for name, vr := range runner.Vars {
		s.env = append(s.env, fmt.Sprintf("%s=%s", name, vr.Str))
	}
}

// execCommon is the shared implementation for executing commands
func (s *Shell) execCommon(ctx context.Context, command string, stdout, stderr io.Writer) error {
	line, err := syntax.NewParser().Parse(strings.NewReader(command), "")
	if err != nil {
		return fmt.Errorf("could not parse command: %w", err)
	}

	runner, err := s.newInterp(stdout, stderr)
	if err != nil {
		return fmt.Errorf("could not run command: %w", err)
	}

	err = runner.Run(ctx, line)
	s.updateShellFromRunner(runner)
	s.logger.InfoPersist("command finished", "command", command, "err", err)
	return err
}

// exec executes commands using a cross-platform shell interpreter.
func (s *Shell) exec(ctx context.Context, command string) (string, string, error) {
	var stdout, stderr bytes.Buffer
	err := s.execCommon(ctx, command, &stdout, &stderr)
	return stdout.String(), stderr.String(), err
}

// execStream executes commands using POSIX shell emulation with streaming output
func (s *Shell) execStream(ctx context.Context, command string, stdout, stderr io.Writer) error {
	return s.execCommon(ctx, command, stdout, stderr)
}

func (s *Shell) execHandlers() []func(next interp.ExecHandlerFunc) interp.ExecHandlerFunc {
	handlers := []func(next interp.ExecHandlerFunc) interp.ExecHandlerFunc{
		s.blockHandler(),
	}
	// Always enable safety handlers on OpenBSD, but acceptable everywhere
	if useGoCoreUtils {
		handlers = append(handlers, s.shHandler())
		handlers = append(handlers, s.scriptExecutionHandler())
		handlers = append(handlers, coreutils.ExecHandler)
	}
	return handlers
}

// IsInterrupt checks if an error is due to interruption
func IsInterrupt(err error) bool {
	return errors.Is(err, context.Canceled) ||
		errors.Is(err, context.DeadlineExceeded)
}

// ExitCode extracts the exit code from an error
func ExitCode(err error) int {
	if err == nil {
		return 0
	}
	var exitErr interp.ExitStatus
	if errors.As(err, &exitErr) {
		return int(exitErr)
	}
	return 1
}
