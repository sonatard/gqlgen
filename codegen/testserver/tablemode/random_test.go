package tablemode_test

import (
	"fmt"
	"math/rand/v2"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/vektah/gqlparser/v2/ast"

	"github.com/99designs/gqlgen/codegen/testserver/tablemode/functions"
	"github.com/99designs/gqlgen/codegen/testserver/tablemode/table"
)

// TestRandomRequests sends operations built at random from the schema to both
// executors and requires the same responses. The operations follow the schema, so
// that most of them reach the resolvers, and some carry wrong or missing values to
// reach the error paths. A failure prints the seed, which reproduces the request.
func TestRandomRequests(t *testing.T) {
	n := 3000
	if testing.Short() {
		n = 300
	}
	for seed := range uint64(n) {
		compareRandomRequest(t, seed)
	}
}

// FuzzRandomRequests is TestRandomRequests driven by go test -fuzz.
func FuzzRandomRequests(f *testing.F) {
	for seed := range uint64(10) {
		f.Add(seed)
	}
	f.Fuzz(compareRandomRequest)
}

func compareRandomRequest(t *testing.T, seed uint64) {
	t.Helper()
	functionsSchema := functions.NewSchema()
	tableSchema := table.NewSchema()

	g := &generator{
		schema: tableSchema.Schema(),
		rand:   rand.New(rand.NewPCG(seed, seed)),
		vars:   map[string]any{},
	}
	req := g.request()
	want := run(t, functionsSchema, req)
	got := run(t, tableSchema, req)
	if want != got {
		t.Fatalf("seed %d\nquery: %s\nvariables: %v\nfunctions: %s\ntable:     %s",
			seed, req.query, req.variables, want, got)
	}
}

// generator builds a random operation from a schema.
type generator struct {
	schema  *ast.Schema
	rand    *rand.Rand
	vars    map[string]any
	varDefs []string
	aliases int
	labels  int
}

func (g *generator) chance(p float64) bool { return g.rand.Float64() < p }

func pick[T any](g *generator, items []T) T { return items[g.rand.IntN(len(items))] }

func (g *generator) request() request {
	op, root, directive := "query", g.schema.Query, "@logQuery"
	if g.chance(0.15) {
		op, root, directive = "mutation", g.schema.Mutation, "@logMutation"
	}
	sel := g.selection(root, 0)
	var b strings.Builder
	b.WriteString(op)
	if len(g.varDefs) > 0 {
		b.WriteString("(" + strings.Join(g.varDefs, ", ") + ")")
	}
	if g.chance(0.2) {
		b.WriteString(" " + directive)
	}
	b.WriteString(" " + sel)
	return request{query: b.String(), variables: g.vars, quiet: true}
}

// selection returns a selection set for a value of the type def.
func (g *generator) selection(def *ast.Definition, depth int) string {
	var parts []string
	switch def.Kind {
	case ast.Object:
		fields := g.fields(def)
		count := 1 + g.rand.IntN(min(4, len(fields)))
		for _, f := range fields[:count] {
			if part := g.field(f, depth); part != "" {
				parts = append(parts, part)
			}
		}
		if len(parts) == 0 || g.chance(0.1) {
			parts = append(parts, "__typename")
		}
	case ast.Interface, ast.Union:
		parts = append(parts, "__typename")
		if def.Kind == ast.Interface && g.chance(0.5) {
			parts = append(parts, g.field(def.Fields.ForName("id"), depth))
		}
		for _, impl := range g.schema.GetPossibleTypes(def) {
			if !g.chance(0.6) {
				continue
			}
			spread := "... on " + impl.Name
			if depth > 0 && g.chance(0.15) {
				// Labels are unique, as the spec requires.
				g.labels++
				spread += fmt.Sprintf(" @defer(label: %q)", "d"+strconv.Itoa(g.labels))
			}
			parts = append(parts, spread+" "+g.selection(impl, depth+1))
		}
	}
	return "{ " + strings.Join(parts, " ") + " }"
}

// fields returns the fields of def that can be selected, in random order.
func (g *generator) fields(def *ast.Definition) ast.FieldList {
	var fields ast.FieldList
	for _, f := range def.Fields {
		if !strings.HasPrefix(f.Name, "__") {
			fields = append(fields, f)
		}
	}
	g.rand.Shuffle(len(fields), func(i, j int) { fields[i], fields[j] = fields[j], fields[i] })
	return fields
}

// field returns the selection of f, or "" when f has a composite type and the
// selection is already too deep.
func (g *generator) field(f *ast.FieldDefinition, depth int) string {
	def := g.schema.Types[f.Type.Name()]
	composite := def.Kind == ast.Object || def.Kind == ast.Interface || def.Kind == ast.Union
	if composite && depth >= 3 {
		return ""
	}
	var b strings.Builder
	if g.chance(0.3) {
		g.aliases++
		fmt.Fprintf(&b, "a%d: ", g.aliases)
	}
	b.WriteString(f.Name)
	var args []string
	for _, a := range f.Arguments {
		if a.Type.NonNull && a.DefaultValue == nil || g.chance(0.6) {
			args = append(args, a.Name+": "+g.argument(a.Type))
		}
	}
	if len(args) > 0 {
		b.WriteString("(" + strings.Join(args, ", ") + ")")
	}
	switch r := g.rand.Float64(); {
	case r < 0.05:
		b.WriteString(" @skip(if: true)")
	case r < 0.1:
		b.WriteString(" @include(if: false)")
	case r < 0.2:
		b.WriteString(` @trace(label: "t")`)
	}
	if composite {
		b.WriteString(" " + g.selection(def, depth+1))
	}
	return b.String()
}

// argument returns an argument value of type t: mostly a literal, sometimes a
// variable.
func (g *generator) argument(t *ast.Type) string {
	literal, value := g.value(t, 0)
	if g.chance(0.25) {
		name := "v" + strconv.Itoa(len(g.varDefs))
		g.varDefs = append(g.varDefs, "$"+name+": "+t.String())
		g.vars[name] = value
		return "$" + name
	}
	return literal
}

// ids are the IDs the generator uses: those of the test data, and ones that match
// nothing.
var ids = []string{"1000", "1001", "1002", "1003", "2000", "2001", "3000", "3001", "9999", "x"}

// value returns a random value of type t as a GraphQL literal and as the JSON value of
// a variable. A small share of values is invalid for t.
func (g *generator) value(t *ast.Type, depth int) (string, any) {
	if !t.NonNull && g.chance(0.15) {
		return "null", nil
	}
	if g.chance(0.03) {
		i := g.rand.IntN(4)
		return []string{`"wrong"`, "17", "{}", "[]"}[i], []any{"wrong", 17, map[string]any{}, []any{}}[i]
	}
	if t.Elem != nil {
		n := g.rand.IntN(3)
		if g.chance(0.15) {
			// A single value where a list is expected is coerced to a list of one.
			return g.value(t.Elem, depth)
		}
		literals, values := make([]string, n), make([]any, n)
		for i := range n {
			literals[i], values[i] = g.value(t.Elem, depth)
		}
		return "[" + strings.Join(literals, ", ") + "]", values
	}
	def := g.schema.Types[t.NamedType]
	switch def.Kind {
	case ast.Enum:
		v := pick(g, def.EnumValues).Name
		return v, v
	case ast.InputObject:
		return g.input(def, depth)
	}
	switch def.Name {
	case "Int":
		v := pick(g, []int{0, 1, 2, 4, -1, 1000, 2147483647})
		return strconv.Itoa(v), v
	case "Float":
		v := pick(g, []float64{0, 1.5, -2.25, 172})
		return strconv.FormatFloat(v, 'f', -1, 64), v
	case "Boolean":
		v := g.chance(0.5)
		return strconv.FormatBool(v), v
	case "ID":
		v := pick(g, ids)
		return strconv.Quote(v), v
	case "Time":
		v := pick(g, []string{"2020-01-01T00:00:00Z", "2026-05-06T07:08:09Z", "yesterday"})
		return strconv.Quote(v), v
	case "Money":
		v := pick(g, []string{"1.05", "0", "-3", "12.345", "lots"})
		return strconv.Quote(v), v
	case "Map":
		return `{ a: 1, b: "two" }`, map[string]any{"a": 1, "b": "two"}
	}
	v := pick(g, []string{"", "a", "LUKE", "Falcon", "X", "日本語", "a rather long commentary text"})
	return strconv.Quote(v), v
}

// input returns a random value of the input object def.
func (g *generator) input(def *ast.Definition, depth int) (string, any) {
	var literals []string
	values := map[string]any{}
	for _, f := range def.Fields {
		required := f.Type.NonNull && f.DefaultValue == nil
		if !required && (depth > 1 || !g.chance(0.6)) {
			continue
		}
		literal, value := g.value(f.Type, depth+1)
		literals = append(literals, f.Name+": "+literal)
		values[f.Name] = value
	}
	slices.Sort(literals)
	return "{" + strings.Join(literals, ", ") + "}", values
}
