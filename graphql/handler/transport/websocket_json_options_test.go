package transport_test

import (
	"context"
	"net/http/httptest"
	"testing"
	"time"

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

// TestWebsocketPingPongJSONv2 checks that the server's ping and pong-only
// messages, which carry no payload, are written in the JSONv2 mode.
// encoding/json/v2 cannot marshal an empty, non-nil json.RawMessage.
func TestWebsocketPingPongJSONv2(t *testing.T) {
	for msgType, ws := range map[string]transport.Websocket{
		graphqltransportwsPingMsg: {PingPongInterval: 20 * time.Millisecond},
		graphqltransportwsPongMsg: {PongOnlyInterval: 20 * time.Millisecond},
	} {
		t.Run(msgType, func(t *testing.T) {
			h := testserver.New()
			h.AddTransport(ws)
			h.SetJSONVersion(graphql.JSONv2)
			srv := httptest.NewServer(h)
			defer srv.Close()

			c := wsConnectWithSubprotocol(srv.URL, graphqltransportwsSubprotocol)
			defer c.Close()
			c.SetReadDeadline(time.Now().Add(2 * time.Second))

			require.NoError(
				t,
				c.WriteJSON(&operationMessage{Type: graphqltransportwsConnectionInitMsg}),
			)
			var msg operationMessage
			require.NoError(t, c.ReadJSON(&msg))
			assert.Equal(t, graphqltransportwsConnectionAckMsg, msg.Type)
			require.NoError(t, c.ReadJSON(&msg))
			assert.Equal(t, msgType, msg.Type)
		})
	}
}
