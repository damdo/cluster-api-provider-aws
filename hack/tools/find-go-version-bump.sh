#!/usr/bin/env bash
# find-go-version-bump.sh — find which dependency bump in a commit forced a
# go.mod's `go` directive to jump.
#
# Usage: hack/tools/find-go-version-bump.sh [<gomod-path>] [<commit>]
#
#   <gomod-path>  path to the go.mod to inspect (default: go.mod)
#   <commit>      commit whose parent..commit diff is inspected (default: HEAD)
#
# The script:
#   1. Diffs <gomod-path> between <commit>^ and <commit>.
#   2. Bails out early if the `go` directive did not change.
#   3. Collects every module whose required version changed in that diff.
#   4. Looks up each new version's own go.mod `go` directive (via `go list -m -json`).
#   5. Reports the modules whose directive is >= the new go directive, since
#      those are the ones capable of having forced the bump (MVS picks the
#      max across the module graph).
set -euo pipefail

GOMOD="${1:-go.mod}"
COMMIT="${2:-HEAD}"

if [[ ! -f "$GOMOD" ]]; then
  echo "error: $GOMOD not found" >&2
  exit 1
fi

PARENT="${COMMIT}^"

old_go="$(git show "${PARENT}:${GOMOD}" | awk '$1=="go"{print $2; exit}')"
new_go="$(git show "${COMMIT}:${GOMOD}" | awk '$1=="go"{print $2; exit}')"

if [[ -z "$new_go" ]]; then
  echo "error: no 'go' directive found in ${COMMIT}:${GOMOD}" >&2
  exit 1
fi

if [[ "$old_go" == "$new_go" ]]; then
  echo "The 'go' directive in ${GOMOD} did not change between ${PARENT} and ${COMMIT} (stayed at ${old_go})."
  exit 0
fi

echo "go directive in ${GOMOD}: ${old_go} -> ${new_go}  (${PARENT}..${COMMIT})"
echo

# Collect require-line version bumps: "module oldver -> newver"
declare -A NEW_VER
while IFS=$'\t' read -r mod ver; do
  NEW_VER["$mod"]="$ver"
done < <(
  git show "${COMMIT}:${GOMOD}" |
    grep -E '^\t[^ ]+ v[0-9]' |
    sed -E 's/^\t([^ ]+) (v[^ ]+).*/\1\t\2/'
)

declare -A OLD_VER
while IFS=$'\t' read -r mod ver; do
  OLD_VER["$mod"]="$ver"
done < <(
  git show "${PARENT}:${GOMOD}" |
    grep -E '^\t[^ ]+ v[0-9]' |
    sed -E 's/^\t([^ ]+) (v[^ ]+).*/\1\t\2/'
)

changed=()
for mod in "${!NEW_VER[@]}"; do
  if [[ "${OLD_VER[$mod]:-}" != "${NEW_VER[$mod]}" ]]; then
    changed+=("$mod")
  fi
done

if [[ ${#changed[@]} -eq 0 ]]; then
  echo "No require-line version changes found; the go directive change was likely manual."
  exit 0
fi

echo "Checking the go.mod 'go' directive declared by each of the ${#changed[@]} bumped module(s)..."
echo "(this queries the module proxy for each new version)"
echo

printf '%-65s %-12s %-12s %-10s\n' "MODULE" "OLD" "NEW" "REQUIRES-GO"
printf '%-65s %-12s %-12s %-10s\n' "------" "---" "---" "-----------"

suspects=()
for mod in "${changed[@]}"; do
  new="${NEW_VER[$mod]}"
  old="${OLD_VER[$mod]:-<none>}"
  requires_go="$(go list -m -json "${mod}@${new}" 2>/dev/null | awk -F'"' '/"GoVersion"/{print $4}')"
  requires_go="${requires_go:-?}"
  printf '%-65s %-12s %-12s %-10s\n' "$mod" "$old" "$new" "$requires_go"
  if [[ "$requires_go" != "?" ]]; then
    # keep modules whose own go directive is >= the *old* go version and
    # could plausibly have driven MVS to select new_go
    lowest="$(printf '%s\n%s\n' "$requires_go" "$old_go" | sort -V | head -1)"
    if [[ "$lowest" == "$old_go" && "$requires_go" != "$old_go" ]]; then
      suspects+=("$mod (requires go >= ${requires_go})")
    fi
  fi
done

echo
if [[ ${#suspects[@]} -eq 0 ]]; then
  echo "No single bumped module declares a go directive above ${old_go}."
  echo "The bump may come from a transitive dependency not listed directly in ${GOMOD} -- rerun with 'go mod graph' to dig further."
else
  echo "Likely cause(s) of the ${old_go} -> ${new_go} jump (modules requiring a newer Go than before):"
  for s in "${suspects[@]}"; do
    echo "  - $s"
  done
fi
