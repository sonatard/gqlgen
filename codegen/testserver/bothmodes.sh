#!/bin/sh
# bothmodes.sh generates the executor of a test server in both exec modes, so that the
# same tests run against each: go test uses the functions mode, and go test -tags
# exectable uses table mode. Run it from the directory of the config with the arguments
# of gqlgen, the config first. The executor may be generated into another directory.
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

# exec_setting prints the value of the exec setting $1 in the config.
exec_setting() {
	awk -v key="$1:" '
		/^[^ ]/ { exec_section = $0 == "exec:" }
		exec_section && $1 == key { gsub(/"/, "", $2); print $2; exit }
	' "$config"
}

# The functions mode writes its executor into exec.filename, or with the follow-schema
# layout into *.generated.go files in exec.dir.
if [ "$(exec_setting layout)" = follow-schema ]; then
	dir=$(exec_setting dir)
	functions_files="$dir/*.generated.go"
else
	filename=$(exec_setting filename)
	dir=$(dirname "${filename:-generated.go}")
	functions_files=${filename:-generated.go}
fi

# Table mode goes first: the follow-schema layout writes its root file as
# root_.generated.go whatever the file names are, so it is renamed before the
# functions mode writes its own.
$gqlgen -config "$table_config" "$@"
if [ -f "$dir/root_.generated.go" ] && grep -q '^  layout: follow-schema$' "$table_config"; then
	mv -f "$dir/root_.generated.go" "$dir/root_.table.go"
fi
constrain exectable "$dir"/*.table.go

$gqlgen -config "$config" "$@"
for f in $functions_files; do
	if [ -f "$f" ]; then
		constrain '!exectable' "$f"
	fi
done
