package transport

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/99designs/gqlgen/graphql"
)

// TestJSONDecodeMatchesEncodingJSON pins the encoding/json/v2 decoder to the
// encoding/json decoder it replaced: for every input both either fail with
// the same message or produce identical parameters.
func TestJSONDecodeMatchesEncodingJSON(t *testing.T) {
	for i, body := range encodingJSONSeeds() {
		t.Run(fmt.Sprintf("%02d", i), func(t *testing.T) {
			requireDecodesLikeEncodingJSON(t, []byte(body))
		})
	}
}

// FuzzJSONDecodeMatchesEncodingJSON checks the same property as
// TestJSONDecodeMatchesEncodingJSON against inputs the fuzzer generates.
func FuzzJSONDecodeMatchesEncodingJSON(f *testing.F) {
	for _, body := range encodingJSONSeeds() {
		f.Add([]byte(body))
	}
	f.Fuzz(requireDecodesLikeEncodingJSON)
}

func requireDecodesLikeEncodingJSON(t *testing.T, body []byte) {
	t.Helper()

	var want graphql.RawParams
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.UseNumber()
	wantErr := dec.Decode(&want)

	var got graphql.RawParams
	gotErr := jsonDecode(bytes.NewReader(body), &got)

	if wantErr != nil {
		require.EqualErrorf(t, gotErr, wantErr.Error(), "body %q", body)
		return
	}
	require.NoErrorf(t, gotErr, "encoding/json accepted %q", body)
	require.Equal(t, want, got, "body %q", body)
}

// encodingJSONSeeds returns the FuzzJSONDecode seeds plus the inputs where
// the two packages' defaults are known to differ.
func encodingJSONSeeds() []string {
	return append(jsonDecodeSeeds(),
		`{"Query":"{ a }","OPERATIONNAME":"Q","Variables":{"A":1}}`,
		`{"operation_name":"Q","operation-name":"R","Extensions_":{}}`,
		`{"query":"a","Query":"b"}`,
		`{"variables":{"x":1},"VARIABLES":{"y":2}}`,
		`{"variables":`+strings.Repeat(`[`, 10001)+strings.Repeat(`]`, 10001)+`}`,
		"{\"query\":\"\xff\",\"variables\":{\"s\":\"\xc0\xaf\",\"\xff\":1}}",
		`{"variables":{"a":[{},[],{"b":[1,"x",null,true,{"c":{}}]}],"s":"\u00e9\n\"q\""}}`,
		`{"variables":{"x":}}`,
		`{"variables":{"x":1 "y":2}}`,
		`{"variables":{"x":[1,]}}`,
		`{"variables":{"x":tru}}`,
		`{"query":"\udc00 \ud800\udc00"}`,
		`{"query":"{ a }","query":"{ b }","variables":{"x":1,"x":2}}`,
		`{"variables":{"z":null,"o":{},"l":[],"n":-0,"e":1E+2,"d":0.10}}`,
		`{"variables":{"deep":[[[[[[[[[[1]]]]]]]]]]}}`,
		`{"extensions":{"persistedQuery":{"version":1.0}}}`,
		` {"query":"{ a }"} `,
		"\n{\"query\":\"{ a }\"}\n",
		`{"query":"{ a }","variables":null,"extensions":null}`,
		`{"query":null}`,
		`{"unknown":"member","query":"{ a }"}`,
		`{"variables":{"x":1}} 2`,
		`{"variables":{"x":1}}}`,
		// An invalid escape that straddles the Decoder's first 64-byte read,
		// where the error message quotes only the bytes read so far.
		`{"00000000":{"00":"00","0": {"0000":"0","00000":"000000","0\u0X00`,
	)
}
