package transport

import (
	"fmt"
	"io"

	"github.com/vektah/gqlparser/v2/gqlerror"

	"github.com/99designs/gqlgen/graphql"
	"github.com/99designs/gqlgen/graphql/handler/internal/respjson"
)

func writeJson(w io.Writer, exec graphql.GraphExecutor, response *graphql.Response) {
	b, err := respjson.Marshal(exec, response)
	if err != nil {
		panic(fmt.Errorf("unable to marshal %s: %w", string(response.Data), err))
	}
	w.Write(b)
}

func writeJsonError(w io.Writer, exec graphql.GraphExecutor, msg string) {
	writeJson(w, exec, &graphql.Response{Errors: gqlerror.List{{Message: msg}}})
}

func writeJsonErrorf(w io.Writer, exec graphql.GraphExecutor, format string, args ...any) {
	writeJson(
		w,
		exec,
		&graphql.Response{Errors: gqlerror.List{{Message: fmt.Sprintf(format, args...)}}},
	)
}

func writeJsonGraphqlError(w io.Writer, exec graphql.GraphExecutor, err ...*gqlerror.Error) {
	writeJson(w, exec, &graphql.Response{Errors: err})
}
