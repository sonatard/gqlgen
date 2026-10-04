#!/bin/sh
# bothmodes.sh generates the executor of a test server in both exec modes, so that the
# same tests run against each: go test uses the functions mode, and go test -tags
# exectable uses table mode. Run it from the directory of the test server, which must
# also be the directory the executor is generated into, with the arguments of gqlgen,
# the config first.
#
# The config of table mode is derived from the given one: exec.mode is table, the files
# of the executor are named *.table.go, and the resolver section is dropped, since the
# tests use the stub, which is the same for both modes. The files of each mode get a
# build constraint that leaves out those of the other.
set -eu

config=$1
shift
gqlgen="go run $(dirname "$0")/../../testdata/gqlgen.go"

table_config=.gqlgen.table.yml
trap 'rm -f "$table_config"' EXIT
awk '
	/^[^ ]/ { exec_section = 0; skip = 0 }
	/^resolver:/ { skip = 1 }
	/^skip_validation:/ || skip { next }
	exec_section && /^  filename: / { sub(/\.go$/, ".table.go") }
	exec_section && /^  layout: follow-schema$/ { print; print "  filename_template: \"{name}.table.go\""; next }
	{ print }
	$0 == "exec:" { exec_section = 1; print "  mode: table" }
	END {
		print "skip_validation: true"
		print "go_build_tags:"
		print "  - exectable"
	}
' "$config" >"$table_config"

# constrain puts the build constraint $1 on the files that follow.
constrain() {
	tag=$1
	shift
	for f in "$@"; do
		{
			printf '//go:build %s\n\n' "$tag"
			cat "$f"
		} >"$f.tmp"
		mv -f "$f.tmp" "$f"
	done
}

# Table mode goes first: the follow-schema layout writes its root file as
# root_.generated.go whatever the file names are, so it is renamed before the
# functions mode writes its own.
$gqlgen -config "$table_config" "$@"
if grep -q '^  layout: follow-schema$' "$table_config"; then
	mv -f root_.generated.go root_.table.go
fi
constrain exectable ./*.table.go

$gqlgen -config "$config" "$@"
for f in generated.go ./*.generated.go; do
	if [ -f "$f" ]; then
		constrain '!exectable' "$f"
	fi
done
