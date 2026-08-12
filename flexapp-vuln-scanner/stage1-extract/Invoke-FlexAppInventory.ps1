#Requires -Version 7.0

<#
.SYNOPSIS
    Stage 1 entry point: extracts/mounts a FlexApp package and writes a
    single JSON inventory file per package (see schemas/inventory.schema.json).
.DESCRIPTION
    Dispatches by extension:
      .vhdx              -> mounted directly (classic FlexApp)
      .exe / .flexapp    -> unwrapped via Expand-FlexAppOnePackage first,
                             then the resulting .vhdx is mounted the same way
    -Path may be a single package file or a directory; a directory is
    enumerated non-recursively for *.vhdx/*.exe/*.flexapp (batch scanning of
    nested folders is out of scope for this pass - see PLAN.md).

    Per PLAN.md's build order, this pass does NOT resolve version identity
    and does NOT apply the exclusion ruleset - see Get-FileInventory.ps1's
    notes. Nothing leaves the machine except the JSON file(s) this writes.

.PARAMETER Path
    A single .vhdx/.exe/.flexapp file, or a directory containing them.
.PARAMETER OutputDir
    Directory to write "<package-basename>.inventory.json" into. Created if
    it doesn't exist.
.PARAMETER TempRoot
    Scratch directory for FlexApp One extraction. Cleaned up per package.
#>

[CmdletBinding()]
param(
    [Parameter(Mandatory)]
    [string]$Path,

    [Parameter(Mandatory)]
    [string]$OutputDir,

    [string]$TempRoot = (Join-Path ([System.IO.Path]::GetTempPath()) 'flexapp-vuln-scanner')
)

$ErrorActionPreference = 'Stop'
$ToolVersion = '0.1.0'

. (Join-Path $PSScriptRoot 'Mount-ClassicFlexApp.ps1')
. (Join-Path $PSScriptRoot 'Expand-FlexAppOne.ps1')
. (Join-Path $PSScriptRoot 'Read-PackageMetadataXml.ps1')
. (Join-Path $PSScriptRoot 'Get-FileInventory.ps1')

if (-not (Test-Path -LiteralPath $OutputDir)) {
    New-Item -ItemType Directory -Path $OutputDir -Force | Out-Null
}

# Tracks currently-mounted VHDX image paths so an interrupt (Ctrl-C or engine
# exit) can still dismount them. Per-package try/finally below is the primary
# mechanism; this is the belt-and-suspenders backstop.
$script:MountedImages = [System.Collections.Generic.List[string]]::new()

$cleanupAction = {
    foreach ($imagePath in $script:MountedImages) {
        try { Dismount-DiskImage -ImagePath $imagePath -ErrorAction SilentlyContinue | Out-Null } catch {}
    }
}
Register-EngineEvent -SourceIdentifier ([System.Management.Automation.PsEngineEvent]::Exiting) -Action $cleanupAction | Out-Null

function Get-TargetPackagePaths {
    param([string]$InputPath)

    if (Test-Path -LiteralPath $InputPath -PathType Container) {
        return Get-ChildItem -LiteralPath $InputPath -File |
            Where-Object { $_.Extension -in '.vhdx', '.exe', '.flexapp' } |
            Select-Object -ExpandProperty FullName
    }

    if (Test-Path -LiteralPath $InputPath -PathType Leaf) {
        return @($InputPath)
    }

    throw "Path not found: $InputPath"
}

function Get-SidecarXmlPath {
    param([string]$VhdxPath)

    $dir = Split-Path -Path $VhdxPath -Parent
    $baseName = [System.IO.Path]::GetFileNameWithoutExtension($VhdxPath)
    $candidate = Join-Path $dir "$baseName.package.xml"
    if (Test-Path -LiteralPath $candidate) { return $candidate }
    return $null
}

function Invoke-SinglePackage {
    param([string]$PackagePath)

    $scanStartedUtc = (Get-Date).ToUniversalTime().ToString('o')
    $extension = [System.IO.Path]::GetExtension($PackagePath).ToLowerInvariant()

    $packageType = switch ($extension) {
        '.vhdx' { 'classic-vhdx' }
        '.exe' { 'flexapp-one' }
        '.flexapp' { 'flexapp-one' }
        default { throw "Unsupported package extension '$extension' for '$PackagePath'." }
    }

    $extractedTempDir = $null
    $vhdxPath = $PackagePath
    $xmlPath = $null

    try {
        if ($packageType -eq 'flexapp-one') {
            $extractedTempDir = Join-Path $TempRoot ([guid]::NewGuid().ToString())
            Write-Verbose "Unwrapping FlexApp One package '$PackagePath' into '$extractedTempDir'"
            $extractResult = Expand-FlexAppOnePackage -PackageExePath $PackagePath -OutputDir $extractedTempDir
            $vhdxPath = $extractResult.VhdxPath
            $xmlPath = $extractResult.XmlPath
        }
        else {
            $xmlPath = Get-SidecarXmlPath -VhdxPath $vhdxPath
        }

        $mountInfo = $null
        try {
            Write-Verbose "Mounting '$vhdxPath' read-only"
            $mountInfo = Mount-ClassicFlexApp -VhdxPath $vhdxPath
            $script:MountedImages.Add($vhdxPath)

            $metadata = if ($xmlPath) { Read-FlexAppPackageXml -XmlPath $xmlPath } else { $null }

            Write-Verbose "Walking '$($mountInfo.RootPath)'"
            $fileRecords = @(Get-FileInventoryRecord -RootPath $mountInfo.RootPath)

            $scanFinishedUtc = (Get-Date).ToUniversalTime().ToString('o')

            $inventory = [ordered]@{
                schemaVersion = '1.0'
                package = [ordered]@{
                    sourcePath      = $PackagePath
                    packageType     = $packageType
                    flexAppXml      = $metadata
                    scanStartedUtc  = $scanStartedUtc
                    scanFinishedUtc = $scanFinishedUtc
                    toolVersion     = $ToolVersion
                }
                files = $fileRecords
            }

            $outBaseName = [System.IO.Path]::GetFileNameWithoutExtension($PackagePath)
            $outPath = Join-Path $OutputDir "$outBaseName.inventory.json"
            $json = $inventory | ConvertTo-Json -Depth 10
            [System.IO.File]::WriteAllText($outPath, $json, [System.Text.UTF8Encoding]::new($false))

            Write-Host "Wrote $outPath ($($fileRecords.Count) files, $(( $fileRecords | Where-Object { $_.readError } ).Count) read errors)"
        }
        finally {
            if ($mountInfo) {
                Dismount-ClassicFlexApp -VhdxPath $vhdxPath
                $script:MountedImages.Remove($vhdxPath) | Out-Null
            }
        }
    }
    finally {
        if ($extractedTempDir -and (Test-Path -LiteralPath $extractedTempDir)) {
            Remove-Item -LiteralPath $extractedTempDir -Recurse -Force -ErrorAction SilentlyContinue
        }
    }
}

try {
    $targets = Get-TargetPackagePaths -InputPath $Path
    if (-not $targets -or $targets.Count -eq 0) {
        Write-Warning "No .vhdx/.exe/.flexapp packages found at '$Path'."
        return
    }

    foreach ($target in $targets) {
        try {
            Invoke-SinglePackage -PackagePath $target
        }
        catch {
            Write-Warning "Failed to process '$target': $_"
        }
    }
}
finally {
    & $cleanupAction
    Unregister-Event -SourceIdentifier ([System.Management.Automation.PsEngineEvent]::Exiting) -ErrorAction SilentlyContinue
}
