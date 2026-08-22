<#
.SYNOPSIS
    Builds ProfileUnitySplashScreenManager.exe from the PowerShell script using
    PS2EXE, finalises the SBOM, and packages the distributable zip.
.DESCRIPTION
    Run this on a Windows machine with PowerShell (5.1 or 7+; Windows PowerShell
    5.1 is the safest bet since ps2exe/WPF/WinForms are all designed around it).

    This script is the authority for three things the Sparks Tool Project Review
    Checklist requires to agree with each other:

      * Version (section 6) -- read from $AppVersion in
        Set-ProfileUnitySplashScreenLogo.ps1, never hardcoded here, and stamped
        into the executable's file metadata, the SBOM and the zip filename.
      * SBOM (section 4) -- bom.cdx.json is rewritten with the ps2exe version
        actually resolved at build time, so the shipped SBOM describes the
        artifact rather than a guess.
      * Packaging (section 7) -- the zip carries Spark_License.pdf and
        bom.cdx.json side by side at its top level.

    ps2exe is never installed floating. Either pass -Ps2ExeVersion to pin it, or
    have it already installed and this script records whichever version that is.
.PARAMETER Ps2ExeVersion
    The exact ps2exe version to install and use, e.g. '1.0.15'. Omit only if the
    module is already installed -- the resolved version is then recorded in the
    SBOM either way.
.PARAMETER SkipPackage
    Compile the executable but do not build the distributable zip.
.NOTES
    Run from the folder holding Set-ProfileUnitySplashScreenLogo.ps1,
    app-icon.ico, Spark_License.pdf and bom.cdx.json, or pass explicit paths.

    After this completes, re-run the Grype scan against the regenerated
    bom.cdx.json (checklist section 5) -- the SBOM changed, so the previous
    scan no longer describes what ships.
#>

param(
    [string]$ScriptPath   = (Join-Path $PSScriptRoot 'Set-ProfileUnitySplashScreenLogo.ps1'),
    [string]$IconPath     = (Join-Path $PSScriptRoot 'app-icon.ico'),
    [string]$OutputDir    = $PSScriptRoot,
    [string]$SbomPath     = (Join-Path $PSScriptRoot 'bom.cdx.json'),
    [string]$NoticesPath  = (Join-Path $PSScriptRoot 'THIRD-PARTY-NOTICES.txt'),
    [string]$Ps2ExeVersion,
    [switch]$SkipPackage
)

$ErrorActionPreference = 'Stop'
$ToolName = 'ProfileUnitySplashScreenManager'

if (-not (Test-Path -LiteralPath $ScriptPath)) { throw "Script not found: $ScriptPath" }
if (-not (Test-Path -LiteralPath $IconPath))   { throw "Icon not found: $IconPath" }

# ---------------------------------------------------------------------------
# 1. Version -- single source of truth, read from the script itself
# ---------------------------------------------------------------------------
$scriptText = Get-Content -LiteralPath $ScriptPath -Raw -Encoding UTF8
# Single-quoted so '\$' reaches the regex engine as an escaped dollar. In a
# double-quoted PowerShell string, '`$' collapses to a bare '$', which the regex
# engine reads as an end-of-line anchor and which therefore never matches.
$versionPattern = '(?m)^\s*\$AppVersion\s*=\s*''([0-9]+\.[0-9]+\.[0-9]+)'''
$versionMatch = [regex]::Match($scriptText, $versionPattern)
if (-not $versionMatch.Success) {
    throw "Could not read `$AppVersion from $ScriptPath. Section 6 of the review checklist requires one source of truth for the version -- do not hardcode it here."
}
$AppVersion = $versionMatch.Groups[1].Value
$FileVersion = "$AppVersion.0"
Write-Host "Version (from $([IO.Path]::GetFileName($ScriptPath))): $AppVersion"

$OutputPath = Join-Path $OutputDir "$ToolName.exe"

# ---------------------------------------------------------------------------
# 2. Resolve ps2exe to an exact version -- never floating
# ---------------------------------------------------------------------------
if ($Ps2ExeVersion) {
    $installed = Get-Module -ListAvailable -Name ps2exe |
                 Where-Object { $_.Version.ToString() -eq $Ps2ExeVersion }
    if (-not $installed) {
        Write-Host "Installing ps2exe $Ps2ExeVersion from PSGallery (current user scope)..."
        Install-Module -Name ps2exe -RequiredVersion $Ps2ExeVersion -Scope CurrentUser -Force -AllowClobber
    }
    Import-Module ps2exe -RequiredVersion $Ps2ExeVersion -Force
} else {
    $available = @(Get-Module -ListAvailable -Name ps2exe | Sort-Object Version -Descending)
    if ($available.Count -eq 0) {
        throw @"
ps2exe is not installed, and this script will not install an unpinned version --
the review checklist (section 4) requires dependency versions to be pinned.

Pick a version and pass it explicitly:

    Find-Module ps2exe -AllVersions | Select-Object -First 5 Version, PublishedDate
    .\Build-Exe.ps1 -Ps2ExeVersion '<version>'

The version you pass is recorded in bom.cdx.json as what actually shipped.
"@
    }
    Import-Module ps2exe -RequiredVersion $available[0].Version -Force
}

$resolvedPs2Exe = (Get-Module ps2exe).Version.ToString()
$ps2ExeModuleBase = (Get-Module ps2exe).ModuleBase
Write-Host "ps2exe resolved to: $resolvedPs2Exe  ($ps2ExeModuleBase)"

# ---------------------------------------------------------------------------
# 3. Compile
# ---------------------------------------------------------------------------
Write-Host "Compiling $ScriptPath -> $OutputPath ..."

ps2exe -inputFile $ScriptPath `
       -outputFile $OutputPath `
       -iconFile $IconPath `
       -title 'ProfileUnity SplashScreen Logo Manager' `
       -company 'Liquidware' `
       -product 'ProfileUnity SplashScreen Logo Manager' `
       -version $FileVersion `
       -copyright 'Liquidware' `
       -noConsole `
       -requireAdmin `
       -STA

if (-not (Test-Path -LiteralPath $OutputPath)) {
    throw "ps2exe did not produce $OutputPath -- check the output above for errors."
}
Write-Host "Built: $OutputPath"

# ---------------------------------------------------------------------------
# 4. Finalise the SBOM with what actually shipped (checklist section 4)
# ---------------------------------------------------------------------------
if (Test-Path -LiteralPath $SbomPath) {
    Write-Host "Updating $([IO.Path]::GetFileName($SbomPath)) ..."
    $bom = Get-Content -LiteralPath $SbomPath -Raw -Encoding UTF8 | ConvertFrom-Json

    $bom.metadata.timestamp = (Get-Date).ToUniversalTime().ToString(
        "yyyy-MM-ddTHH:mm:ssZ", [System.Globalization.CultureInfo]::InvariantCulture)
    $bom.metadata.component.version = $AppVersion
    $bom.version = [int]$bom.version + 1

    $exeHash = (Get-FileHash -LiteralPath $OutputPath -Algorithm SHA256).Hash.ToLowerInvariant()

    # The compiled executable embeds a host stub generated by ps2exe, so ps2exe
    # is part of what the customer receives -- not merely a build-time tool.
    $bom.components = @(
        [pscustomobject]@{
            type        = 'application'
            'bom-ref'   = "pkg:nuget/ps2exe@$resolvedPs2Exe"
            name        = 'ps2exe'
            version     = $resolvedPs2Exe
            purl        = "pkg:nuget/ps2exe@$resolvedPs2Exe"
            description = 'Compiles a PowerShell script into a Windows executable that hosts the PowerShell engine. Its generated host stub is embedded in the shipped .exe.'
            scope       = 'required'
            licenses    = @(
                [pscustomobject]@{ license = [pscustomobject]@{ id = 'MIT' } }
            )
            externalReferences = @(
                [pscustomobject]@{ type = 'distribution'; url = 'https://www.powershellgallery.com/packages/ps2exe' }
            )
        }
    )

    $bom.metadata.component | Add-Member -NotePropertyName 'hashes' -NotePropertyValue @(
        [pscustomobject]@{ alg = 'SHA-256'; content = $exeHash }
    ) -Force

    $json = ConvertTo-Json -InputObject $bom -Depth 12
    Set-Content -LiteralPath $SbomPath -Value $json -Encoding UTF8
    Write-Host "  SBOM: version $AppVersion, ps2exe $resolvedPs2Exe, exe SHA-256 $exeHash"
} else {
    Write-Warning "SBOM not found at $SbomPath -- section 4 requires it to ship next to the license PDF."
}

# ---------------------------------------------------------------------------
# 5. Verify the third-party notice against the module's own license file
# ---------------------------------------------------------------------------
$moduleLicense = Get-ChildItem -LiteralPath $ps2ExeModuleBase -File -ErrorAction SilentlyContinue |
                 Where-Object { $_.Name -match '^(LICENSE|LICENCE|COPYING)(\.txt|\.md)?$' } |
                 Select-Object -First 1
if ($moduleLicense) {
    Write-Host "ps2exe ships its own license text: $($moduleLicense.FullName)"
    Write-Host "  Confirm THIRD-PARTY-NOTICES.txt matches it before submitting."
} else {
    Write-Warning "No LICENSE file found in the installed ps2exe module ($ps2ExeModuleBase). Verify the license text in THIRD-PARTY-NOTICES.txt against the module's published source before submitting."
}

# ---------------------------------------------------------------------------
# 6. Package the distributable (checklist section 7)
# ---------------------------------------------------------------------------
if (-not $SkipPackage) {
    $zipPath = Join-Path $OutputDir "$ToolName-$AppVersion.zip"
    $payload = @(
        $OutputPath
        $ScriptPath
        $IconPath
        (Join-Path $PSScriptRoot 'Build-Exe.ps1')
        (Join-Path $PSScriptRoot 'Spark_License.pdf')
        $SbomPath
        $NoticesPath
        (Join-Path $PSScriptRoot 'README.md')
        (Join-Path $PSScriptRoot 'CHANGELOG.md')
    )

    $missing = @($payload | Where-Object { -not (Test-Path -LiteralPath $_) })
    if ($missing.Count -gt 0) {
        throw "Cannot package -- these required files are missing:`n  $($missing -join "`n  ")"
    }

    if (Test-Path -LiteralPath $zipPath) { Remove-Item -LiteralPath $zipPath -Force }
    Compress-Archive -LiteralPath $payload -DestinationPath $zipPath -CompressionLevel Optimal
    Write-Host "Packaged: $zipPath"
    Write-Host ""
    Write-Host "Remaining before submission:"
    Write-Host "  * Re-run Grype against $([IO.Path]::GetFileName($SbomPath)) -- it changed (checklist section 5)."
    Write-Host "  * Confirm the exe launches, elevates, and the About dialog shows version $AppVersion."
}
