package transport_test

import (
	"context"
	"encoding/json/v2"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/99designs/gqlgen/graphql/handler/testserver"
	"github.com/99designs/gqlgen/graphql/handler/transport"
)

// TestWebsocketResponseJSONOptions checks that the options set with
// SetResponseJSONOptions reach the websocket messages, envelope included,
// for both subprotocols.
func TestWebsocketResponseJSONOptions(t *testing.T) {
	newServer := func(opts json.Options) *httptest.Server {
		h := testserver.New()
		h.AddTransport(transport.Websocket{
			InitFunc: func(
				ctx context.Context,
				_ transport.InitPayload,
			) (context.Context, *transport.InitPayload, error) {
				return ctx, &transport.InitPayload{"html": "<b>"}, nil
			},
		})
		h.SetResponseJSONOptions(opts)
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
			defaults := newServer(nil)
			defer defaults.Close()
			ack := initAck(t, defaults, subprotocol)
			assert.Contains(t, ack, `"type":"connection_ack"`)
			assert.Contains(t, ack, `"html":"\u003cb\u003e"`)

			v2 := newServer(json.DefaultOptionsV2())
			defer v2.Close()
			ack = initAck(t, v2, subprotocol)
			assert.Contains(t, ack, `"type":"connection_ack"`)
			assert.Contains(t, ack, `"html":"<b>"`)
		})
	}
}
