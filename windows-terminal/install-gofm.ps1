# Installs gofm for the current user and adds a "gofm" profile to Windows Terminal
# (Cascadia Mono NF font, dark-blue colour scheme, underscore cursor).
# The font itself is not installed: see "Font with icons" in README.md.
#
#   powershell -ExecutionPolicy Bypass -File install-gofm.ps1 [-AddToPath]
#
# Nothing needs administrator rights. To uninstall, delete the two folders printed below
# (and remove the folder from your user PATH if you used -AddToPath).
# gofm is portable: its settings (gofm.ini) and memory (state.json) live next to
# gofm.exe in the program folder. Installing again replaces only gofm.exe and keeps them.

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

# 4. The icon font is installed by hand: only check that it is there.
$fontDirs = @((Join-Path $env:WINDIR 'Fonts'), (Join-Path $env:LOCALAPPDATA 'Microsoft\Windows\Fonts'))
$hasFont = $false
foreach ($d in $fontDirs) {
    if (Test-Path $d) {
        if (Get-ChildItem -Path $d -Filter 'CascadiaMonoNF*' -ErrorAction SilentlyContinue) { $hasFont = $true }
    }
}
if (-not $hasFont) {
    Write-Host "NOTE: the font 'Cascadia Mono NF' is not installed. gofm's icons need it:" -ForegroundColor Yellow
    Write-Host "      https://github.com/microsoft/cascadia-code/releases -> zip -> ttf\CascadiaMonoNF.ttf -> Install." -ForegroundColor Yellow
    Write-Host "      Without it, press Alt+I in gofm to use basic symbols." -ForegroundColor Yellow
}

Write-Host "gofm installed in:        $dest"
Write-Host "Settings (created at the first start): $(Join-Path $dest 'gofm.ini')"
Write-Host "Terminal profile in:      $frag"
Write-Host "Restart Windows Terminal and choose the 'gofm' profile from the drop-down menu."
