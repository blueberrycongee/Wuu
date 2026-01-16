#!/usr/bin/env bash
set -euo pipefail

MAX_ITERS="${MAX_ITERS:-200}"

cd "$(dirname "$0")/.."
mkdir -p logs

for ((i=1; i<=MAX_ITERS; i++)); do
  if [[ -f STOP ]]; then
    echo "STOP file found. Exiting."
    exit 0
  fi

  ts="$(date +%Y%m%d-%H%M%S)"
  log="logs/codex-${ts}.log"
  echo "Iteration ${i}/${MAX_ITERS} -> ${log}"

  # Feed the prompt to codex and capture output.
  set +e
  cat prompt.md | codex >"${log}" 2>&1
  set -e

  if [[ -f STOP ]]; then
    echo "STOP file found after run. Exiting."
    exit 0
  fi

  if [[ -z "$(git status --porcelain)" ]]; then
    echo "No changes detected. Exiting."
    exit 0
  fi
done

echo "Max iterations reached. Exiting."
exit 0

