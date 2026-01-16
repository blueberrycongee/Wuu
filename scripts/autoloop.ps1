param(
  [int]$MaxIters = 200,
  [string]$Distro = "Ubuntu"
)

$ErrorActionPreference = "Stop"

Set-Location $PSScriptRoot\..

New-Item -ItemType Directory -Force -Path logs | Out-Null

function Resolve-CodexPath {
  $cmd = Get-Command codex.exe -ErrorAction SilentlyContinue
  if ($cmd) { return $cmd.Source }

  $cmd = Get-Command codex -ErrorAction SilentlyContinue
  if ($cmd) { return $cmd.Source }

  $windsurf = Join-Path $env:USERPROFILE ".windsurf\\extensions"
  if (Test-Path $windsurf) {
    $candidates = Get-ChildItem -Path $windsurf -Directory -Filter "openai.chatgpt-*" -ErrorAction SilentlyContinue |
      ForEach-Object { Join-Path $_.FullName "bin\\windows-x86_64\\codex.exe" } |
      Where-Object { Test-Path $_ }
    $picked = $candidates | Select-Object -First 1
    if ($picked) { return $picked }
  }

  throw "codex CLI not found. Try: where.exe codex  OR install Codex CLI and ensure it's on PATH."
}

$codexPath = Resolve-CodexPath

for ($i = 1; $i -le $MaxIters; $i++) {
  if (Test-Path .\STOP) {
    Write-Host "STOP file found. Exiting."
    exit 0
  }

  # Enforce single-thread main-branch mode.
  git checkout main | Out-Null
  if ($LASTEXITCODE -ne 0) { throw "failed to checkout main" }
  git pull --rebase origin main 2>$null | Out-Null

  $before = (git rev-parse HEAD)
  $ts = Get-Date -Format "yyyyMMdd-HHmmss"
  $log = Join-Path "logs" ("codex-" + $ts + ".log")

  Write-Host "Iteration $i/$MaxIters -> $log"

  # Feed the prompt to codex and capture output.
  # If codex returns non-zero, we still keep the log and continue unless STOP exists.
  Get-Content .\prompt.md -Raw | & $codexPath 2>&1 | Out-File -FilePath $log -Encoding utf8

  if (Test-Path .\STOP) {
    Write-Host "STOP file found after run. Exiting."
    exit 0
  }

  $after = (git rev-parse HEAD)
  if ($before -eq $after) {
    Write-Host "No new commit detected. Exiting."
    exit 0
  }
}

Write-Host "Max iterations reached. Exiting."
exit 0
