package codegen

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/vektah/gqlparser/v2/ast"

	"github.com/99designs/gqlgen/codegen/config"
)

func TestCheckTableMode(t *testing.T) {
	newData := func() *Data {
		return &Data{Config: &config.Config{Exec: config.ExecConfig{Mode: config.ExecModeTable}}}
	}

	require.NoError(t, checkTableMode(newData()))

	data := newData()
	data.SubscriptionRoot = &Object{}
	require.ErrorContains(t, checkTableMode(data), "does not support subscriptions")

	data = newData()
	data.Config.Federation = config.PackageConfig{Filename: "federation.go"}
	require.ErrorContains(t, checkTableMode(data), "does not support federation")

	data = newData()
	obj := &Object{Definition: &ast.Definition{Name: "User"}}
	obj.Fields = []*Field{
		{FieldDefinition: &ast.FieldDefinition{Name: "posts"}, Batch: true, Object: obj},
	}
	data.Objects = Objects{obj}
	require.EqualError(
		t,
		checkTableMode(data),
		"exec.mode table does not support batch resolvers yet (User.posts); use exec.mode functions",
	)
}

func TestObjectMarshal(t *testing.T) {
	schema := &ast.Schema{Types: map[string]*ast.Definition{
		"Query":     {Name: "Query", Kind: ast.Object},
		"User":      {Name: "User", Kind: ast.Object},
		"Character": {Name: "Character", Kind: ast.Interface},
	}}
	data := &Data{Config: &config.Config{Schema: schema}}

	require.Equal(t, "ec._User(ctx, sel, &v)", data.ObjectMarshal("User", "sel", "&v"))
	require.Equal(t, "ec._Query(ctx, sel)", data.ObjectMarshal("Query", "sel", ""))

	data.Config.UseFunctionSyntaxForExecutionContext = true
	require.Equal(t, "_User(ctx, ec, sel, v)", data.ObjectMarshal("User", "sel", "v"))

	data.Config.UseFunctionSyntaxForExecutionContext = false
	data.Config.Exec.Mode = config.ExecModeTable
	require.Equal(
		t,
		"objectUser.Marshal(ctx, ec, sel, &v)",
		data.ObjectMarshal("User", "sel", "&v"),
	)
	require.Equal(
		t,
		"objectQuery.Marshal(ctx, ec, sel, nil)",
		data.ObjectMarshal("Query", "sel", ""),
	)
	require.Equal(t, "_Character(ctx, ec, sel, v)", data.ObjectMarshal("Character", "sel", "v"))
}

func TestDirectiveTableUse(t *testing.T) {
	d := &Directive{
		Name: "length",
		Args: []*FieldArgument{
			{ArgumentDefinition: &ast.ArgumentDefinition{Name: "min"}, Value: int64(1)},
			{ArgumentDefinition: &ast.ArgumentDefinition{Name: "max"}, Default: int64(20)},
			{ArgumentDefinition: &ast.ArgumentDefinition{Name: "pattern"}},
		},
	}
	require.Equal(t, "directiveLength", d.TableVar())
	require.Equal(t, `directiveLength.With(map[string]any{"min": 1, "max": 20})`, d.TableUse())
	require.Equal(t, `directiveUpper.With(nil)`, (&Directive{Name: "upper"}).TableUse())
	require.Equal(
		t,
		`exec.Dirs(directiveUpper.With(nil), directiveLength.With(map[string]any{"min": 1, "max": 20}))`,
		tableUses([]*Directive{{Name: "upper"}, d}),
	)
}
