param(
  [int]$MaxIters = 200,
  [string]$Distro = "Ubuntu",
  [int]$MaxNoProgress = 5
)

$ErrorActionPreference = "Stop"

Set-Location $PSScriptRoot\..

New-Item -ItemType Directory -Force -Path logs | Out-Null

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

function Invoke-GitSafe {
  param(
    [Parameter(Mandatory = $true)][string]$Args
  )

  # Windows PowerShell 5.1 treats native stderr output as error records when
  # $ErrorActionPreference=Stop. Git writes some normal progress messages to stderr.
  # Running through cmd.exe avoids turning stderr into terminating errors.
  $sslBackend = ""
  if ($Args -match "^(pull|push|fetch|clone|ls-remote)\\b") {
    # Some machines hit `OpenSSL SSL_connect: SSL_ERROR_SYSCALL` talking to github.com over HTTPS.
    # Git-for-Windows supports switching to the Windows TLS stack via Schannel.
    #
    # Note: Schannel revocation checks can also break GitHub HTTPS on some networks; disabling
    # revocation checks here makes the autoloop significantly more stable in practice.
    $sslBackend = "-c http.sslBackend=schannel -c http.schannelCheckRevoke=false "
  }

  $cmd = "git " + $sslBackend + $Args + " 2>&1"
  $output = cmd /c $cmd
  if ($LASTEXITCODE -ne 0) {
    $text = ""
    if ($output) { $text = "`n" + ($output -join "`n") }
    throw ("git " + $Args + " failed with exit code " + $LASTEXITCODE + $text)
  }
}

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

$noProgress = 0

for ($i = 1; $i -le $MaxIters; $i++) {
  if (Test-Path .\STOP) {
    Write-Host "STOP file found. Exiting."
    exit 0
  }

  # Avoid unsafe `git pull --rebase` when an earlier iteration left uncommitted changes.
  $dirty = git status --porcelain
  if (-not [string]::IsNullOrWhiteSpace($dirty)) {
    Write-Host "Working tree is not clean; refusing to run autoloop."
    Write-Host "Run: git status --porcelain"
    Write-Host $dirty
    Write-Host "Fix by committing/stashing changes, or discard with: git restore -SW ."
    exit 1
  }

  # Enforce single-thread main-branch mode.
  Invoke-GitSafe "checkout main"
  Invoke-GitSafe "pull --rebase origin main"

  $before = (git rev-parse HEAD)
  $ts = Get-Date -Format "yyyyMMdd-HHmmss"
  $log = Join-Path "logs" ("codex-" + $ts + ".log")

  Write-Host "Iteration $i/$MaxIters -> $log"

  # Feed the prompt to codex and capture output.
  # If codex returns non-zero, we still keep the log and continue unless STOP exists.
  # Run via cmd.exe to avoid Windows PowerShell turning native stderr output into terminating errors.
  $workdir = (Get-Location).Path
  $codexQuoted = '"' + $codexPath + '"'
  $logQuoted = '"' + $log + '"'
  $workdirQuoted = '"' + $workdir + '"'
  $modelArg = ""
  if ($codexModel) {
    $modelArg = " --model " + $codexModel
  }

  $promptFile = "prompt.md"
  $cmdCore = "exec"
  if (Test-Path "logs\\_last_session_marker") {
    $promptFile = "prompt_followup.md"
    # Resume requires placing -C on `exec`, not on `resume`.
    # Use `-` so the follow-up prompt is read from stdin.
    $cmdCore = "exec -C " + $workdirQuoted + " resume --last -"
  } else {
    $cmdCore = "exec -C " + $workdirQuoted
  }

  $cmdLine = "type " + $promptFile + " | " + $codexQuoted +
    " --dangerously-bypass-approvals-and-sandbox" + $modelArg + " " + $cmdCore +
    " 1> " + $logQuoted + " 2>&1"

  cmd /c $cmdLine | Out-Null
  $codexExit = $LASTEXITCODE
  if ($codexExit -eq 0) {
    Set-Content -Path "logs\\_last_session_marker" -Value "ok" -Encoding ascii
  } else {
    Remove-Item -Force "logs\\_last_session_marker" -ErrorAction SilentlyContinue
  }

  if (Test-Path .\STOP) {
    Write-Host "STOP file found after run. Exiting."
    exit 0
  }

  $after = (git rev-parse HEAD)
  if ($before -eq $after) {
    $noProgress += 1
    Write-Host "No new commit detected ($noProgress/$MaxNoProgress)."
    if ($noProgress -ge $MaxNoProgress) {
      Write-Host "No progress threshold reached. Exiting."
      exit 0
    }
  } else {
    $noProgress = 0
  }
}

Write-Host "Max iterations reached. Exiting."
exit 0
