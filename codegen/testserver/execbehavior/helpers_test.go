//go:generate go run ../../../testdata/gqlgen.go -config gqlgen.yml -stub stub.go

package execbehavior

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sort"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/handler/transport"
)

// newServer returns a server for the schema with the given resolvers and directives.
func newServer(resolvers *Stub, directives DirectiveRoot) *handler.Server {
	srv := handler.New(NewExecutableSchema(Config{Resolvers: resolvers, Directives: directives}))
	srv.AddTransport(transport.POST{})
	return srv
}

// post sends query to srv and returns the response body as JSON, with the locations of
// errors removed and errors sorted by message, so that tests can compare it as a whole.
func post(t *testing.T, srv http.Handler, query string) string {
	t.Helper()

	body, err := json.Marshal(map[string]any{"query": query})
	require.NoError(t, err)
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp), w.Body.String())
	if errs, ok := resp["errors"].([]any); ok {
		for _, e := range errs {
			delete(e.(map[string]any), "locations")
		}
		sort.Slice(errs, func(i, j int) bool {
			return errs[i].(map[string]any)["message"].(string) < errs[j].(map[string]any)["message"].(string)
		})
	}
	out, err := json.Marshal(resp)
	require.NoError(t, err)
	return string(out)
}
