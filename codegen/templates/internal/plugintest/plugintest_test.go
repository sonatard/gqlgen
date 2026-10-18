// Package plugintest calls the templates package as a plugin outside of it does, with the
// templates in .gotpl files beside the calling source file.
package plugintest

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/99designs/gqlgen/codegen/templates"
	"github.com/99designs/gqlgen/internal/code"
)

// skipWithoutSourcePaths skips a test when the binary was built with -trimpath, which
// leaves the templates package no path of the calling source file to look in.
func skipWithoutSourcePaths(t *testing.T) {
	t.Helper()
	_, file, _, _ := runtime.Caller(0)
	if _, err := os.Stat(file); err != nil {
		t.Skip("the test binary has no source paths, as with -trimpath")
	}
}

func TestRenderReadsTemplatesBesideCaller(t *testing.T) {
	skipWithoutSourcePaths(t)
	filename := filepath.Join(t.TempDir(), "out.go")
	err := templates.Render(templates.Options{
		PackageName: "out",
		Filename:    filename,
		Data:        map[string]string{"Name": "hello"},
		Packages:    code.NewPackages(),
	})
	require.NoError(t, err)

	b, err := os.ReadFile(filename)
	require.NoError(t, err)
	require.Contains(t, string(b), `strings.ToUpper("hello from the plugin template")`)
}
