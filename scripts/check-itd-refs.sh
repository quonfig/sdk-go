#!/usr/bin/env bash
# Fail when the workflows in this repo check out integration-test-data at
# different refs, or check it out without a pinned ref (qfg-6vno.7). The
# unit, chaos and failover-chaos jobs must all run against the same ITD tag,
# so the chaos gates cannot silently drift behind the unit pin.
#
# Usage: scripts/check-itd-refs.sh [workflows-dir]   (default .github/workflows)
set -euo pipefail

dir="${1:-.github/workflows}"
shopt -s nullglob
files=("$dir"/*.yml "$dir"/*.yaml)
[ ${#files[@]} -gt 0 ] || { echo "::error::no workflow files under $dir"; exit 1; }

# Print "file<TAB>ref" for every step whose `repository:` is
# quonfig/integration-test-data. A step ends at the next `- ` list item.
# A checkout with no ref prints "<none>".
refs=$(awk '
  function flush() { if (itd) print FILENAME "\t" (ref == "" ? "<none>" : ref); itd = 0; ref = "" }
  FNR == 1 { flush() }
  /^[[:space:]]*- / { flush() }
  /^[[:space:]]*repository:[[:space:]]*quonfig\/integration-test-data[[:space:]]*$/ { itd = 1 }
  /^[[:space:]]*ref:/ { r = $0; sub(/^[[:space:]]*ref:[[:space:]]*/, "", r); sub(/[[:space:]]*(#.*)?$/, "", r); gsub(/["'\'']/, "", r); ref = r }
  END { flush() }
' "${files[@]}")

if [ -z "$refs" ]; then
  echo "::error::no integration-test-data checkout found under $dir"
  exit 1
fi

echo "integration-test-data refs:"
echo "$refs" | sed 's/^/  /'

if echo "$refs" | cut -f2 | grep -qx '<none>'; then
  echo "::error::an integration-test-data checkout has no pinned ref"
  exit 1
fi
distinct=$(echo "$refs" | cut -f2 | sort -u)
if [ "$(echo "$distinct" | wc -l | tr -d ' ')" -ne 1 ]; then
  echo "::error::integration-test-data refs differ across workflows: $(echo "$distinct" | tr '\n' ' ')— bump them together"
  exit 1
fi
echo "OK: every workflow pins integration-test-data at $distinct"
