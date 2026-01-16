param(
  [string]$Model = "gpt-5.2-codex"
)

$ErrorActionPreference = "Stop"
Set-Location $PSScriptRoot\..

$codex = Join-Path (Get-Location) "codex.exe"
if (!(Test-Path $codex)) {
  $cmd = Get-Command codex.exe -ErrorAction SilentlyContinue
  if ($cmd) { $codex = $cmd.Source } else { throw "codex.exe not found" }
}

New-Item -ItemType Directory -Force -Path logs | Out-Null
$ts = Get-Date -Format "yyyyMMdd-HHmmss"
$log = Join-Path "logs" ("codex-header-" + $ts + ".log")

$workdir = (Get-Location).Path

$cmdLine = "echo Say OK | """ + $codex + """ --dangerously-bypass-approvals-and-sandbox --model " + $Model +
  " exec -C """ + $workdir + """ 1> """ + $log + """ 2>&1"
cmd /c $cmdLine | Out-Null

Get-Content $log -TotalCount 25
