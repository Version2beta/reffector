
    
  
#!/usr/bin/env bash
# Run from ~/Development/reffector/experiments (or pass that directory as $1).
# Prints the inputs needed for the paired agent prompts. Does not change repos.
set -euo pipefail

experiments_dir="${1:-.}"
experiments_dir="$(cd "$experiments_dir" && pwd -P)"

printf 'Experiment directory: %s\n' "$experiments_dir"
printf 'Go toolchain: %s\n' "$(go version 2>/dev/null || printf 'UNAVAILABLE')"
if command -v reffector >/dev/null 2>&1; then
  printf 'Reffector binary: %s\n' "$(command -v reffector)"
else
  printf 'Reffector binary: UNAVAILABLE (required for treatment runs)\n'
fi

report_repo() {
  local name="$1" expected_tag="$2" repo revision tag status module scope
  shift 2
  repo="$experiments_dir/$name"

  printf '\n=== %s ===\n' "$name"
  if [[ ! -d "$repo" ]] || [[ "$(git -C "$repo" rev-parse --show-toplevel 2>/dev/null || true)" != "$repo" ]]; then
    printf 'ERROR: expected a Git repository root at ./%s\n' "$name"
    return 1
  fi

  revision="$(git -C "$repo" rev-parse HEAD)"
  tag="$(git -C "$repo" describe --tags --exact-match 2>/dev/null || true)"
  status="CLEAN"
  if [[ -n "$(git -C "$repo" status --porcelain)" ]]; then
    status="DIRTY"
  fi
  module="$(sed -n 's/^module[[:space:]]\{1,\}//p' "$repo/go.mod" | head -n 1)"

  printf '[REPO_PATH] = ./%s\n' "$name"
  printf '[REVISION]  = %s\n' "$revision"
  printf '[SCOPE]     ='
  for scope in "$@"; do printf ' %s' "$scope"; done
  printf '\n'
  printf 'Module: %s\n' "$module"
  printf 'Exact tag: %s (suggested: %s)\n' "${tag:-none}" "$expected_tag"
  printf 'Working tree: %s\n' "$status"
  printf 'Analyze from experiment root: (cd ./%s && reffector -json' "$name"
  for scope in "$@"; do printf ' %s' "$scope"; done
  printf ')\n'

  for scope in "$@"; do
    if [[ ! -d "$repo/$scope" ]]; then
      printf 'WARNING: scope directory not found: %s\n' "$scope"
    fi
  done
  if [[ "$status" != CLEAN ]]; then
    printf 'WARNING: uncommitted/untracked files may invalidate the comparison.\n'
  fi
  if [[ "$tag" != "$expected_tag" ]]; then
    printf 'NOTE: checkout is not at the suggested tag; record this SHA and use it for both runs.\n'
  fi
}

report_repo caddy v2.11.7 ./modules/caddyhttp
report_repo restic v0.19.1 ./internal/repository ./internal/backend/local

printf '\nMetrics (elapsed time, tokens, tool calls, findings) are collected during each agent run, not by this preflight script.\n'


