package pi

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"
)

// Install is a pi found on this Mac, and the environment to run it in.
type Install struct {
	Bin string
	// Env is the environment of the user's login shell, with pi's own
	// directory first in its PATH: an app opened in the Finder has
	// neither the PATH that finds node, which runs pi, nor the variables,
	// as API keys, its user's shell sets for pi.
	Env []string
}

// ErrNotFound is the error of a Mac without pi, or none that can be found.
var ErrNotFound = errors.New("pi is not installed")

// Find looks for pi: at path, if not "", or else as the login shell finds
// it, or in the places installers of node's packages put it.
func Find(ctx context.Context, path string) (Install, error) {
	env := ShellEnv(ctx)
	if path != "" {
		path = expandHome(path)
		if !executable(path) {
			return Install{}, ErrNotFound
		}
		return Install{Bin: path, Env: withPath(env, filepath.Dir(path))}, nil
	}
	for _, dir := range filepath.SplitList(lookup(env, "PATH")) {
		if p := filepath.Join(dir, "pi"); dir != "" && executable(p) {
			return Install{Bin: p, Env: withPath(env, dir)}, nil
		}
	}
	for _, p := range candidates() {
		if executable(p) {
			return Install{Bin: p, Env: withPath(env, filepath.Dir(p))}, nil
		}
	}
	return Install{Env: env}, ErrNotFound
}

// candidates are where installers put pi, newest node first.
func candidates() []string {
	home, _ := os.UserHomeDir()
	var out []string
	for _, pattern := range []string{
		filepath.Join(home, ".nvm/versions/node/*/bin/pi"),
		filepath.Join(home, ".fnm/node-versions/*/installation/bin/pi"),
		filepath.Join(home, "Library/Application Support/fnm/node-versions/*/installation/bin/pi"),
		filepath.Join(home, ".asdf/installs/nodejs/*/bin/pi"),
		filepath.Join(home, ".local/share/mise/installs/node/*/bin/pi"),
	} {
		m, _ := filepath.Glob(pattern)
		slices.SortFunc(m, func(a, b string) int { return compareVersions(b, a) })
		out = append(out, m...)
	}
	for _, p := range []string{
		"/opt/homebrew/bin/pi",
		"/usr/local/bin/pi",
		filepath.Join(home, ".volta/bin/pi"),
		filepath.Join(home, ".bun/bin/pi"),
		filepath.Join(home, ".local/bin/pi"),
		filepath.Join(home, ".npm-global/bin/pi"),
		filepath.Join(home, "Library/pnpm/pi"),
	} {
		out = append(out, p)
	}
	return out
}

// compareVersions orders paths by the versions in them, as v22.10.1 after
// v22.9.0.
func compareVersions(a, b string) int {
	na, nb := numbers(a), numbers(b)
	for i := 0; i < len(na) && i < len(nb); i++ {
		if na[i] != nb[i] {
			return na[i] - nb[i]
		}
	}
	return len(na) - len(nb)
}

func numbers(s string) []int {
	var out []int
	for _, f := range strings.FieldsFunc(s, func(r rune) bool { return r < '0' || r > '9' }) {
		n, _ := strconv.Atoi(f)
		out = append(out, n)
	}
	return out
}

func executable(p string) bool {
	st, err := os.Stat(p)
	return err == nil && !st.IsDir() && st.Mode()&0o111 != 0
}

func expandHome(p string) string {
	if rest, ok := strings.CutPrefix(p, "~/"); ok {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, rest)
	}
	return p
}

// ShellEnv is the environment the user's login shell sets up, as a
// terminal has it, or this process's if the shell cannot tell within a
// few seconds.
func ShellEnv(ctx context.Context) []string {
	base := os.Environ()
	shell := lookup(base, "SHELL")
	if shell == "" {
		shell = "/bin/zsh"
	}
	ctx, cancel := context.WithTimeout(ctx, 6*time.Second)
	defer cancel()
	// Interactive, for the variables of .zshrc, and a login shell, for
	// those of .zprofile; what the startup files print goes before the
	// mark.
	const mark = "__NEXTTODO_ENV__"
	cmd := exec.CommandContext(ctx, shell, "-ilc", "printf '"+mark+"'; env -0")
	cmd.Env = append(base, "NEXTTODO_SHELL_ENV=1")
	cmd.Stdin = nil
	cmd.WaitDelay = time.Second
	out, _ := cmd.Output()
	_, after, found := bytes.Cut(out, []byte(mark))
	if !found {
		return base
	}
	var env []string
	for _, kv := range bytes.Split(after, []byte{0}) {
		s := string(kv)
		k, _, ok := strings.Cut(s, "=")
		if !ok || k == "" || k == "_" || k == "SHLVL" || k == "PWD" || k == "OLDPWD" || k == "NEXTTODO_SHELL_ENV" {
			continue
		}
		env = append(env, s)
	}
	if lookup(env, "PATH") == "" {
		return base
	}
	return env
}

// withPath puts dir first in the PATH of env.
func withPath(env []string, dir string) []string {
	path := lookup(env, "PATH")
	parts := []string{dir}
	for _, p := range filepath.SplitList(path) {
		if p != dir && p != "" {
			parts = append(parts, p)
		}
	}
	for _, p := range []string{"/opt/homebrew/bin", "/usr/local/bin", "/usr/bin", "/bin", "/usr/sbin", "/sbin"} {
		if !slices.Contains(parts, p) {
			parts = append(parts, p)
		}
	}
	return Setenv(env, "PATH", strings.Join(parts, string(filepath.ListSeparator)))
}

func lookup(env []string, key string) string {
	for i := len(env) - 1; i >= 0; i-- {
		if v, ok := strings.CutPrefix(env[i], key+"="); ok {
			return v
		}
	}
	return ""
}

// Setenv returns env with key set to value.
func Setenv(env []string, key, value string) []string {
	out := make([]string, 0, len(env)+1)
	for _, kv := range env {
		if !strings.HasPrefix(kv, key+"=") {
			out = append(out, kv)
		}
	}
	return append(out, key+"="+value)
}

// Version is the version of pi, as "1.6.0".
func (in Install) Version(ctx context.Context) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, in.Bin, "--version")
	cmd.Env = in.Env
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	f := strings.Fields(string(out))
	if len(f) == 0 {
		return "", errors.New("pi said no version")
	}
	return strings.TrimPrefix(f[len(f)-1], "v"), nil
}
