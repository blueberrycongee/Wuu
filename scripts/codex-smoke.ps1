param(
  [string]$Distro = "Ubuntu"
)

$ErrorActionPreference = "Stop"

Set-Location $PSScriptRoot\..

function Resolve-CodexPath {
  $local = Join-Path (Get-Location) "codex.exe"
  if (Test-Path $local) { return $local }

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
Write-Host "codex path: $codexPath"
Write-Host "codex version:"
& $codexPath --version

$cfg = Join-Path $env:USERPROFILE ".codex\\config.toml"
if (Test-Path $cfg) {
  Write-Host "codex config: $cfg"
  $cfgText = Get-Content $cfg -Raw
  $model = ([regex]::Match($cfgText, '^\s*model\s*=\s*\"([^\"]+)\"', 'Multiline')).Groups[1].Value
  $effort = ([regex]::Match($cfgText, '^\s*model_reasoning_effort\s*=\s*\"([^\"]+)\"', 'Multiline')).Groups[1].Value
  if ($model) { Write-Host "config model: $model" } else { Write-Host "config model: (not found)" }
  if ($effort) { Write-Host "config reasoning effort: $effort" } else { Write-Host "config reasoning effort: (not found)" }
} else {
  Write-Host "codex config: (missing) $cfg"
  $model = ""
  $effort = ""
}

Write-Host "gh auth status:"
& gh auth status

Write-Host "WSL check:"
& wsl -l -q

Write-Host "WSL rust cache sanity:"
& wsl -d $Distro -- bash -lc "test -f /mnt/d/wuu-cache/cargo/env && echo 'ok: /mnt/d/wuu-cache/cargo/env exists' || echo 'missing: /mnt/d/wuu-cache/cargo/env'"

$prompt = @"
This is a smoke test. Reply with exactly: OK
"@

New-Item -ItemType Directory -Force -Path logs | Out-Null
$ts = Get-Date -Format "yyyyMMdd-HHmmss"
$lastMsgFile = Join-Path "logs" ("codex-smoke-last-" + $ts + ".txt")

Write-Host "codex exec smoke run (non-interactive)..."
if ($model) {
  # Explicitly request the configured model so we verify the CLI accepts it.
  $null = $prompt | & $codexPath -a never -s workspace-write exec -C (Get-Location) --model $model --output-last-message $lastMsgFile
} else {
  $null = $prompt | & $codexPath -a never -s workspace-write exec -C (Get-Location) --output-last-message $lastMsgFile
}

Write-Host "codex last message file: $lastMsgFile"
if (Test-Path $lastMsgFile) {
  Write-Host (Get-Content $lastMsgFile -Raw)
} else {
  Write-Host "(missing last message file)"
}

Write-Host "ok: codex can run non-interactively"
