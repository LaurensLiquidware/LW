#Requires -Version 7.0

<#
.SYNOPSIS
    Unwraps a FlexApp One self-extracting package into its VHDX + XML.
.DESCRIPTION
    Per PLAN.md's confirmed CLI reference and safety rule: the package
    executable is invoked with EXACTLY "--extract <path> --skipico" and
    nothing else. This function does not accept, forward, or construct any
    other argument. The same executable also installs/uninstalls the
    FlexApp service, mounts/launches the app, replaces or deletes packages,
    etc. - none of that is reachable from this function by design.

    Exit codes for this tool are undocumented, so success is judged by
    whether the expected .vhdx/.xml actually appear in the output folder,
    not by the process exit code (which is still captured for diagnostics).
#>

function Expand-FlexAppOnePackage {
    [CmdletBinding()]
    param(
        [Parameter(Mandatory)]
        [string]$PackageExePath,

        [Parameter(Mandatory)]
        [string]$OutputDir
    )

    if (-not (Test-Path -LiteralPath $PackageExePath)) {
        throw "FlexApp One package not found: $PackageExePath"
    }

    # --extract requires the target folder to already exist.
    if (-not (Test-Path -LiteralPath $OutputDir)) {
        New-Item -ItemType Directory -Path $OutputDir -Force | Out-Null
    }

    $stdoutPath = Join-Path $OutputDir '_extract.stdout.log'
    $stderrPath = Join-Path $OutputDir '_extract.stderr.log'

    # Hardcoded argument list - see safety rule in PLAN.md §"Assumptions", point 3.
    $arguments = @('--extract', $OutputDir, '--skipico')

    $process = Start-Process -FilePath $PackageExePath `
        -ArgumentList $arguments `
        -NoNewWindow `
        -Wait `
        -PassThru `
        -RedirectStandardOutput $stdoutPath `
        -RedirectStandardError $stderrPath `
        -ErrorAction Stop

    $vhdxPath = Get-ChildItem -LiteralPath $OutputDir -Filter '*.vhdx' -File -ErrorAction SilentlyContinue | Select-Object -First 1
    $xmlPath  = Get-ChildItem -LiteralPath $OutputDir -Filter '*.xml'  -File -ErrorAction SilentlyContinue | Select-Object -First 1

    if (-not $vhdxPath) {
        # -Encoding UTF8, matching this project's explicit-encoding convention
        # elsewhere - Sparks Tool Project Review Checklist §1. Best-effort: the
        # actual encoding flexappone.exe writes to these redirected
        # stdout/stderr files is undocumented (see PLAN.md's "flexappone.exe
        # assumptions" note), so this is only exercised on the failure path
        # (an already-thrown, human-read diagnostic message), not a success-
        # path artifact whose correctness the rest of the pipeline depends on.
        $stderrText = if (Test-Path -LiteralPath $stderrPath) { Get-Content -LiteralPath $stderrPath -Raw -Encoding UTF8 } else { '' }
        $stdoutText = if (Test-Path -LiteralPath $stdoutPath) { Get-Content -LiteralPath $stdoutPath -Raw -Encoding UTF8 } else { '' }
        throw "Extraction of '$PackageExePath' did not produce a .vhdx in '$OutputDir' (exit code $($process.ExitCode)). stdout: $stdoutText stderr: $stderrText"
    }

    [PSCustomObject]@{
        VhdxPath = $vhdxPath.FullName
        XmlPath  = if ($xmlPath) { $xmlPath.FullName } else { $null }
        ExitCode = $process.ExitCode
    }
}
