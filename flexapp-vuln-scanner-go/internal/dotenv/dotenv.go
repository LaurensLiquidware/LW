// Package dotenv loads FVS_* settings from an optional .env file next to
// where the server is run, so an operator can set them once instead of
// exporting environment variables before every start (README.md,
// ".env.example" already documented this workflow; nothing actually read
// the file until now).
package dotenv

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
)

// Parse reads KEY=VALUE pairs, one per line. Blank lines and lines whose
// first non-space character is '#' are ignored. A value may be wrapped in
// single or double quotes, which are stripped; otherwise it is used as-is
// with only leading/trailing whitespace trimmed -- no variable expansion,
// no multiline values, no "export " prefix. That covers what
// .env.example needs without pulling in a parsing dependency for a format
// this deliberately small.
func Parse(r io.Reader) (map[string]string, error) {
	out := make(map[string]string)
	scanner := bufio.NewScanner(r)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			return nil, fmt.Errorf("line %d: missing '=' in %q", lineNum, line)
		}
		key = strings.TrimSpace(key)
		if key == "" {
			return nil, fmt.Errorf("line %d: empty key", lineNum)
		}
		out[key] = unquote(strings.TrimSpace(value))
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

// Load reads path (typically ".env" in the current directory -- the
// server's other relative defaults, like the sqlite file and TLS cert,
// are already cwd-relative, so this matches) and sets each key as a
// process environment variable, skipping any key that's already set. A
// real environment variable therefore always wins over the file, so
// deployments that already export FVS_* don't need to change anything.
// A missing file is not an error -- the file is optional, matching
// .env.example's own "copy to .env and fill in" framing.
func Load(path string) error {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	defer f.Close()

	kv, err := Parse(f)
	if err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	for k, v := range kv {
		if _, exists := os.LookupEnv(k); exists {
			continue
		}
		if err := os.Setenv(k, v); err != nil {
			return err
		}
	}
	return nil
}

// SetValue updates path's KEY=VALUE line to value, preserving every other
// line (including comments and blank lines) verbatim, and creates path if
// it doesn't exist yet. If key already appears (matched the same way
// Parse extracts it -- first "=" on the line, trimmed), that line is
// replaced in place; otherwise a new "KEY=VALUE" line is appended.
func SetValue(path, key, value string) error {
	var lines []string
	if data, err := os.ReadFile(path); err == nil {
		lines = strings.Split(strings.TrimRight(string(data), "\n"), "\n")
		if len(lines) == 1 && lines[0] == "" {
			lines = nil
		}
	} else if !os.IsNotExist(err) {
		return err
	}

	newLine := key + "=" + value
	replaced := false
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		k, _, ok := strings.Cut(trimmed, "=")
		if ok && strings.TrimSpace(k) == key {
			lines[i] = newLine
			replaced = true
			break
		}
	}
	if !replaced {
		lines = append(lines, newLine)
	}

	return os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o600)
}

func unquote(v string) string {
	if len(v) >= 2 {
		if (v[0] == '"' && v[len(v)-1] == '"') || (v[0] == '\'' && v[len(v)-1] == '\'') {
			return v[1 : len(v)-1]
		}
	}
	return v
}
