#Requires -Version 7.0

<#
.SYNOPSIS
    Runs a Windows Defender custom scan against a mounted FlexApp
    package's contents and reports whether anything was flagged.
.DESCRIPTION
    Shells out to MpCmdRun.exe (the same CLI Windows Security's own
    "Scan a specific folder" option uses) rather than calling any
    Defender PowerShell cmdlet directly -- MpCmdRun.exe ships with every
    Windows Defender install and needs no extra module, matching how
    Mount-ClassicFlexApp.ps1's own dependencies (Storage cmdlets) are
    always-present built-ins, not optional add-ons.

    MpCmdRun.exe's own exit code is not a reliable "clean vs. infected"
    signal by itself -- Microsoft doesn't document it precisely, and a
    nonzero exit can also mean a scan-execution problem (access denied,
    a locked file) rather than a real detection. The verdict is instead
    built from two sources, either of which is enough to call it
    "threats-found":

      1. Parsing MpCmdRun's own scan-summary text (the "<===LIST OF
         DETECTED THREATS===>" block) for "Threat :" entries and their
         listed resources. This is the authoritative, always-present
         source -- confirmed against real scans where it's the ONLY
         source that reported anything.
      2. Cross-checking Get-MpThreatDetection (part of the built-in
         Defender module) for any detection whose InitialDetectionTime
         is at or after this scan's start time and whose Resources
         reference the scanned path -- kept as a second source because,
         when it does have an entry, it carries a SeverityID the text
         output doesn't. It has proven unreliable on its own: real scans
         against a detection-only (-DisableRemediation) run have shown
         zero matching entries even though MpCmdRun's own text output
         plainly listed the threat -- most likely because
         Get-MpThreatDetection's history is tied to actions Defender
         took, and there's no action to record here.

    Detection-only: the scan runs with -DisableRemediation, so Defender
    reports what it finds but never quarantines/removes/cleans anything
    it flags. This tool's own contract is "detect and report" -- taking
    a destructive default action on a customer's package (which this
    Stage would otherwise keep inventorying right after the scan runs)
    would be a surprising, out-of-scope side effect, and the mounted
    VHDX case makes it worse: Defender modifying a file inside a live
    mount can corrupt the mount, not just the file.

    Never throws: any failure along the way (Defender absent, disabled,
    or its cmdlets unavailable -- e.g. a non-Windows dev machine, or a
    Windows box running a different antivirus product entirely) reports
    status "unavailable"/"error" instead of failing Stage 1 outright.
    This is an additional signal alongside the CVE-matching Stage 2
    already does, not a replacement for it, and not something every
    environment running this tool can be assumed to have.
.PARAMETER Path
    The mounted package's root directory to scan (Mount-ClassicFlexApp's
    RootPath).
#>

function Find-MpCmdRunPath {
    # Checked in the order Microsoft's own docs list: the Platform
    # directory (kept current by Windows Update independently of the OS
    # build) takes precedence over the static Program Files copy. Guards
    # against $env:ProgramData/$env:ProgramFiles being unset (never true
    # on a real Windows install, but this function's whole contract is
    # "never throws" -- Join-Path/Test-Path throw a terminating error on
    # a null Path argument under $ErrorActionPreference = 'Stop', which
    # Invoke-FlexAppInventory.ps1 sets globally, so skipping a null env
    # var here rather than calling Join-Path with it is load-bearing,
    # not defensive-programming theater.
    if ($env:ProgramData) {
        $platformRoot = Join-Path $env:ProgramData 'Microsoft\Windows Defender\Platform'
        if (Test-Path -LiteralPath $platformRoot) {
            $newest = Get-ChildItem -LiteralPath $platformRoot -Directory -ErrorAction SilentlyContinue |
                Sort-Object Name -Descending |
                Select-Object -First 1
            if ($newest) {
                $candidate = Join-Path $newest.FullName 'MpCmdRun.exe'
                if (Test-Path -LiteralPath $candidate) { return $candidate }
            }
        }
    }

    if ($env:ProgramFiles) {
        $fallback = Join-Path $env:ProgramFiles 'Windows Defender\MpCmdRun.exe'
        if (Test-Path -LiteralPath $fallback) { return $fallback }
    }

    return $null
}

function ConvertFrom-MpCmdRunScanOutput {
    # Parses MpCmdRun.exe -Scan's plain-text summary for its
    # "<===LIST OF DETECTED THREATS===>" block, e.g.:
    #
    #   Threat : Virus:DOS/EICAR_Test_File
    #       Resources : 2 total
    #           file:_C:\mount\a.zip->eicar.com
    #           containerfile:_C:\mount\a.zip
    #   --------------------------------------------------------------
    #
    # Each "Threat :" line starts a new entry; subsequent indented lines
    # up to the next "Threat :" line or a dashed separator are its
    # resources (the "Resources : N total" count line is skipped, and
    # each resource's "file:_"/"containerfile:_"-style prefix is
    # stripped so the resource strings this returns match the same
    # convention Get-MpThreatDetection's Resources use). No severity is
    # available from this text -- only Get-MpThreatDetection carries a
    # SeverityID.
    param([string]$Text)

    $threats = @()
    $current = $null
    foreach ($line in ($Text -split "`r?`n")) {
        if ($line -match '^\s*Threat\s*:\s*(.+?)\s*$') {
            if ($current) { $threats += [PSCustomObject]$current }
            $current = [ordered]@{ threatName = $Matches[1]; resources = @() }
            continue
        }
        if (-not $current) { continue }
        if ($line -match '^\s*Resources\s*:\s*\d+\s*total\s*$') { continue }
        if ($line -match '^\s*-{5,}\s*$') { continue }
        $trimmed = $line.Trim()
        # Restricted to MpCmdRun's documented resource-type prefixes --
        # NOT a generic "letters-then-colon" match, which would also hit
        # a bare "C:\..." path's own drive-letter colon and truncate it.
        if ($trimmed -match '^(?:file|containerfile|registry|process|service|driver|resource):_?(.+)$') {
            $current.resources += $Matches[1]
        }
    }
    if ($current) { $threats += [PSCustomObject]$current }
    return $threats
}

function Invoke-DefenderScan {
    [CmdletBinding()]
    param(
        [Parameter(Mandatory)]
        [string]$Path
    )

    $result = [ordered]@{
        tool                  = 'Windows Defender'
        ran                   = $false
        status                = 'unavailable'
        threats               = @()
        pathScanned           = $Path
        scanStartedUtc        = $null
        scanFinishedUtc       = $null
        durationSeconds       = $null
        signatureVersion      = $null
        signatureLastUpdatedUtc = $null
        engineVersion         = $null
        details               = $null
        message               = $null
    }

    $mpCmdRun = Find-MpCmdRunPath
    if (-not $mpCmdRun) {
        $result.message = 'MpCmdRun.exe not found -- Windows Defender does not appear to be installed on this machine.'
        return [PSCustomObject]$result
    }

    # Best-effort -- shown alongside the verdict so a reader can tell how
    # current the signatures were at scan time, not just that "something"
    # ran. Never fatal: Get-MpComputerStatus can be unavailable even when
    # MpCmdRun.exe itself is present (e.g. the real-time protection
    # service isn't running).
    try {
        $mpStatus = Get-MpComputerStatus -ErrorAction Stop
        $result.signatureVersion = $mpStatus.AntivirusSignatureVersion
        if ($mpStatus.AntivirusSignatureLastUpdated) {
            $result.signatureLastUpdatedUtc = $mpStatus.AntivirusSignatureLastUpdated.ToUniversalTime().ToString('o')
        }
        $result.engineVersion = $mpStatus.AMEngineVersion
    }
    catch {
        # Left null -- not treated as a scan failure.
    }

    $scanStart = Get-Date
    $result.scanStartedUtc = $scanStart.ToUniversalTime().ToString('o')

    try {
        # -DisableRemediation: this is a detection-only pass. Without it,
        # MpCmdRun applies Defender's configured default action (usually
        # Quarantine/Remove) to anything it flags -- unacceptable here
        # since $Path is a customer's mounted package: Stage 1 needs an
        # intact, unmodified copy to keep inventorying after the scan
        # runs, and silently deleting files out from under a mounted
        # VHDX would also corrupt the mount itself, not just the file.
        $output = & $mpCmdRun -Scan -ScanType 3 -File $Path -DisableRemediation 2>&1 | Out-String
        $exitCode = $LASTEXITCODE
        $result.ran = $true
    }
    catch {
        $result.status = 'error'
        $result.message = "Failed to run MpCmdRun.exe: $_"
        return [PSCustomObject]$result
    }
    finally {
        $scanEnd = Get-Date
        $result.scanFinishedUtc = $scanEnd.ToUniversalTime().ToString('o')
        $result.durationSeconds = [math]::Round(($scanEnd - $scanStart).TotalSeconds, 1)
    }
    $result.details = $output.Trim()

    # Primary source: MpCmdRun's own text output, which has proven to be
    # the only source that reliably reports anything for a
    # detection-only (-DisableRemediation) scan.
    $parsedThreats = @(ConvertFrom-MpCmdRunScanOutput -Text $output)

    # Secondary source: only useful when it has a match, since it adds a
    # SeverityID the text output doesn't carry -- but it is NOT required
    # to have one; see the .DESCRIPTION note on why it's unreliable here.
    $detections = @()
    try {
        $detections = @(Get-MpThreatDetection -ErrorAction Stop |
            Where-Object {
                $_.InitialDetectionTime -ge $scanStart.AddMinutes(-1) -and
                (@($_.Resources) | Where-Object { $_ -like "*$Path*" })
            })
    }
    catch {
        # Module missing, or Defender's real-time service isn't running
        # -- not fatal, since $parsedThreats can still confirm a
        # detection on its own.
        $detections = @()
    }

    if ($detections.Count -gt 0) {
        $result.status = 'threats-found'
        $result.threats = @($detections | ForEach-Object {
            [ordered]@{
                threatName = $_.ThreatName
                resources  = @($_.Resources)
                severity   = "$($_.SeverityID)"
            }
        })
    }
    elseif ($parsedThreats.Count -gt 0) {
        $result.status = 'threats-found'
        $result.threats = @($parsedThreats | ForEach-Object {
            [ordered]@{
                threatName = $_.threatName
                resources  = @($_.resources)
                severity   = ''
            }
        })
    }
    elseif ($exitCode -eq 0) {
        $result.status = 'clean'
    }
    else {
        $result.status = 'error'
        $result.message = "MpCmdRun.exe exited with code $exitCode but no threat could be parsed from its output or confirmed via Get-MpThreatDetection:`n$output"
    }

    return [PSCustomObject]$result
}
