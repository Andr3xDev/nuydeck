#!/usr/bin/env bash
set -euo pipefail

mode="status"
roots=()

usage() {
  cat <<'EOF'
Usage: codegraph-health.sh [--sync|--init|--index] [repo...]

Default: read-only status for indexed and unindexed Git repos.
--sync:  run codegraph sync for repos that already have .codegraph/
--init:  run codegraph init for repos without .codegraph/
--index: run codegraph index for repos that already have .codegraph/
EOF
}

while [ "$#" -gt 0 ]; do
  case "$1" in
    --sync|--init|--index) mode="${1#--}" ;;
    -h|--help) usage; exit 0 ;;
    *) roots+=("$1") ;;
  esac
  shift
done

if ! command -v codegraph >/dev/null 2>&1; then
  echo "codegraph missing"
  exit 1
fi

if [ "${#roots[@]}" -eq 0 ]; then
  for root in "$HOME/dev" "$HOME/laburo"; do
    [ -d "$root" ] && roots+=("$root")
  done
fi

repos=()
for root in "${roots[@]}"; do
  if [ -d "$root/.git" ] || [ -d "$root/.codegraph" ]; then
    repos+=("$root")
  elif [ -d "$root" ]; then
    while IFS= read -r gitdir; do
      repos+=("${gitdir%/.git}")
    done < <(find "$root" -mindepth 2 -maxdepth 4 -type d -name .git 2>/dev/null | sort)
  fi
done

if [ "${#repos[@]}" -eq 0 ]; then
  echo "no repos found"
  exit 0
fi

for repo in "${repos[@]}"; do
  printf '\n== %s ==\n' "$repo"
  if [ -d "$repo/.codegraph" ]; then
    case "$mode" in
      sync) codegraph sync "$repo" ;;
      index) codegraph index "$repo" ;;
      init) codegraph status "$repo" ;;
      status) codegraph status "$repo" ;;
    esac
  else
    if [ "$mode" = "init" ]; then
      codegraph init "$repo"
    else
      echo "not initialized"
      echo "next: codegraph init '$repo'"
    fi
  fi
done
