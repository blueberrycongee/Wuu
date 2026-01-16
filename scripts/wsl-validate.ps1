param()

$ErrorActionPreference = "Stop"

Set-Location $PSScriptRoot\..

$cmd = "cd /mnt/d/Desktop/Wuu && " +
  ". /mnt/d/wuu-cache/cargo/env && " +
  "RUSTUP_HOME=/mnt/d/wuu-cache/rustup " +
  "cargo fmt --all && cargo clippy --all-targets -- -D warnings && cargo test"

wsl -d Ubuntu -- bash -lc $cmd
