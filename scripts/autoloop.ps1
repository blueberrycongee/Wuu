param(
  [int]$MaxIters = 200
)

$ErrorActionPreference = "Stop"

Set-Location $PSScriptRoot\..

New-Item -ItemType Directory -Force -Path logs | Out-Null

for ($i = 1; $i -le $MaxIters; $i++) {
  if (Test-Path .\STOP) {
    Write-Host "STOP file found. Exiting."
    exit 0
  }

  $before = (git rev-parse HEAD)
  $ts = Get-Date -Format "yyyyMMdd-HHmmss"
  $log = Join-Path "logs" ("codex-" + $ts + ".log")

  Write-Host "Iteration $i/$MaxIters -> $log"

  # Feed the prompt to codex and capture output.
  # If codex returns non-zero, we still keep the log and continue unless STOP exists.
  cmd /c "type prompt.md | codex" 1> $log 2>&1

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
