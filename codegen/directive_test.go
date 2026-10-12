package codegen

import (
	"go/types"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/vektah/gqlparser/v2/ast"

	"github.com/99designs/gqlgen/codegen/config"
)

func TestDirectiveCallArgs(t *testing.T) {
	d := &Directive{
		Args: []*FieldArgument{
			{
				ArgumentDefinition: &ast.ArgumentDefinition{
					Name: "def1",
				},
				TypeReference: &config.TypeReference{
					GO: types.Default(
						types.NewNamed(
							types.NewTypeName(0, nil, "string", nil),
							types.Typ[types.String],
							nil,
						),
					),
				},
			},
			{
				ArgumentDefinition: &ast.ArgumentDefinition{
					Name: "def2",
				},
				TypeReference: &config.TypeReference{
					GO: types.Default(types.NewStruct(nil, nil)),
				},
			},
		},
	}

	got := d.CallArgs()

	assert.Equal(t, `ctx, obj, n, args["def1"].(string), args["def2"].(struct{})`, got)
}

func TestDirectiveArgVarNames(t *testing.T) {
	args := func(names ...string) ast.ArgumentDefinitionList {
		list := make(ast.ArgumentDefinitionList, len(names))
		for i, name := range names {
			list[i] = &ast.ArgumentDefinition{Name: name}
		}
		return list
	}

	t.Run("names of the generated code get a suffix", func(t *testing.T) {
		assert.Equal(t, map[string]string{
			"obj":     "objArg",
			"rawArgs": "rawArgsArg",
			"asMap":   "asMapArg",
			"it":      "itArg",
			"nil":     "nilArg",
			"true":    "trueArg",
			"false":   "falseArg",
			"value":   "value",
		}, directiveArgVarNames(args("obj", "rawArgs", "asMap", "it", "nil", "true", "false", "value")))
	})

	t.Run("the suffix does not take the name of another argument", func(t *testing.T) {
		assert.Equal(t, map[string]string{
			"rawArgs":       "rawArgsArgArgArg",
			"rawArgsArg":    "rawArgsArg",
			"rawArgsArgArg": "rawArgsArgArg",
		}, directiveArgVarNames(args("rawArgs", "rawArgsArg", "rawArgsArgArg")))
	})
}
