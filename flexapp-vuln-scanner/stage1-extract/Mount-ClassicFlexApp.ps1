#Requires -Version 7.0

<#
.SYNOPSIS
    Mounts a classic FlexApp VHDX read-only and returns its volume root path.
.DESCRIPTION
    Never mounts read-write. Callers are responsible for calling
    Dismount-ClassicFlexApp in a finally block - this module only wraps the
    mount half so PLAN.md's "always dismount in a finally block, including
    on Ctrl-C" rule stays visible at the call site rather than hidden here.
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

    # Give the OS a moment to surface the volume/drive letter after mount.
    $volume = $null
    for ($attempt = 0; $attempt -lt 10 -and -not $volume; $attempt++) {
        $volume = $diskImage |
            Get-Disk -ErrorAction SilentlyContinue |
            Get-Partition -ErrorAction SilentlyContinue |
            Get-Volume -ErrorAction SilentlyContinue |
            Where-Object { $_.DriveLetter } |
            Select-Object -First 1
        if (-not $volume) { Start-Sleep -Milliseconds 250 }
    }

    if (-not $volume) {
        Dismount-DiskImage -ImagePath $VhdxPath -ErrorAction SilentlyContinue | Out-Null
        throw "Mounted '$VhdxPath' but no drive-lettered volume appeared - unsupported layout or the VHDX has no readable partition."
    }

    [PSCustomObject]@{
        ImagePath   = $VhdxPath
        DriveLetter = $volume.DriveLetter
        RootPath    = "$($volume.DriveLetter):\"
    }
}

function Dismount-ClassicFlexApp {
    [CmdletBinding()]
    param(
        [Parameter(Mandatory)]
        [string]$VhdxPath
    )

    Dismount-DiskImage -ImagePath $VhdxPath -ErrorAction SilentlyContinue | Out-Null
}
