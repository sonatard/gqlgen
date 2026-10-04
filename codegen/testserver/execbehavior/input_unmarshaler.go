package execbehavior

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
)

// Coordinates is an input object that unmarshals itself, and rejects a latitude out of
// range.
type Coordinates struct {
	Lat, Lng float64
}

func (c *Coordinates) UnmarshalGQL(v any) error {
	m, ok := v.(map[string]any)
	if !ok {
		return fmt.Errorf("coordinates must be an object, got %T", v)
	}
	for name, dst := range map[string]*float64{"lat": &c.Lat, "lng": &c.Lng} {
		switch n := m[name].(type) {
		case float64:
			*dst = n
		case int64:
			*dst = float64(n)
		case json.Number:
			f, err := n.Float64()
			if err != nil {
				return err
			}
			*dst = f
		default:
			return fmt.Errorf("%s must be a number, got %T", name, m[name])
		}
	}
	if c.Lat < -90 || c.Lat > 90 {
		return errors.New("latitude out of range")
	}
	return nil
}

func (c Coordinates) MarshalGQL(w io.Writer) {
	_, _ = io.WriteString(w, strconv.Quote(fmt.Sprintf("%g,%g", c.Lat, c.Lng)))
}
