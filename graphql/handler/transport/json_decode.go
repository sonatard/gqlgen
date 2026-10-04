package transport

import (
	"bytes"
	jsonv1 "encoding/json"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"io"

	"github.com/99designs/gqlgen/graphql"
	"github.com/99designs/gqlgen/graphql/handler/internal/respjson"
)

// jsonDecode reads the JSON in r into val in exec's JSON mode.
//
// In the JSONv1 mode it reads the first value in r with encoding/json's
// Decoder and UseNumber, so numbers that land in an `any` become json.Number.
//
// In the JSONv2 mode it reads all of r as one value with the encoding/json/v2
// defaults: names match case-sensitively, and duplicate names, invalid UTF-8
// and trailing data are errors. encoding/json/v2 has no UseNumber, so numbers
// that land in an `any` become jsontext.Value, which keeps their text as
// json.Number does, rather than float64, which would lose the precision of
// integers above 2^53.
func jsonDecode(exec graphql.GraphExecutor, r io.Reader, val any) error {
	if !respjson.IsJSONv2(exec) {
		dec := jsonv1.NewDecoder(r)
		dec.UseNumber()
		return dec.Decode(val)
	}
	return json.UnmarshalRead(r, val, decodeOptions)
}

var decodeOptions = json.WithUnmarshalers(json.UnmarshalFromFunc(unmarshalAny))

// unmarshalAny builds every value headed for an `any`, which is where
// everything in the variables and extensions maps lands, as encoding/json/v2
// would, except that numbers become jsontext.Value.
//
// Handling only numbers and leaving everything else to encoding/json/v2 by
// returning errors.ErrUnsupported would be shorter, but it is about 2.5 times
// slower on bodies with many variables.
func unmarshalAny(dec *jsontext.Decoder, v *any) (err error) {
	*v, err = decodeAny(dec)
	return err
}

func decodeAny(dec *jsontext.Decoder) (any, error) {
	if dec.PeekKind() == '0' {
		// ReadValue returns bytes that the next read overwrites.
		num, err := dec.ReadValue()
		return jsontext.Value(bytes.Clone(num)), err
	}
	tok, err := dec.ReadToken()
	if err != nil {
		return nil, err
	}
	switch tok.Kind() {
	case '{':
		obj := make(map[string]any)
		for dec.PeekKind() != '}' {
			// The Decoder rejects duplicate names.
			name, err := dec.ReadToken()
			if err != nil {
				return nil, err
			}
			if obj[name.String()], err = decodeAny(dec); err != nil {
				return nil, err
			}
		}
		_, err = dec.ReadToken()
		return obj, err
	case '[':
		arr := []any{}
		for dec.PeekKind() != ']' {
			val, err := decodeAny(dec)
			if err != nil {
				return nil, err
			}
			arr = append(arr, val)
		}
		_, err = dec.ReadToken()
		return arr, err
	case '"':
		return tok.String(), nil
	case 't', 'f':
		return tok.Bool(), nil
	default: // 'n'
		return nil, nil
	}
}

// jsonUnmarshal unmarshals data into val with the JSON package of exec's
// JSON mode, with the package's defaults. Unlike jsonDecode, numbers that
// land in an `any` become float64 in both modes.
func jsonUnmarshal(exec graphql.GraphExecutor, data []byte, val any) error {
	if !respjson.IsJSONv2(exec) {
		return jsonv1.Unmarshal(data, val)
	}
	return json.Unmarshal(data, val)
}
