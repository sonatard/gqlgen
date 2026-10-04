// Package execbehavior pins down how the generated executor behaves at runtime, so that a
// change to how the executor is generated can be checked against it. It covers arguments
// and input coercion, nullability, resolver errors and panics, directives, @defer,
// subscriptions, complexity and introspection.
//
// Each behavior has its own schema and test file. The tests compare whole responses, data
// and errors together, so that a change in any part of a response shows.
//
// Some tests pin a known bug: behavior that differs from what the spec or the
// documentation says. Their expected response is the current one, and a comment marks
// them: a line that starts with "KNOWN BUG:" says what happens now, and a line that
// starts with "Expected:" says what should happen and why. A change that fixes such a
// bug updates the expected response and removes the comment.
//
// The subpackages hold the schemas that need a different gqlgen.yml.
package execbehavior
