// Package dotenv loads PUMC_* settings from an optional .env file next to
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
// deployments that already export PUMC_* don't need to change anything.
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

func unquote(v string) string {
	if len(v) >= 2 {
		if (v[0] == '"' && v[len(v)-1] == '"') || (v[0] == '\'' && v[len(v)-1] == '\'') {
			return v[1 : len(v)-1]
		}
	}
	return v
}
