param()

$ErrorActionPreference = "Stop"

Set-Location $PSScriptRoot\..

$cmd = @"
mkdir -p /mnt/d/wuu-cache/cargo /mnt/d/wuu-cache/rustup
if [ ! -x /mnt/d/wuu-cache/cargo/bin/rustup ]; then
  env CARGO_HOME=/mnt/d/wuu-cache/cargo RUSTUP_HOME=/mnt/d/wuu-cache/rustup sh -c 'curl -sSf https://sh.rustup.rs | sh -s -- -y --no-modify-path --profile minimal --default-toolchain stable'
fi
. /mnt/d/wuu-cache/cargo/env
env RUSTUP_HOME=/mnt/d/wuu-cache/rustup rustup default stable >/dev/null
env RUSTUP_HOME=/mnt/d/wuu-cache/rustup rustup component add clippy rustfmt >/dev/null
echo ok
"@

wsl -d Ubuntu -- bash -lc $cmd

