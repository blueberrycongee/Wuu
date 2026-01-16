#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."

export RUSTUP_HOME="${RUSTUP_HOME:-/mnt/d/wuu-cache/rustup}"

if [[ -f /mnt/d/wuu-cache/cargo/env ]]; then
  # Prefer the D: drive rustup/cargo install if present.
  . /mnt/d/wuu-cache/cargo/env
fi

cargo fmt --all
cargo clippy --all-targets -- -D warnings
cargo test
