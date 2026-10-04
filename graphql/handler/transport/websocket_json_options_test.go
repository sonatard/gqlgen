package transport_test

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/99designs/gqlgen/graphql"
	"github.com/99designs/gqlgen/graphql/handler/testserver"
	"github.com/99designs/gqlgen/graphql/handler/transport"
)

// TestWebsocketFollowsJSONVersion checks that the JSON mode reaches the
// websocket messages, envelope included, for both subprotocols.
func TestWebsocketFollowsJSONVersion(t *testing.T) {
	newServer := func(version graphql.JSONVersion) *httptest.Server {
		h := testserver.New()
		h.AddTransport(transport.Websocket{
			InitFunc: func(
				ctx context.Context,
				_ transport.InitPayload,
			) (context.Context, *transport.InitPayload, error) {
				return ctx, &transport.InitPayload{"html": "<b>"}, nil
			},
		})
		h.SetJSONVersion(version)
		return httptest.NewServer(h)
	}
	initAck := func(t *testing.T, srv *httptest.Server, subprotocol string) string {
		t.Helper()
		c := wsConnectWithSubprotocol(srv.URL, subprotocol)
		defer c.Close()
		require.NoError(t, c.WriteJSON(&operationMessage{Type: connectionInitMsg}))
		_, data, err := c.ReadMessage()
		require.NoError(t, err)
		return string(data)
	}

	for _, subprotocol := range []string{"graphql-ws", graphqltransportwsSubprotocol} {
		t.Run(subprotocol, func(t *testing.T) {
			defaults := newServer(graphql.JSONv1)
			defer defaults.Close()
			ack := initAck(t, defaults, subprotocol)
			assert.Contains(t, ack, `"type":"connection_ack"`)
			assert.Contains(t, ack, `"html":"\u003cb\u003e"`)

			v2 := newServer(graphql.JSONv2)
			defer v2.Close()
			ack = initAck(t, v2, subprotocol)
			assert.Contains(t, ack, `"type":"connection_ack"`)
			assert.Contains(t, ack, `"html":"<b>"`)
		})
	}
}
