package execbehavior

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/99designs/gqlgen/graphql/handler/extension"
	"github.com/99designs/gqlgen/graphql/introspection"
)

// TestIntrospectionSnapshot compares the responses to the full introspection query, and to
// a query for what it leaves out, with testdata/introspection.json. Run it with UPDATE_SNAPSHOT=1 to rewrite the file after
// changing the schema on purpose.
func TestIntrospectionSnapshot(t *testing.T) {
	srv := newServer(&Stub{}, DirectiveRoot{})
	srv.Use(extension.Introspection{})

	query := func(q string) json.RawMessage {
		body, err := json.Marshal(map[string]any{"query": q})
		require.NoError(t, err)
		req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		srv.ServeHTTP(w, req)
		require.Equal(t, http.StatusOK, w.Code)
		return w.Body.Bytes()
	}
	// The standard query leaves out isOneOf and deprecated input fields.
	snapshot, err := json.Marshal(map[string]json.RawMessage{
		"full": query(introspection.Query),
		"details": query(`{
			oneOf: __type(name: "OneOfInput") { isOneOf inputFields { name } }
			input: __type(name: "DescribedInput") {
				isOneOf
				inputFields(includeDeprecated: true) { name description defaultValue isDeprecated deprecationReason }
			}
		}`),
	})
	require.NoError(t, err)

	var got bytes.Buffer
	require.NoError(t, json.Indent(&got, snapshot, "", "  "))
	got.WriteString("\n")

	const path = "testdata/introspection.json"
	if os.Getenv("UPDATE_SNAPSHOT") != "" {
		require.NoError(t, os.MkdirAll("testdata", 0o755))
		require.NoError(t, os.WriteFile(path, got.Bytes(), 0o644))
	}
	want, err := os.ReadFile(path)
	require.NoError(t, err)
	// Git may check the snapshot out with CRLF line endings on Windows.
	want = bytes.ReplaceAll(want, []byte("\r\n"), []byte("\n"))
	require.Equal(t, string(want), got.String())
}
