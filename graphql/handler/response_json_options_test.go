package handler_test

import (
	"context"
	"encoding/json/jsontext"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/99designs/gqlgen/graphql"
	"github.com/99designs/gqlgen/graphql/handler/testserver"
	"github.com/99designs/gqlgen/graphql/handler/transport"
)

func TestResponsesFollowJSONVersion(t *testing.T) {
	srv := testserver.New()
	srv.AddTransport(transport.GET{})
	srv.AroundResponses(func(ctx context.Context, next graphql.ResponseHandler) *graphql.Response {
		resp := next(ctx)
		resp.Extensions = map[string]any{"html": "<b>"}
		return resp
	})

	query := func() string {
		resp := get(srv, "/foo?query={name}")
		require.Equal(t, http.StatusOK, resp.Code, resp.Body.String())
		return resp.Body.String()
	}

	// JSONv1, the default, writes with encoding/json, which escapes HTML
	// characters.
	assert.JSONEq(t, `{"data":{"name":"test"},"extensions":{"html":"<b>"}}`, query())
	assert.Contains(t, query(), `"html":"\u003cb\u003e"`)

	// JSONv2 writes with encoding/json/v2's defaults, which do not.
	srv.SetJSONVersion(graphql.JSONv2)
	assert.Contains(t, query(), `"html":"<b>"`)

	// Response options are added to them.
	srv.SetResponseJSONOptions(jsontext.EscapeForHTML(true))
	assert.Contains(t, query(), `"html":"\u003cb\u003e"`)

	// and ignored in JSONv1.
	srv.SetResponseJSONOptions(jsontext.EscapeForHTML(false))
	srv.SetJSONVersion(graphql.JSONv1)
	assert.Contains(t, query(), `"html":"\u003cb\u003e"`)
}
