package tablemode_test

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"slices"
	"sort"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/vektah/gqlparser/v2"
	"github.com/vektah/gqlparser/v2/ast"
	"github.com/vektah/gqlparser/v2/gqlerror"

	"github.com/99designs/gqlgen/codegen/testserver/tablemode/functions"
	"github.com/99designs/gqlgen/codegen/testserver/tablemode/table"
	"github.com/99designs/gqlgen/graphql"
	"github.com/99designs/gqlgen/graphql/executor"
	"github.com/99designs/gqlgen/graphql/handler/extension"
	"github.com/99designs/gqlgen/graphql/introspection"
)

type request struct {
	name      string
	query     string
	variables map[string]any
	// contains lists strings the response must contain, so that two executors that
	// fail the same way cannot pass by matching each other.
	contains []string
	// complexityLimit enables the fixed complexity limit extension when it is set.
	complexityLimit int
	// noIntrospection leaves out the introspection extension, as servers that disable
	// introspection do.
	noIntrospection bool
}

var requests = []request{
	{
		name: "characters",
		query: `{
			hero { __typename id name appearsIn friends { __typename name ... on Human { height(unit: FOOT) } ... on Droid { primaryFunction serial } } }
			luke: human(id: "1000") { id name height mass starships { id name } secret nickname tags wallet friends { name } }
			ship: starship(id: "3000") { name length(unit: FOOT) history }
			han: human(id: "1002") { name nickname height(unit: METER) }
			vader: human(id: "1001") { name starships { name } }
			droid(id: "2000") { id name primaryFunction serial appearsIn friends { name } }
		}`,
		contains: []string{`"LUKE"`, `"Solo"`, `"12.34"`, `"Protocol"`, `starships are classified`},
	},
	{
		name:     "non-null error propagates to the nullable parent",
		query:    `{ droid(id: "2001") { name primaryFunction } other: droid(id: "2000") { name } }`,
		contains: []string{`droid has no function`, `"C-3PO"`},
	},
	{
		name:     "missing droid",
		query:    `{ droid(id: "9999") { name } }`,
		contains: []string{`no droid 9999`},
	},
	{
		name:     "union",
		query:    `{ search(text: "a") { __typename ... on Human { name } ... on Droid { name serial } ... on Starship { id name history } } }`,
		contains: []string{`"Falcon"`, `"HAN"`},
	},
	{
		name:     "non-null list element error nulls the whole response",
		query:    `{ search(text: "X") { ... on Starship { name length } } }`,
		contains: []string{`unknown length`},
	},
	{
		name:     "interfaces",
		query:    `{ node(id: "2000") { id __typename ... on Droid { name } } human: node(id: "1000") { id ... on Character { name } } missing: node(id: "x") { id } }`,
		contains: []string{`"C-3PO"`},
	},
	{
		name:      "variables and custom scalars",
		query:     `query($e: Episode!, $since: Time) { reviews(episode: $e, since: $since) { stars commentary time tags episode } }`,
		variables: map[string]any{"e": "JEDI", "since": "2020-01-01T00:00:00Z"},
		contains:  []string{`"2026-01-02T03:04:05Z"`},
	},
	{
		name: "input with defaults, nesting and directives",
		query: `{ echoReview(input: {
			stars: 4, commentary: "fine", time: "2026-05-06T07:08:09Z", note: "dropped",
			nested: { numbers: [1, 2], deeper: { value: "deep" } }, wallet: "1.05"
		}) { stars commentary time tags episode } }`,
		contains: []string{
			`"fresh"`,
			`"default"`,
			`"deep"`,
			`"wallet:105"`,
			`"validated"`,
			`"JEDI"`,
		},
	},
	{
		name:  "input from variables",
		query: `query($in: ReviewInput!) { echoReview(input: $in) { stars commentary tags episode } }`,
		variables: map[string]any{
			"in": map[string]any{"stars": 1, "tags": []any{"x"}, "episode": "EMPIRE"},
		},
		contains: []string{`"EMPIRE"`, `"validated"`},
	},
	{
		name:     "input field directive error",
		query:    `{ echoReview(input: { stars: 1, commentary: "this commentary is far too long" }) { stars } }`,
		contains: []string{`out of range`},
	},
	{
		name:     "input object directive error",
		query:    `{ echoReview(input: { stars: 6 }) { stars } }`,
		contains: []string{`at most 5 stars`},
	},
	{
		name:      "invalid custom scalar in input",
		query:     `query($in: ReviewInput!) { echoReview(input: $in) { stars } }`,
		variables: map[string]any{"in": map[string]any{"stars": 1, "wallet": "abc"}},
		contains:  []string{`invalid money`},
	},
	{
		name:     "list of inputs",
		query:    `{ a: echoReviews(inputs: [{ stars: 1 }, { stars: 2, tags: [] }]) { stars tags } b: echoReviews(inputs: null) { stars } c: echoReviews(inputs: []) { stars } }`,
		contains: []string{`"validated"`},
	},
	{
		name:     "map input",
		query:    `{ a: echoMap(input: { size: 3, role: USER }) b: echoMap(input: null) c: echoMap }`,
		contains: []string{`unnamed`, `USER`},
	},
	{
		name:     "input field set by a resolver",
		query:    `{ a: echoResolved(input: { label: "l", shout: "hey" }) b: echoResolved(input: { label: "l" }) }`,
		contains: []string{`"l/HEY"`},
	},
	{
		name:     "input field resolver error",
		query:    `{ echoResolved(input: { label: "l", shout: "" }) }`,
		contains: []string{`shout must not be empty`},
	},
	{
		name:     "omittable input fields",
		query:    `{ a: echoOmittable(input: { value: null }) b: echoOmittable(input: { count: 3 }) c: echoOmittable(input: {}) }`,
		contains: []string{`"nil,unset"`, `"unset,3"`, `"unset,unset"`},
	},
	{
		name:     "argument directive returning nil",
		query:    `{ echoNullified(value: "x") }`,
		contains: []string{`"nil"`},
	},
	{
		name:     "argument directive error",
		query:    `{ search(text: "") { __typename } }`,
		contains: []string{`out of range`},
	},
	{
		name: "type that marshals itself, bound to a converted scalar",
		query: `{ human(id: "1000") { code } a: echoReview(input: { stars: 1, code: "#x" }) { code }
			b: echoReview(input: { stars: 1 }) { code } }`,
		contains: []string{`"#luke"`, `"#x"`},
	},
	{
		name:     "invalid value of a type that marshals itself",
		query:    `{ echoReview(input: { stars: 1, code: "" }) { code } }`,
		contains: []string{`code must be a non-empty string`},
	},
	{
		name:     "scalars and enums",
		query:    `{ money(amount: "2.50") roles }`,
		contains: []string{`"5.00"`, `"GUEST"`},
	},
	{
		name:     "argument defaults",
		query:    `{ a: withDefaults b: withDefaults(a: 5, b: "y", c: [JEDI, NEWHOPE], d: { value: "w" }) c: withDefaults(a: null, c: null) }`,
		contains: []string{`"1|x|[EMPIRE]|v|[1 2]"`},
	},
	{
		name:  "null propagation",
		query: `{ nullChain { required optional child { required optional } other: child { optional requiredChild { required optional } } } }`,
		contains: []string{
			`"grandchild"`,
			`cannot return null for non-null field NullChain.required (String!)`,
		},
	},
	{
		name:     "errors and panics",
		query:    `{ failingOptional panicking summary { label count extra query { roles summary { label } } } }`,
		contains: []string{`optional failure`, `internal system error`, `"summary"`},
	},
	{
		name:     "non-null root error nulls data",
		query:    `{ failing summary { label } }`,
		contains: []string{`failing on purpose`},
	},
	{
		name:     "field and object directives",
		query:    `{ secretNumber human(id: "1000") { name secret } droid(id: "2000") { name } }`,
		contains: []string{`role ADMIN required`, `"LUKE"`, `"C-3PO"`},
	},
	{
		name:     "query directives",
		query:    `query @logQuery { human(id: "1002") { name @trace(label: "t") nickname @trace(label: "n") } summary { label @trace } }`,
		contains: []string{`"HAN [t]"`, `"Solo [n]"`},
	},
	{
		name:     "mutation",
		query:    `mutation @logMutation { createReview(episode: EMPIRE, review: { stars: 3, commentary: "ok" }) { stars commentary episode tags } setRole(role: GUEST) }`,
		contains: []string{`"EMPIRE"`, `"GUEST"`},
	},
	{
		name:     "defer",
		query:    `{ human(id: "1000") { name ... @defer(label: "ships") { starships { name } } ... @defer { friends { name } } } }`,
		contains: []string{`"ships"`, `"Falcon"`, `"hasNext":true`},
	},
	{
		name:     "type introspection",
		query:    `{ __type(name: "Human") { name kind fields { name args { name defaultValue } type { kind name ofType { kind name } } } interfaces { name } } }`,
		contains: []string{`"Character"`, `"METER"`},
	},
	{
		name:     "full introspection",
		query:    introspection.Query,
		contains: []string{`"ReviewInput"`, `"validReview"`},
	},
	{
		name:            "complexity uses the arguments",
		query:           `{ a: human(id: "1000") { height } b: human(id: "1002") { height(unit: FOOT) } }`,
		complexityLimit: 50,
		contains:        []string{`exceeds the limit of 50`},
	},
	{
		name:            "complexity under the limit",
		query:           `{ a: human(id: "1000") { height } }`,
		complexityLimit: 50,
		contains:        []string{`"height":1.72`},
	},
	{
		name:            "schema introspection when introspection is disabled",
		query:           `{ __schema { queryType { name } } roles }`,
		noIntrospection: true,
		contains:        []string{`introspection disabled`, `"ADMIN"`},
	},
	{
		name:     "typename on the root",
		query:    `{ __typename }`,
		contains: []string{`"Query"`},
	},
}

func TestTableModeMatchesFunctionsMode(t *testing.T) {
	functionsSchema := functions.NewSchema()
	tableSchema := table.NewSchema()

	for _, req := range requests {
		t.Run(req.name, func(t *testing.T) {
			want := run(t, functionsSchema, req)
			got := run(t, tableSchema, req)
			require.Equal(t, want, got)
			for _, s := range req.contains {
				require.Contains(t, want, s)
			}
		})
	}
}

// TestCustomSchema checks that an executable schema given a schema of its own in the
// Config answers as the functions mode does, which validates requests against that schema
// and answers introspection from it, but resolves the fields as the code was generated:
// such a schema may leave out a field to hide it, or declare the arguments of a field in
// another order.
func TestCustomSchema(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(s *ast.Schema)
		query  string
	}{
		{
			name: "a field left out",
			change: func(s *ast.Schema) {
				q := s.Types["Query"]
				q.Fields = slices.DeleteFunc(q.Fields, func(f *ast.FieldDefinition) bool {
					return f.Name == "secretNumber"
				})
			},
			query: `{ roles summary { label } }`,
		},
		{
			name: "arguments in another order",
			change: func(s *ast.Schema) {
				slices.Reverse(s.Types["Query"].Fields.ForName("withDefaults").Arguments)
			},
			query: `{ a: withDefaults b: withDefaults(a: 5, b: "y", d: { value: "w" }) }`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := request{query: tc.query}
			want := run(t, schemaFor(t, functions.NewSchemaFor, tc.change), req)
			got := run(t, schemaFor(t, table.NewSchemaFor, tc.change), req)
			require.Equal(t, want, got)
		})
	}
}

// schemaFor returns the executable schema that newSchema makes with the schema of this
// test server, parsed afresh and changed by change, in its Config.
func schemaFor(
	t *testing.T,
	newSchema func(*ast.Schema) graphql.ExecutableSchema,
	change func(*ast.Schema),
) (es graphql.ExecutableSchema) {
	t.Helper()
	src, err := os.ReadFile("schema.graphql")
	require.NoError(t, err)
	s, err := gqlparser.LoadSchema(&ast.Source{Name: "schema.graphql", Input: string(src)})
	require.NoError(t, err)
	change(s)
	require.NotPanics(t, func() { es = newSchema(s) })
	return es
}

// run executes req and returns every response, including deferred ones, as JSON.
func run(t *testing.T, es graphql.ExecutableSchema, req request) string {
	t.Helper()

	exec := executor.New(es)
	if !req.noIntrospection {
		exec.Use(extension.Introspection{})
	}
	if req.complexityLimit > 0 {
		exec.Use(extension.FixedComplexityLimit(req.complexityLimit))
	}

	// Decode the variables from JSON as the HTTP transports do, with numbers as
	// json.Number.
	var variables map[string]any
	if req.variables != nil {
		b, err := json.Marshal(req.variables)
		require.NoError(t, err)
		d := json.NewDecoder(bytes.NewReader(b))
		d.UseNumber()
		require.NoError(t, d.Decode(&variables))
	}

	ctx := graphql.StartOperationTrace(context.Background())
	params := &graphql.RawParams{Query: req.query, Variables: variables}
	opCtx, errs := exec.CreateOperationContext(ctx, params)
	if errs != nil {
		resp := exec.DispatchError(graphql.WithOperationContext(ctx, opCtx), errs)
		return marshal(t, []*graphql.Response{resp})
	}

	handler, ctx := exec.DispatchOperation(ctx, opCtx)
	var responses []*graphql.Response
	for resp := handler(ctx); resp != nil; resp = handler(ctx) {
		responses = append(responses, resp)
		// A request of these tests defers a few fragments at most; many more responses
		// mean the executor does not finish.
		require.Less(t, len(responses), 200, "the executor keeps returning responses")
	}
	return marshal(t, responses)
}

// marshal returns the responses as JSON in an order that does not depend on the order in
// which concurrent resolvers finished: errors are sorted, and deferred responses after the
// first one are sorted by label and path.
func marshal(t *testing.T, responses []*graphql.Response) string {
	t.Helper()

	for _, resp := range responses {
		// Sort by the whole error, as two errors may share a path and a message and
		// differ in their extensions.
		key := make(map[*gqlerror.Error]string, len(resp.Errors))
		for _, err := range resp.Errors {
			b, jsonErr := json.Marshal(err)
			require.NoError(t, jsonErr)
			key[err] = string(b)
		}
		sort.SliceStable(resp.Errors, func(i, j int) bool {
			return key[resp.Errors[i]] < key[resp.Errors[j]]
		})
	}
	if len(responses) > 1 {
		// Every response but the last says that more follow.
		for i, resp := range responses {
			require.NotNil(t, resp.HasNext, "hasNext of response %d", i)
			require.Equal(t, i < len(responses)-1, *resp.HasNext, "hasNext of response %d", i)
		}
		rest := responses[1:]
		sort.SliceStable(rest, func(i, j int) bool {
			if rest[i].Label != rest[j].Label {
				return rest[i].Label < rest[j].Label
			}
			return rest[i].Path.String() < rest[j].Path.String()
		})
		// Which deferred response comes last depends on timing.
		for _, resp := range rest {
			resp.HasNext = nil
		}
	}

	b, err := json.Marshal(responses)
	require.NoError(t, err)
	return string(b)
}

func BenchmarkModes(b *testing.B) {
	query := `{
		hero { __typename id name appearsIn friends { __typename name ... on Human { height(unit: FOOT) } ... on Droid { primaryFunction } } }
		human(id: "1000") { id name height mass starships { id name } nickname tags wallet friends { name } }
		reviews(episode: JEDI) { stars commentary time tags episode }
		echoReview(input: { stars: 4, commentary: "fine", nested: { numbers: [1, 2] } }) { stars tags }
	}`
	for _, mode := range []struct {
		name   string
		schema graphql.ExecutableSchema
	}{
		{"functions", functions.NewSchema()},
		{"table", table.NewSchema()},
	} {
		b.Run(mode.name, func(b *testing.B) {
			exec := executor.New(mode.schema)
			b.ReportAllocs()
			for b.Loop() {
				ctx := graphql.StartOperationTrace(context.Background())
				opCtx, errs := exec.CreateOperationContext(ctx, &graphql.RawParams{Query: query})
				if errs != nil {
					b.Fatal(errs)
				}
				handler, ctx := exec.DispatchOperation(ctx, opCtx)
				if resp := handler(ctx); len(resp.Errors) > 0 {
					b.Fatal(resp.Errors)
				}
			}
		})
	}
}
