package codegen

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestSchemaCopyPath(t *testing.T) {
	require.Equal(t, "schema/a.graphql", schemaCopyPath("schema", "../a.graphql"))
	require.Equal(t, "schema/shared/b.graphql", schemaCopyPath("schema", "../../shared/b.graphql"))
}

func TestCheckSchemaCopies(t *testing.T) {
	require.NoError(t, checkSchemaCopies([]AugmentedSource{
		{Name: "../a.graphql", RelativePath: "schema/a.graphql", Embeddable: true, Copy: true},
		{Name: "../b.graphql", RelativePath: "schema/b.graphql", Embeddable: true, Copy: true},
		{Name: "prelude.graphql", RelativePath: "prelude.graphql", BuiltIn: true},
	}))

	require.EqualError(t, checkSchemaCopies([]AugmentedSource{
		{Name: "../a.graphql", RelativePath: "schema/a.graphql", Embeddable: true, Copy: true},
		{Name: "../../a.graphql", RelativePath: "schema/a.graphql", Embeddable: true, Copy: true},
	}), "embed_schema_dir: ../a.graphql and ../../a.graphql would both be embedded as "+
		"schema/a.graphql; rename one of them")
}

func TestWriteSchemaCopies(t *testing.T) {
	dir := t.TempDir()
	stale := filepath.Join(dir, "schema", "old.graphql")
	require.NoError(t, os.MkdirAll(filepath.Dir(stale), 0o755))
	require.NoError(t, os.WriteFile(stale, []byte("type Old { a: Int }\n"), 0o644))
	sources := []AugmentedSource{
		{
			RelativePath: "schema/a.graphql",
			Embeddable:   true,
			Copy:         true,
			Source:       "type A { a: Int }\n",
		},
		{
			RelativePath: "schema/shared/b.graphql",
			Embeddable:   true,
			Copy:         true,
			Source:       "type B { b: Int }\n",
		},
		// A file inside the directory of the generated code is embedded as it is.
		{RelativePath: "c.graphql", Embeddable: true, Source: "type C { c: Int }\n"},
	}
	a := filepath.Join(dir, "schema", "a.graphql")

	require.NoError(t, writeSchemaCopies(dir, "schema", sources, true))
	content, err := os.ReadFile(a)
	require.NoError(t, err)
	require.Equal(t, schemaCopyNotice+"type A { a: Int }\n", string(content))
	content, err = os.ReadFile(filepath.Join(dir, "schema", "shared", "b.graphql"))
	require.NoError(t, err)
	require.Equal(t, schemaCopyNotice+"type B { b: Int }\n", string(content))
	require.NoFileExists(t, stale)
	require.NoFileExists(t, filepath.Join(dir, "c.graphql"))

	// A copy whose content has not changed keeps its modification time.
	past := time.Now().Add(-time.Hour)
	require.NoError(t, os.Chtimes(a, past, past))
	require.NoError(t, writeSchemaCopies(dir, "schema", sources, true))
	info, err := os.Stat(a)
	require.NoError(t, err)
	require.WithinDuration(t, past, info.ModTime(), time.Second)

	// Without the notice, a copy is the text of the file.
	require.NoError(t, writeSchemaCopies(dir, "schema", sources, false))
	content, err = os.ReadFile(a)
	require.NoError(t, err)
	require.Equal(t, "type A { a: Int }\n", string(content))

	// Without a directory, nothing is written or deleted.
	require.NoError(t, writeSchemaCopies(dir, "", sources, true))
	require.FileExists(t, a)
}
