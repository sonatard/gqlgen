package transport

import (
	jsonv1 "encoding/json"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"io"
)

// jsonDecode reads the first JSON value from r into val using encoding/json/v2.
//
// It runs with jsonv1.DefaultOptionsV1(), the options encoding/json itself is
// built on, so it decodes and reports errors the way encoding/json's Decoder
// with UseNumber, which the transports and the scalar unmarshalers were
// written against, does. The one addition is UseNumber itself: numbers that
// land in an `any` become json.Number, not float64.
//
// Each call gets a fresh Decoder, as each json.NewDecoder call did. Pooling
// Decoders made small requests about 20% faster, but the text of some syntax
// errors depends on how much input the Decoder has buffered, and a pooled
// Decoder's larger buffer made those messages differ.
func jsonDecode(r io.Reader, val any) error {
	err := json.UnmarshalDecode(jsontext.NewDecoder(r, jsonOptions), val, jsonOptions)
	if err != nil && err.Error() == errUnexpectedEnd {
		// (*json.Decoder).Decode reports truncated input this way.
		return io.ErrUnexpectedEOF
	}
	return err
}

// errUnexpectedEnd is the message DefaultOptionsV1 gives truncated input.
const errUnexpectedEnd = "unexpected end of JSON input"

var jsonOptions = json.JoinOptions(
	jsonv1.DefaultOptionsV1(),
	json.WithUnmarshalers(json.UnmarshalFromFunc(unmarshalAnyWithRawNumber)),
)

// unmarshalAnyWithRawNumber stands in for (*json.Decoder).UseNumber, which
// has no public option in encoding/json/v2. It takes over every value headed
// for an `any`, which is where everything inside the Variables and
// Extensions maps lands, and builds the same Go values encoding/json would,
// except that numbers keep their text as a json.Number.
//
// Handling only numbers and returning errors.ErrUnsupported for everything
// else would be shorter, but it made BenchmarkJSONDecode about 2.5 times
// slower, close to encoding/json's own Decoder on Go 1.27.
func unmarshalAnyWithRawNumber(dec *jsontext.Decoder, v *any) (err error) {
	*v, err = decodeAny(dec)
	return err
}

func decodeAny(dec *jsontext.Decoder) (any, error) {
	tok, err := dec.ReadToken()
	if err != nil {
		return nil, err
	}
	switch tok.Kind() {
	case '{':
		obj := make(map[string]any)
		for dec.PeekKind() != '}' {
			name, err := dec.ReadToken()
			if err != nil {
				return nil, err
			}
			// A repeated name overwrites the earlier value, as in encoding/json.
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
	case '0':
		return jsonv1.Number(tok.String()), nil
	case '"':
		return tok.String(), nil
	case 't', 'f':
		return tok.Bool(), nil
	default: // 'n'
		return nil, nil
	}
}
