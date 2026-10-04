//go:generate ../bothmodes.sh gqlgen.yml -stub stub.go

package execbehavior

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sort"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/99designs/gqlgen/graphql"
	"github.com/99designs/gqlgen/graphql/executor"
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
	return postVars(t, srv, query, nil)
}

// postVars is post with variables.
func postVars(t *testing.T, srv http.Handler, query string, variables map[string]any) string {
	t.Helper()

	body, err := json.Marshal(map[string]any{"query": query, "variables": variables})
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

// execute runs query on es and returns every response, including the deferred ones
// that follow the first, as a JSON array. The deferred responses are sorted by label
// and path, and the errors of each response by message, so that the result does not
// depend on the order in which concurrent fields finished.
func execute(t *testing.T, es graphql.ExecutableSchema, query string) string {
	t.Helper()

	exec := executor.New(es)
	ctx := graphql.StartOperationTrace(context.Background())
	opCtx, errs := exec.CreateOperationContext(ctx, &graphql.RawParams{Query: query})
	require.Empty(t, errs)
	handler, ctx := exec.DispatchOperation(ctx, opCtx)

	var responses []*graphql.Response
	for range 100 {
		resp := handler(ctx)
		if resp == nil {
			break
		}
		for _, err := range resp.Errors {
			err.Locations = nil
		}
		sort.Slice(
			resp.Errors,
			func(i, j int) bool { return resp.Errors[i].Message < resp.Errors[j].Message },
		)
		responses = append(responses, resp)
	}
	if len(responses) > 1 {
		rest := responses[1:]
		sort.SliceStable(rest, func(i, j int) bool {
			if rest[i].Label != rest[j].Label {
				return rest[i].Label < rest[j].Label
			}
			return rest[i].Path.String() < rest[j].Path.String()
		})
		// Which deferred response comes last, and so says that nothing is left, depends
		// on timing; the first response always says that more follow.
		for _, resp := range rest {
			resp.HasNext = nil
		}
	}
	out, err := json.Marshal(responses)
	require.NoError(t, err)
	return string(out)
}
