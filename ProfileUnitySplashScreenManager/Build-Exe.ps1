<#
.SYNOPSIS
    Builds ProfileUnitySplashScreenLogoManager.exe from the PowerShell script using PS2EXE.
.DESCRIPTION
    Run this on a Windows machine with PowerShell (5.1 or 7+; Windows PowerShell
    5.1 is the safest bet since ps2exe/WPF/WinForms are all designed around it).
    Installs the PS2EXE module if it isn't already present, then compiles
    Set-ProfileUnitySplashLogo.ps1 into a standalone .exe with:
      - the Liquidware app icon (app-icon.ico)
      - no visible console window (it's a GUI app)
      - an embedded manifest requesting admin elevation (UAC prompt on launch,
        since the tool needs to write to Program Files)
      - STA threading (required for WPF)
.NOTES
    Run this from the same folder as Set-ProfileUnitySplashLogo.ps1 and
    app-icon.ico, or pass explicit -ScriptPath / -IconPath / -OutputPath.
#>

param(
    [string]$ScriptPath = (Join-Path $PSScriptRoot 'Set-ProfileUnitySplashScreenLogo.ps1'),
    [string]$IconPath   = (Join-Path $PSScriptRoot 'app-icon.ico'),
    [string]$OutputPath = (Join-Path $PSScriptRoot 'ProfileUnitySplashScreenLogoManager.exe')
)

if (-not (Test-Path $ScriptPath)) { throw "Script not found: $ScriptPath" }
if (-not (Test-Path $IconPath))   { throw "Icon not found: $IconPath" }

if (-not (Get-Module -ListAvailable -Name ps2exe)) {
    Write-Host "PS2EXE module not found -- installing from PSGallery (current user scope)..."
    Install-Module -Name ps2exe -Scope CurrentUser -Force -AllowClobber
}
Import-Module ps2exe

Write-Host "Compiling $ScriptPath -> $OutputPath ..."

ps2exe -inputFile $ScriptPath `
       -outputFile $OutputPath `
       -iconFile $IconPath `
       -title 'ProfileUnity SplashScreen Logo Manager' `
       -company 'Liquidware' `
       -product 'ProfileUnity SplashScreen Logo Manager' `
       -version '1.0.0.0' `
       -copyright 'Liquidware' `
       -noConsole `
       -requireAdmin `
       -STA

if (Test-Path $OutputPath) {
    Write-Host "Done: $OutputPath"
} else {
    Write-Warning "ps2exe did not report success -- check the output above for errors."
}
