#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."

export TMPDIR="${TMPDIR:-/mnt/d/Desktop/Wuu/.wuu-cache/tmp}"
export CARGO_HOME="${CARGO_HOME:-/mnt/d/wuu-cache/cargo}"
export RUSTUP_HOME="${RUSTUP_HOME:-/mnt/d/wuu-cache/rustup}"

if [[ -f /mnt/d/wuu-cache/cargo/env ]]; then
  . /mnt/d/wuu-cache/cargo/env
fi

export PATH="/mnt/d/wuu-cache/cargo/bin:$PATH"
export WUU_UPDATE_GOLDENS=1
export WUU_SLOW_TESTS="${WUU_SLOW_TESTS:-1}"

cargo test --test stage2_bootstrap_tests
