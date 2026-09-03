#!/bin/sh
# Regenerate the README screenshots from a synthetic session store.
#
#   sh docs/tools/make_images.sh
#
# Renders through TestLiveRender, then converts the captured ANSI frames to SVG.
# Nothing from your own ~/.claude store is read.

set -eu

root="$(cd "$(dirname "$0")/../.." && pwd)"
work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT INT TERM

store="$work/.claude/projects"
python3 "$root/docs/tools/make_demo_store.py" "$store"

# Compile first, with the real environment: the renders run under a fake HOME so
# that paths abbreviate to ~/code/..., and Go cannot build under one.
( cd "$root" && go test -c -o "$work/ui.test" ./internal/ui )

render() {
	out="$1"
	shift
	env -i HOME=/Users/you PATH="$PATH" \
		CL_RENDER_STORE="$store" \
		CL_RENDER_CWD=/Users/you/code/acme-api \
		CL_RENDER_REPO=/Users/you/code/acme-api \
		CL_RENDER_OUT="$out" \
		"$@" \
		"$work/ui.test" -test.run TestLiveRender >/dev/null
}

render "$work/main.ansi" CL_RENDER_COLS=132 CL_RENDER_ROWS=24 CL_RENDER_DOWN=1
render "$work/search.ansi" CL_RENDER_COLS=132 CL_RENDER_ROWS=16 CL_RENDER_QUERY=pagination

python3 "$root/docs/tools/ansi2svg.py" "$work/main.ansi" \
	"$root/docs/demo.svg" "cl — ~/code/acme-api"
python3 "$root/docs/tools/ansi2svg.py" "$work/search.ansi" \
	"$root/docs/search.svg" "cl — searching"

echo "regenerated docs/demo.svg and docs/search.svg"
