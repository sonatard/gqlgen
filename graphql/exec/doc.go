// Package exec is the runtime for executors generated with exec.mode set to "table".
//
// In that mode gqlgen generates tables that describe the schema's inputs and objects
// instead of one function per field. The tables hold the code that differs per field,
// such as the getter or the resolver call, and this package holds the code that is the
// same for every field: building the field context, running directives and middleware,
// propagating nulls and running fields concurrently.
//
// The values here are created by generated code. EC is the generated executionContext
// type. Functions that unmarshal or marshal a Go value take a context and the execution
// context first, such as unmarshalNString2string(ctx, ec, v).
package exec
