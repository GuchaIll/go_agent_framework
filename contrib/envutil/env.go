package envutil

import (
	"bufio"
	"os"
	"strings"
)

// Load reads a .env file (if it exists) and sets any variables that are not
// already present in the environment. Lines starting with # are ignored.
func Load(path string) {
	f, err := os.Open(path)
	if err != nil {
		return // missing file is fine
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		val = strings.TrimSpace(val)
		// Strip surrounding quotes.
		val = strings.Trim(val, `"'`)
		// Only set if not already in the environment.
		if os.Getenv(key) == "" {
			os.Setenv(key, val)
		}
	}
}
