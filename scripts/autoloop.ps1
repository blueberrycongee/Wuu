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

function Get-CodexModelFromConfig {
  $cfg = Join-Path $env:USERPROFILE ".codex\\config.toml"
  if (!(Test-Path $cfg)) { return $null }
  $cfgText = Get-Content $cfg -Raw
  $m = ([regex]::Match($cfgText, '^\s*model\s*=\s*\"([^\"]+)\"', 'Multiline')).Groups[1].Value
  if ([string]::IsNullOrWhiteSpace($m)) { return $null }
  return $m
}

$codexModel = Get-CodexModelFromConfig

# Autoloop assumes a clean working tree; otherwise `git pull --rebase` is unsafe.
$dirty = git status --porcelain
if (-not [string]::IsNullOrWhiteSpace($dirty)) {
  Write-Host "Working tree is not clean. Please commit/stash changes before starting autoloop."
  exit 1
}

for ($i = 1; $i -le $MaxIters; $i++) {
  if (Test-Path .\STOP) {
    Write-Host "STOP file found. Exiting."
    exit 0
  }

  # Enforce single-thread main-branch mode.
  git checkout main | Out-Null
  if ($LASTEXITCODE -ne 0) { throw "failed to checkout main" }
  # Git frequently writes progress/info to stderr; merge streams so PowerShell doesn't treat it as an error.
  $null = git pull --rebase origin main 2>&1
  if ($LASTEXITCODE -ne 0) { throw "git pull --rebase failed" }

  $before = (git rev-parse HEAD)
  $ts = Get-Date -Format "yyyyMMdd-HHmmss"
  $log = Join-Path "logs" ("codex-" + $ts + ".log")

  Write-Host "Iteration $i/$MaxIters -> $log"

  # Feed the prompt to codex and capture output.
  # If codex returns non-zero, we still keep the log and continue unless STOP exists.
  $prompt = Get-Content .\prompt.md -Raw
  $args = @("--dangerously-bypass-approvals-and-sandbox", "exec", "-C", (Get-Location))
  if ($codexModel) { $args += @("--model", $codexModel) }
  $prompt | & $codexPath @args 2>&1 | Out-File -FilePath $log -Encoding utf8

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
