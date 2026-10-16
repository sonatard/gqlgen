// Package plugintest calls the templates package as a plugin outside of it does, with the
// templates in .gotpl files beside the calling source file.
package plugintest

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/99designs/gqlgen/codegen/templates"
	"github.com/99designs/gqlgen/internal/code"
)

func TestRenderReadsTemplatesBesideCaller(t *testing.T) {
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

func TestImportsOfReadsTemplatesBesideCaller(t *testing.T) {
	imports, err := templates.ImportsOf(templates.Options{
		PackageName: "out",
		Filename:    filepath.Join(t.TempDir(), "out.go"),
		Data:        map[string]string{"Name": "hello"},
		Packages:    code.NewPackages(),
	})
	require.NoError(t, err)
	require.Len(t, imports, 1)
	require.Equal(t, "strings", imports[0].Path)
}
