#Requires -Version 7.0

<#
.SYNOPSIS
    Mounts a classic FlexApp VHDX read-only and returns its volume root path.
.DESCRIPTION
    Never mounts read-write. Callers are responsible for calling
    Dismount-ClassicFlexApp in a finally block - this module only wraps the
    mount half so PLAN.md's "always dismount in a finally block, including
    on Ctrl-C" rule stays visible at the call site rather than hidden here.

    Mounts to a scratch folder via Add-PartitionAccessPath rather than
    waiting for/relying on an automatic drive letter. Confirmed on a real
    Windows 11 host (2026-08-13): the disk comes online immediately and the
    partition/volume is healthy with the correct filesystem and label, but
    Windows does NOT reliably assign it a drive letter - waiting longer
    never helps, since no assignment is coming. Add-PartitionAccessPath
    also sidesteps drive-letter exhaustion on a host scanning many packages.
#>

function Mount-ClassicFlexApp {
    [CmdletBinding()]
    param(
        [Parameter(Mandatory)]
        [string]$VhdxPath
    )

    if (-not (Test-Path -LiteralPath $VhdxPath)) {
        throw "VHDX not found: $VhdxPath"
    }

    $diskImage = Mount-DiskImage -ImagePath $VhdxPath -Access ReadOnly -PassThru -ErrorAction Stop

    $partitions = $null
    for ($attempt = 0; $attempt -lt 10 -and -not $partitions; $attempt++) {
        $partitions = @($diskImage |
            Get-Disk -ErrorAction SilentlyContinue |
            Get-Partition -ErrorAction SilentlyContinue)
        if (-not $partitions) { Start-Sleep -Milliseconds 250 }
    }

    if (-not $partitions) {
        Dismount-DiskImage -ImagePath $VhdxPath -ErrorAction SilentlyContinue | Out-Null
        throw "Mounted '$VhdxPath' but no partitions appeared - unsupported layout or the VHDX has no readable partition."
    }

    # The real data partition is always by far the largest on FlexApp
    # captures - small system/reserved partitions (e.g. a ~128 MB GPT MSR
    # partition) sort well behind it. Simple, robust heuristic; no GPT type
    # GUID matching needed.
    $dataPartition = $partitions | Sort-Object -Property Size -Descending | Select-Object -First 1

    $mountRoot = Join-Path ([System.IO.Path]::GetTempPath()) "flexapp-vuln-scanner-mount-$([guid]::NewGuid())"
    New-Item -ItemType Directory -Path $mountRoot -Force | Out-Null

    try {
        Add-PartitionAccessPath -DiskNumber $dataPartition.DiskNumber -PartitionNumber $dataPartition.PartitionNumber -AccessPath $mountRoot -ErrorAction Stop
    }
    catch {
        Remove-Item -LiteralPath $mountRoot -Force -Recurse -ErrorAction SilentlyContinue
        Dismount-DiskImage -ImagePath $VhdxPath -ErrorAction SilentlyContinue | Out-Null
        throw "Mounted '$VhdxPath' but failed to assign an access path to disk $($dataPartition.DiskNumber) partition $($dataPartition.PartitionNumber): $_"
    }

    [PSCustomObject]@{
        ImagePath       = $VhdxPath
        DiskNumber      = $dataPartition.DiskNumber
        PartitionNumber = $dataPartition.PartitionNumber
        RootPath        = $mountRoot
    }
}

function Dismount-ClassicFlexApp {
    [CmdletBinding()]
    param(
        [Parameter(Mandatory)]
        [string]$VhdxPath,

        [string]$RootPath,
        [Nullable[int]]$DiskNumber,
        [Nullable[int]]$PartitionNumber
    )

    if ($RootPath -and ($null -ne $DiskNumber) -and ($null -ne $PartitionNumber)) {
        Remove-PartitionAccessPath -DiskNumber $DiskNumber -PartitionNumber $PartitionNumber -AccessPath $RootPath -ErrorAction SilentlyContinue
    }

    Dismount-DiskImage -ImagePath $VhdxPath -ErrorAction SilentlyContinue | Out-Null

    if ($RootPath -and (Test-Path -LiteralPath $RootPath)) {
        Remove-Item -LiteralPath $RootPath -Force -Recurse -ErrorAction SilentlyContinue
    }
}
