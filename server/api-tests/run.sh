#!/usr/bin/env bash
set -euo pipefail

env_name="dev"; module=""; folder=""; allow_flaky=0
while [[ $# -gt 0 ]]; do
  case "$1" in
    -e) env_name="$2"; shift 2 ;;
    -m) module="$2"; shift 2 ;;
    -f) folder="$2"; shift 2 ;;
    --allow-flaky) allow_flaky=1; shift ;;
    *) echo "unknown arg: $1"; exit 2 ;;
  esac
done

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$script_dir"

env_base="environments/${env_name}.postman_environment.json"
env_local="environments/${env_name}.local.json"
env_merged="environments/${env_name}.merged.json"
[[ -f "$env_base" ]] || { echo "missing $env_base"; exit 1; }

if [[ -f "$env_local" ]]; then
  jq -s '.[0] as $b | .[1] as $l | $b * { values: ((($b.values + $l.values) | group_by(.key) | map(.[-1]))) }' "$env_base" "$env_local" > "$env_merged"
else
  cp "$env_base" "$env_merged"
fi

if [[ -n "$module" ]]; then collections=("collections/${module}.postman_collection.json"); else collections=(collections/*.postman_collection.json); fi
[[ "$allow_flaky" == "1" && -f known-flaky.md ]] && echo "(allow-flaky enabled)"

mkdir -p reports
exit_code=0
for c in "${collections[@]}"; do
  name=$(basename "$c" .postman_collection.json)
  args=(run "$c" -e "$env_merged" --reporters cli,htmlextra,junit,json
        --reporter-htmlextra-export "reports/${name}.html"
        --reporter-junit-export "reports/${name}.junit.xml"
        --reporter-json-export "reports/${name}.json")
  [[ -n "$folder" ]] && args+=(--folder "$folder")
  echo ">>> newman ${args[*]}"
  newman "${args[@]}" || exit_code=$?
done

[[ -f scripts/extract-responses.js ]] && { echo ">>> node scripts/extract-responses.js"; node scripts/extract-responses.js || true; }
rm -f "$env_merged"
exit $exit_code
