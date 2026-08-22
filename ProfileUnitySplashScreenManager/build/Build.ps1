<#
.SYNOPSIS
    Builds the Windows executable and the distributable zip.
.DESCRIPTION
    All of the logic lives in cmd/build so there is one implementation rather than
    a bash copy and a PowerShell copy drifting apart. This wrapper only checks the
    toolchain is reachable and forwards its arguments.

    Requires Go and, unless -skip-ui is passed, Node.js 22.22.3 or newer.

    Set the PRIMEUI_LICENSE_KEY environment variable to embed the PrimeNG
    commercial license key. Without it the build succeeds but the running
    application shows PrimeNG's "Invalid PrimeUI License" banner. See README.md,
    "PrimeNG licensing".
.EXAMPLE
    .\build\Build.ps1
.EXAMPLE
    $env:PRIMEUI_LICENSE_KEY = '<key>'; .\build\Build.ps1
.EXAMPLE
    .\build\Build.ps1 -skip-ui
#>
[CmdletBinding()]
param(
    [Parameter(ValueFromRemainingArguments = $true)]
    [string[]]$BuildArgs
)

$ErrorActionPreference = 'Stop'
Set-Location (Join-Path $PSScriptRoot '..')

if (-not (Get-Command go -ErrorAction SilentlyContinue)) {
    throw 'go is not on PATH.'
}

& go run ./cmd/build @BuildArgs
if ($LASTEXITCODE -ne 0) {
    throw "Build failed with exit code $LASTEXITCODE."
}
