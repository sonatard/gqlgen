package exec

import (
	"context"
	"fmt"

	"github.com/99designs/gqlgen/graphql"
)

// Arg describes one argument of a field.
type Arg[EC any] struct {
	Name string
	// Directives are the directives declared on the argument. They receive the map of
	// raw arguments.
	Directives []Directive[EC]
	// NilOK is set when a directive may return nil for the argument.
	NilOK bool
	// WithNull runs the directives even when the argument is absent, as set by
	// call_argument_directives_with_null.
	WithNull bool
	// Type unmarshals the argument.
	Type *In[EC]
}

// ParseArgs unmarshals rawArgs into a map that holds a value of the argument's Go type
// for every argument, absent ones included.
func ParseArgs[EC any](
	ctx context.Context,
	ec EC,
	args []Arg[EC],
	rawArgs map[string]any,
) (map[string]any, error) {
	return parseArgs(ctx, ec, args, rawArgs, true)
}

// parseArgs is ParseArgs. withPath adds the name of each argument to the path of its
// errors. The functions mode leaves it out for the arguments of the directives in the
// schema, which it unmarshals in place.
func parseArgs[EC any](
	ctx context.Context,
	ec EC,
	args []Arg[EC],
	rawArgs map[string]any,
	withPath bool,
) (map[string]any, error) {
	res := make(map[string]any, len(args))
	for i := range args {
		v, err := args[i].parse(ctx, ec, rawArgs, withPath)
		if err != nil {
			return nil, err
		}
		res[args[i].Name] = v
	}
	return res, nil
}

func (a *Arg[EC]) parse(
	ctx context.Context,
	ec EC,
	rawArgs map[string]any,
	withPath bool,
) (any, error) {
	raw, ok := rawArgs[a.Name]
	if len(a.Directives) == 0 {
		if !ok {
			return a.Type.zero, nil
		}
		if withPath {
			ctx = graphql.WithPathContext(ctx, graphql.NewPathWithField(a.Name))
		}
		return a.Type.unmarshal(ctx, ec, raw)
	}

	if !ok && !a.WithNull {
		return a.Type.zero, nil
	}
	ctx = graphql.WithPathContext(ctx, graphql.NewPathWithField(a.Name))
	next := func(ctx context.Context) (any, error) {
		if !ok {
			return a.Type.zero, nil
		}
		return a.Type.unmarshal(ctx, ec, raw)
	}
	tmp, err := Chain(ec, rawArgs, a.Directives, next)(ctx)
	if err != nil {
		return a.Type.zero, graphql.ErrorOnPath(ctx, err)
	}
	if a.Type.accept(tmp) {
		return tmp, nil
	}
	if a.NilOK && tmp == nil {
		return a.Type.zero, nil
	}
	return a.Type.zero, graphql.ErrorOnPath(ctx, fmt.Errorf(
		"unexpected type %T from directive, should be %s", tmp, a.Type.name))
}
