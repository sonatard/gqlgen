package client

import (
	"net/http"

	"github.com/99designs/gqlgen/graphql"
)

// JSONVersion selects the JSON package the request is written and the
// response is read with. graphql.JSONv1, the default, uses encoding/json and
// graphql.JSONv2 uses encoding/json/v2 with its default behavior. Pass it to
// New to apply it to every request; a request option applies it to that
// request only, and must come before WithFiles.
func JSONVersion(v graphql.JSONVersion) Option {
	return func(bd *Request) {
		bd.jsonVersion = v
	}
}

// Var adds a variable into the outgoing request
func Var(name string, value any) Option {
	return func(bd *Request) {
		if bd.Variables == nil {
			bd.Variables = map[string]any{}
		}

		bd.Variables[name] = value
	}
}

// Operation sets the operation name for the outgoing request
func Operation(name string) Option {
	return func(bd *Request) {
		bd.OperationName = name
	}
}

// Extensions sets the extensions to be sent with the outgoing request
func Extensions(extensions map[string]any) Option {
	return func(bd *Request) {
		bd.Extensions = extensions
	}
}

// Path sets the url that this request will be made against, useful if you are mounting your entire
// router
// and need to specify the url to the graphql endpoint.
func Path(url string) Option {
	return func(bd *Request) {
		bd.HTTP.URL.Path = url
	}
}

// AddHeader adds a header to the outgoing request. This is useful for setting expected
// Authentication headers for example.
func AddHeader(key, value string) Option {
	return func(bd *Request) {
		bd.HTTP.Header.Add(key, value)
	}
}

// BasicAuth authenticates the request using http basic auth.
func BasicAuth(username, password string) Option {
	return func(bd *Request) {
		bd.HTTP.SetBasicAuth(username, password)
	}
}

// AddCookie adds a cookie to the outgoing request
func AddCookie(cookie *http.Cookie) Option {
	return func(bd *Request) {
		bd.HTTP.AddCookie(cookie)
	}
}
