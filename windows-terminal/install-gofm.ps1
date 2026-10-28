# Installs gofm for the current user and adds a "gofm" profile to Windows Terminal
# (Cascadia Mono font, dark-blue colour scheme, underscore cursor).
#
#   powershell -ExecutionPolicy Bypass -File install-gofm.ps1 [-AddToPath]
#
# Nothing needs administrator rights. To uninstall, delete the two folders printed below
# (and remove the folder from your user PATH if you used -AddToPath).

param([switch]$AddToPath)
$ErrorActionPreference = 'Stop'

$here = Split-Path -Parent $MyInvocation.MyCommand.Path
$exe  = if ($env:PROCESSOR_ARCHITECTURE -eq 'ARM64') { 'gofm-arm64.exe' } else { 'gofm.exe' }
$src  = Join-Path $here $exe
if (-not (Test-Path $src)) { $src = Join-Path (Split-Path -Parent $here) $exe } # zip layout: exe one level up
if (-not (Test-Path $src)) { throw "Cannot find $exe next to this script." }

# 1. The program
$dest = Join-Path $env:LOCALAPPDATA 'Programs\gofm'
New-Item -ItemType Directory -Force -Path $dest | Out-Null
Copy-Item $src (Join-Path $dest 'gofm.exe') -Force

# 2. The Windows Terminal profile (a "fragment": Windows Terminal reads it at start-up)
$frag = Join-Path $env:LOCALAPPDATA 'Microsoft\Windows Terminal\Fragments\gofm'
New-Item -ItemType Directory -Force -Path $frag | Out-Null
Copy-Item (Join-Path $here 'gofm-terminal-profile.json') (Join-Path $frag 'gofm.json') -Force

# 3. Optional: run "gofm" from any command prompt
if ($AddToPath) {
    $userPath = [Environment]::GetEnvironmentVariable('Path', 'User')
    if (($userPath -split ';') -notcontains $dest) {
        [Environment]::SetEnvironmentVariable('Path', ($userPath.TrimEnd(';') + ';' + $dest), 'User')
        Write-Host "Added $dest to your user PATH (open a new window to use it)."
    }
}

Write-Host "gofm installed in:        $dest"
Write-Host "Terminal profile in:      $frag"
Write-Host "Restart Windows Terminal and choose the 'gofm' profile from the drop-down menu."
