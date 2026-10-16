package testserver

import (
	"bufio"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestServersGenerateBothModes requires every test server to generate its executor in
// both exec modes. The table mode tests of CI run the test servers with -tags exectable,
// which passes against the functions mode just the same for a server generated without
// -both-modes, so a server that forgets the flag would never be tested in table mode.
// tablemode compares the modes on packages of its own and is left out.
func TestServersGenerateBothModes(t *testing.T) {
	var lines int
	err := filepath.WalkDir(".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if path == "tablemode" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()
		s := bufio.NewScanner(f)
		for s.Scan() {
			line := s.Text()
			if !strings.HasPrefix(line, "//go:generate go run ") || !strings.Contains(line, "testdata/gqlgen.go") {
				continue
			}
			lines++
			require.Contains(t, line, " -both-modes", "%s: the test server is not generated in table mode", path)
		}
		return s.Err()
	})
	require.NoError(t, err)
	require.NotZero(t, lines, "no go:generate line of a test server found")
}
