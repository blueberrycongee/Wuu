#!/usr/bin/env bash
set -euo pipefail

# Installs rustup into the D: drive cache so WSL build artifacts/toolchains
# live on /mnt/d instead of the distro filesystem.

mkdir -p /mnt/d/wuu-cache/cargo /mnt/d/wuu-cache/rustup

if [[ ! -x /mnt/d/wuu-cache/cargo/bin/rustup ]]; then
  env CARGO_HOME=/mnt/d/wuu-cache/cargo RUSTUP_HOME=/mnt/d/wuu-cache/rustup \
    sh -c 'curl -sSf https://sh.rustup.rs | sh -s -- -y --no-modify-path --profile minimal --default-toolchain stable'
fi

. /mnt/d/wuu-cache/cargo/env
env RUSTUP_HOME=/mnt/d/wuu-cache/rustup rustup default stable >/dev/null
env RUSTUP_HOME=/mnt/d/wuu-cache/rustup rustup component add clippy rustfmt >/dev/null

echo "ok: rustup/cargo configured on D:"
echo "  CARGO_HOME=/mnt/d/wuu-cache/cargo"
echo "  RUSTUP_HOME=/mnt/d/wuu-cache/rustup"
command -v cargo
cargo --version

