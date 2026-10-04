package client

import (
	"bytes"
	jsonv1 "encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestDecodeJSONMatchesDecoder pins decodeJSON to encoding/json's
// Decoder.Decode, which the incremental HTTP client used before.
func TestDecodeJSONMatchesDecoder(t *testing.T) {
	inputs := []string{
		`{"data":{"a":1},"hasNext":true}`,
		`{"incremental":[{"data":{"b":"<b>"},"path":["a",0],"label":"l"}],"hasNext":false}`,
		`{"data":null,"errors":[{"message":"boom"}]}`,
		`{"data":{"a":1}} {"data":{"a":2}}`,
		`{"data":{"a":1}}trailing`,
		`{"data":{"a":1},"data":{"a":2}}`,
		`{"Data":{"a":1},"HASNEXT":true}`,
		"{\"data\":{\"s\":\"\xff\"}}",
		`{"data":`,
		``,
		`notjson`,
		`{"hasNext":"yes"}`,
	}
	for i, input := range inputs {
		t.Run(fmt.Sprintf("%02d", i), func(t *testing.T) {
			for _, newTarget := range []func() any{
				func() any { var v any = IncrementalInitialResponse{}; return &v },
				func() any { return &IncrementalInitialResponse{} },
				func() any { return &IncrementalResponse{} },
				func() any { return &map[string]any{} },
			} {
				want, got := newTarget(), newTarget()
				wantErr := jsonv1.NewDecoder(bytes.NewReader([]byte(input))).Decode(want)
				gotErr := decodeJSON(bytes.NewReader([]byte(input)), got)
				if wantErr != nil {
					require.EqualError(t, gotErr, wantErr.Error(), "input %q", input)
					continue
				}
				require.NoError(t, gotErr, "input %q", input)
				require.Equal(t, want, got, "input %q", input)
			}
		})
	}
}
