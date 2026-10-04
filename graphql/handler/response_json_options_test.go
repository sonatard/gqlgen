package handler_test

import (
	"context"
	"encoding/json/v2"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/99designs/gqlgen/graphql"
	"github.com/99designs/gqlgen/graphql/handler/testserver"
	"github.com/99designs/gqlgen/graphql/handler/transport"
)

func TestSetResponseJSONOptions(t *testing.T) {
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

	// By default responses are written as encoding/json wrote them, with HTML
	// characters escaped.
	assert.JSONEq(t, `{"data":{"name":"test"},"extensions":{"html":"<b>"}}`, query())
	assert.Contains(t, query(), `"html":"\u003cb\u003e"`)

	srv.SetResponseJSONOptions(json.DefaultOptionsV2())
	assert.Contains(t, query(), `"html":"<b>"`)

	srv.SetResponseJSONOptions(nil)
	assert.Contains(t, query(), `"html":"\u003cb\u003e"`)
}
