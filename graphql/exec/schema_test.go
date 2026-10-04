package exec

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/vektah/gqlparser/v2"
	"github.com/vektah/gqlparser/v2/ast"

	"github.com/99designs/gqlgen/graphql"
)

const linkSchema = `
directive @tag(name: String!, level: Int = 1) on FIELD_DEFINITION | OBJECT | ARGUMENT_DEFINITION | INPUT_FIELD_DEFINITION | SCALAR
directive @onInput(x: Int) on INPUT_OBJECT | OBJECT
directive @goField(name: String) on FIELD_DEFINITION

scalar Time @tag(name: "time")

interface Node {
	id: ID!
}

union Pet = Dog | Cat

type Dog implements Node @tag(name: "dog") @onInput {
	id: ID!
	name: String
}

type Cat implements Node {
	id: ID!
}

type Query {
	dog(id: ID! @tag(name: "arg"), limit: Int): Dog @tag(name: "field", level: 2) @goField(name: "x")
	pets: [Pet!]!
	node: Node
	when: Time!
	query: Query!
}

type Mutation {
	rename(input: RenameInput!): Dog
}

input RenameInput @onInput(x: 3) {
	id: ID!
	name: String = "anon" @tag(name: "in")
}
`

func linkTestSchema(t *testing.T) (map[string]*Object[*testEC], *Input[*testEC]) {
	t.Helper()
	s := NewSchema[*testEC](gqlparser.MustLoadSchema(&ast.Source{Input: linkSchema}))
	s.ArgumentDirectivesWithNull = true
	call := func(context.Context, *testEC, any, graphql.Resolver, map[string]any) (any, error) {
		return nil, nil
	}
	var tag, onInput DirectiveDef[*testEC]
	tag.Init(s, "tag", []*In[*testEC]{inString.In, inInt.In}, call, nil)
	onInput.Init(s, "onInput", []*In[*testEC]{inInt.In}, call, nil)

	objects := map[string]*Object[*testEC]{}
	for _, name := range []string{"Dog", "Cat", "Query", "Mutation"} {
		objects[name] = &Object[*testEC]{}
	}
	objects["Dog"].Init(s, "Dog", nil, []Field[*testEC]{
		{Name: "id"},
		{Name: "name", IsMethod: true},
	})
	objects["Cat"].Init(s, "Cat", nil, []Field[*testEC]{{Name: "id"}})
	objects["Query"].Init(s, "Query", nil, []Field[*testEC]{
		{Name: "dog", IsResolver: true, Args: []any{inString.In, inInt.In}},
		{Name: "pets", IsResolver: true},
		{Name: "node", IsResolver: true},
		{Name: "when"},
		{Name: "query"},
	})
	objects["Mutation"].Init(s, "Mutation", nil, []Field[*testEC]{
		{Name: "rename", IsResolver: true, Args: []any{inInt.In}},
	})
	var in Input[*testEC]
	in.Init(s, Input[*testEC]{Name: "RenameInput"}, []InputField[*testEC]{
		{Name: "id", In: inInt.In},
		{Name: "name", In: inString.In},
	})
	s.Link()
	return objects, &in
}

func TestLinkObjects(t *testing.T) {
	objects, _ := linkTestSchema(t)
	dog, query, mutation := objects["Dog"], objects["Query"], objects["Mutation"]

	require.True(t, query.root)
	require.False(t, dog.root)
	require.ElementsMatch(t, []string{"Dog", "Node", "Pet"}, dog.implementors)
	require.Equal(t, []string{"Query"}, query.implementors)

	// The directives of an object type run around the fields that return it, except
	// those that apply to input objects.
	require.Len(t, dog.directives, 1)
	require.Equal(t, "tag", dog.directives[0].def.Name)
	require.Equal(t, map[string]any{"name": "dog", "level": 1}, dog.directives[0].args)

	id, name := dog.index["id"], dog.index["name"]
	require.True(t, id.nonNull)
	require.Equal(t, "ID", id.leaf)
	require.False(t, id.IsMethod)
	require.False(t, name.nonNull)
	require.True(t, name.IsMethod)
	require.False(t, name.Concurrent)

	f := query.index["dog"]
	require.False(t, f.nonNull)
	require.Same(t, dog, f.children)
	require.True(t, f.IsMethod)
	require.True(t, f.Concurrent)
	// The directive the tables do not implement is left out.
	require.Len(t, f.directives, 1)
	require.Equal(t, map[string]any{"name": "field", "level": 2}, f.directives[0].args)
	require.Len(t, f.args, 2)
	require.Equal(t, "id", f.args[0].name)
	require.Same(t, inString.In, f.args[0].typ)
	require.True(t, f.args[0].nilOK)
	require.True(t, f.args[0].withNull)
	require.Equal(t, map[string]any{"name": "arg", "level": 1}, f.args[0].directives[0].args)
	require.Equal(t, "limit", f.args[1].name)
	require.False(t, f.args[1].nilOK)
	require.Empty(t, f.args[1].directives)

	require.Equal(t, "Pet", query.index["pets"].leaf)
	require.True(t, query.index["pets"].nonNull)
	require.Equal(t, "INTERFACE", query.index["node"].abstract)
	require.Nil(t, query.index["node"].children)

	// The directives of a scalar run around the fields that return it.
	when := query.index["when"]
	require.Equal(t, "Time", when.leaf)
	require.Len(t, when.directives, 1)
	require.Equal(t, map[string]any{"name": "time", "level": 1}, when.directives[0].args)

	require.True(t, query.index["query"].rootValue)
	require.Same(t, query, query.index["query"].children)

	// The fields of the mutation type are not resolved concurrently.
	require.False(t, mutation.index["rename"].Concurrent)
	require.True(t, mutation.index["rename"].IsMethod)
}

func TestLinkInputs(t *testing.T) {
	_, in := linkTestSchema(t)

	require.Len(t, in.directives, 1)
	require.Equal(t, "onInput", in.directives[0].def.Name)
	require.Equal(t, map[string]any{"x": 3}, in.directives[0].args)

	require.Nil(t, in.fields[0].defaultValue)
	require.Empty(t, in.fields[0].directives)
	require.False(t, in.fields[0].nilOK)
	require.Equal(t, "anon", in.fields[1].defaultValue)
	require.Len(t, in.fields[1].directives, 1)
	require.Equal(t, map[string]any{"name": "in", "level": 1}, in.fields[1].directives[0].args)
	require.True(t, in.fields[1].nilOK)
}

func TestLinkMismatch(t *testing.T) {
	schema := gqlparser.MustLoadSchema(&ast.Source{Input: linkSchema})
	call := func(context.Context, *testEC, any, graphql.Resolver, map[string]any) (any, error) {
		return nil, nil
	}

	s := NewSchema[*testEC](schema)
	new(Object[*testEC]).Init(s, "Dog", nil, []Field[*testEC]{{Name: "color"}})
	require.PanicsWithValue(t, "exec: type Dog has no field color", s.Link)

	s = NewSchema[*testEC](schema)
	new(Object[*testEC]).Init(s, "Query", nil, []Field[*testEC]{{Name: "dog"}})
	new(Object[*testEC]).Init(s, "Dog", nil, nil)
	require.PanicsWithValue(t, "exec: field dog has 2 arguments, the tables 0", s.Link)

	s = NewSchema[*testEC](schema)
	new(DirectiveDef[*testEC]).Init(s, "nope", nil, call, nil)
	require.PanicsWithValue(t, `exec: the schema declares no directive "nope"`, s.Link)

	s = NewSchema[*testEC](schema)
	new(Object[*testEC]).Init(s, "Dog", nil, nil)
	require.PanicsWithValue(t, `exec: object "Dog" registered twice`, func() {
		new(Object[*testEC]).Init(s, "Dog", nil, nil)
	})
}

// An entry of Field.Args may give the Go type of the argument that errors name.
func TestArgsWithGoType(t *testing.T) {
	s := NewSchema[*testEC](gqlparser.MustLoadSchema(&ast.Source{
		Input: `type Query { f(a: Int, b: Int): Int }`,
	}))
	decls := s.schema.Query.Fields.ForName("f").Arguments
	args := s.args(decls, []any{inInt.In, WithGoType{Arg: inInt.In, GoType: "int32"}}, "field f")
	require.Len(t, args, 2)
	require.Empty(t, args[0].goType)
	require.Equal(t, "int32", args[1].goType)
	require.Same(t, inInt.In, args[1].typ)
}

// The tables give the order in which the functions mode unmarshals the arguments of a
// field bound to a method.
func TestOrderArgs(t *testing.T) {
	decls := ast.ArgumentDefinitionList{{Name: "a"}, {Name: "b"}}
	args := []arg[*testEC]{{name: "a"}, {name: "b"}}
	names := func(args []arg[*testEC]) []string {
		var res []string
		for _, a := range args {
			res = append(res, a.name)
		}
		return res
	}
	require.Equal(t, []string{"b", "a"}, names(orderArgs(args, decls, []int{1, 0}, "field f")))
	// A method may take one argument in two parameters whose names differ in case only.
	require.Equal(
		t,
		[]string{"a", "a"},
		names(orderArgs(args[:1], decls[:1], []int{0, 0}, "field f")),
	)
	require.PanicsWithValue(t, "exec: field f has no argument 2 bound to Go", func() {
		orderArgs(args, decls, []int{2}, "field f")
	})
}

// The copy of a value written in the schema, which a request may change, shares none of
// its lists and maps, however deep.
func TestCloneValue(t *testing.T) {
	v := map[string]any{"a": []any{map[string]any{"b": []any{1}}}, "c": "d"}
	c := cloneValue(v).(map[string]any)
	require.Equal(t, v, c)
	c["c"] = "changed"
	c["a"].([]any)[0].(map[string]any)["b"].([]any)[0] = 2
	c["a"].([]any)[0].(map[string]any)["e"] = true
	require.Equal(t, map[string]any{"a": []any{map[string]any{"b": []any{1}}}, "c": "d"}, v)
}

func TestAsGenerated(t *testing.T) {
	require.Equal(t, 3, asGenerated(int64(3)))
	require.Equal(t, "s", asGenerated("s"))
	// Floats keep six decimals, as the functions mode writes them with %f.
	require.Equal(t, "0.123457", fmt.Sprint(asGenerated(0.1234567)))
	require.Equal(t, "0", fmt.Sprint(asGenerated(0.0000001)))
	// The generated code writes -0 as a Go constant, which is 0.
	require.Equal(t, "0", fmt.Sprint(asGenerated(math.Copysign(0, -1))))
	require.Equal(
		t,
		map[string]any{"a": []any{1, 2.5, map[string]any{"b": 4}}},
		asGenerated(map[string]any{"a": []any{int64(1), 2.5, map[string]any{"b": int64(4)}}}),
	)
	// The functions mode does not compile an integer that int cannot hold.
	require.Equal(t, math.MaxInt, asGenerated(int64(math.MaxInt)))
	if strconv.IntSize == 32 {
		require.Panics(t, func() { asGenerated(int64(math.MaxInt32) + 1) })
	}
}
