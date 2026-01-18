#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."

print_usage() {
  cat <<'USAGE'
Usage: scripts/gen-stage2-artifacts.sh [--fast|--slow]

  --fast   Run stage2 bootstrap tests without slow fixture coverage.
  --slow   Run with slow fixture coverage (default).
USAGE
}

mode="slow"
while [[ $# -gt 0 ]]; do
  case "$1" in
    --fast)
      mode="fast"
      shift
      ;;
    --slow)
      mode="slow"
      shift
      ;;
    -h|--help)
      print_usage
      exit 0
      ;;
    *)
      echo "Unknown argument: $1" >&2
      print_usage >&2
      exit 1
      ;;
  esac
done

export TMPDIR="${TMPDIR:-/mnt/d/Desktop/Wuu/.wuu-cache/tmp}"
export CARGO_HOME="${CARGO_HOME:-/mnt/d/wuu-cache/cargo}"
export RUSTUP_HOME="${RUSTUP_HOME:-/mnt/d/wuu-cache/rustup}"

if [[ -f /mnt/d/wuu-cache/cargo/env ]]; then
  . /mnt/d/wuu-cache/cargo/env
fi

export PATH="/mnt/d/wuu-cache/cargo/bin:$PATH"
export WUU_UPDATE_GOLDENS=1
if [[ "$mode" == "slow" ]]; then
  export WUU_SLOW_TESTS=1
else
  unset WUU_SLOW_TESTS
fi

cargo test --test stage2_bootstrap_tests
