<#
.SYNOPSIS
    FlexAppOneDownloadMonitor - Windows system-tray application that monitors
    FlexApp One package downloads in the local Liquidware cache folder.

.DESCRIPTION
    Watches C:\ProgramData\Liquidware\ProfileUnity\Cache\FlexAppOne\ for
    *.download files (the in-progress download extension FlexApp One uses).
    The package size is read directly off that local file (it's pre-allocated
    at its final size the instant it's created, so no remote lookup is
    needed). Real download speed is measured via the LwL.ProfileUnity.Client
    process's write-throughput (Performance Counters), giving an elapsed
    timer, live speed, and an educated-guess ETA. A .token-file fallback also
    catches completions too fast for the .download watcher to see at all
    (logged without a duration, since none was observed). All of this is
    shown from a flyout panel anchored above the system tray icon, with a
    persistent History section below the active downloads.

.NOTES
    Version: 1.0

    WinForms requires an STA thread. If launched under PowerShell 7 (pwsh),
    which defaults to MTA, this script relaunches itself with -STA.

    Run it with:
        powershell.exe -NoProfile -STA -File .\FlexAppOneDownloadMonitor.ps1
    or just double-click / "Run with PowerShell" - the STA relaunch guard
    below handles it either way.
#>

# ---------------------------------------------------------------------------
# STA relaunch guard (WinForms needs a Single Threaded Apartment)
# ---------------------------------------------------------------------------
if ([System.Threading.Thread]::CurrentThread.GetApartmentState() -ne 'STA') {
    $psExe = (Get-Process -Id $PID).Path
    Start-Process -FilePath $psExe -ArgumentList @('-NoProfile', '-STA', '-File', "`"$PSCommandPath`"") | Out-Null
    exit
}

Add-Type -AssemblyName System.Windows.Forms
Add-Type -AssemblyName System.Drawing

Add-Type -Name Win32Focus -Namespace FlexAppMonitor -MemberDefinition @'
    [System.Runtime.InteropServices.DllImport("user32.dll")]
    public static extern System.IntPtr GetForegroundWindow();

    [System.Runtime.InteropServices.DllImport("user32.dll")]
    public static extern bool SetForegroundWindow(System.IntPtr hWnd);
'@

# Tracks the FlexApp One client process's real write-throughput via
# Performance Counters (HKEY_PERFORMANCE_DATA), which - unlike OpenProcess -
# doesn't need a handle to the target process, so it isn't blocked by that
# process running under a different (e.g. SYSTEM) account.
$script:MonitoredPid    = $null
$script:WriteCounter    = $null   # cached System.Diagnostics.PerformanceCounter, re-used so its internal rate calc works
$script:LastProcessScan = [datetime]::MinValue

function Resolve-MonitoredProcess {
    # Only re-scan occasionally (it's cheap but no need every tick), and only
    # if we don't already have a live PID.
    if ($script:MonitoredPid -and (Get-Process -Id $script:MonitoredPid -ErrorAction SilentlyContinue)) { return }
    if (((Get-Date) - $script:LastProcessScan).TotalSeconds -lt 5) { return }
    $script:LastProcessScan = Get-Date

    $script:MonitoredPid = $null
    $script:WriteCounter = $null

    try {
        $proc = Get-Process -Name "*$($script:ProcessNamePattern)*" -ErrorAction SilentlyContinue | Select-Object -First 1
        if (-not $proc) { return }
        $script:MonitoredPid = $proc.Id

        # Performance Counter "Process" instances are named by image name, with
        # #1/#2 suffixes for duplicates - match the right one by its PID via
        # the "ID Process" counter rather than guessing the instance name.
        $category = New-Object System.Diagnostics.PerformanceCounterCategory('Process')
        foreach ($inst in $category.GetInstanceNames()) {
            try {
                $idCounter = New-Object System.Diagnostics.PerformanceCounter('Process', 'ID Process', $inst, $true)
                if ([int]$idCounter.NextValue() -eq $script:MonitoredPid) {
                    $script:WriteCounter = New-Object System.Diagnostics.PerformanceCounter('Process', 'IO Write Bytes/sec', $inst, $true)
                    [void]$script:WriteCounter.NextValue()   # first read is meaningless for a rate counter - prime it
                    break
                }
            } catch {}
        }
    } catch {
        # Process enumeration/perf counters unavailable - leave both null, callers fall back gracefully
    }
}

function Get-MonitoredServiceSpeedBps {
    Resolve-MonitoredProcess
    if (-not $script:WriteCounter) { return 0.0 }
    try {
        return [double]$script:WriteCounter.NextValue()
    } catch {
        $script:WriteCounter = $null   # counter went stale (process restarted etc.) - force a re-resolve next time
        return 0.0
    }
}

# ---------------------------------------------------------------------------
# Configuration
# ---------------------------------------------------------------------------
# Settings persist in a small JSON file next to this script, so the source
# share can be changed (via the tray menu) without editing the script itself.
$script:ConfigPath = Join-Path $PSScriptRoot 'FlexAppOneDownloadMonitor.config.json'
$script:LogPath    = Join-Path $PSScriptRoot 'FlexAppOneDownloadMonitor.log'

function Write-Log {
    param([string]$Message)
    try {
        # Simple rotation so this can't grow forever across a long-running session
        $existing = Get-Item -LiteralPath $script:LogPath -ErrorAction SilentlyContinue
        if ($existing -and $existing.Length -gt 5MB) {
            Move-Item -LiteralPath $script:LogPath -Destination "$($script:LogPath).old" -Force -ErrorAction SilentlyContinue
        }
        $line = "[{0}] {1}" -f (Get-Date -Format 'yyyy-MM-dd HH:mm:ss.fff'), $Message
        Add-Content -LiteralPath $script:LogPath -Value $line -Encoding UTF8
    } catch {
        # Logging must never take the app down - swallow and move on
    }
}

$script:DefaultConfig = @{
    CacheDir = 'C:\ProgramData\Liquidware\ProfileUnity\Cache\FlexAppOne\'
}

function Load-Config {
    if (Test-Path -LiteralPath $script:ConfigPath) {
        try {
            $raw = Get-Content -LiteralPath $script:ConfigPath -Raw | ConvertFrom-Json
            $script:CacheDir = if ($raw.CacheDir) { $raw.CacheDir } else { $script:DefaultConfig.CacheDir }
            return
        } catch {
            # Corrupt or unreadable config - fall through to defaults below
        }
    }
    $script:CacheDir = $script:DefaultConfig.CacheDir
    Save-Config
}

function Save-Config {
    $cfg = @{ CacheDir = $script:CacheDir }
    try {
        $cfg | ConvertTo-Json | Set-Content -LiteralPath $script:ConfigPath -Encoding UTF8
    } catch {
        [System.Windows.Forms.MessageBox]::Show(
            "Could not save settings to`n$($script:ConfigPath)`n`n$($_.Exception.Message)",
            'FlexApp Download Monitor', 'OK', 'Warning') | Out-Null
    }
}

# Fixed - the FlexApp One client process that actually performs the writes.
$script:ProcessNamePattern = 'LwL.ProfileUnity.Client'

Load-Config

$script:PollIntervalMs = 1000

Write-Log "=== FlexApp Download Monitor started (PID $PID) - watching $($script:CacheDir) ==="

# ---------------------------------------------------------------------------
# State
# ---------------------------------------------------------------------------
$script:Downloads      = @{}   # key: full file path -> value: tracking object (hashtable)
$script:History        = New-Object System.Collections.Generic.List[object]   # completed downloads, newest first
$script:NewDownloadDetected = $false   # set by Update-Downloads when a new *.download file shows up
$script:SuppressNextDeactivate = $false   # swallows the one Deactivate event caused by our own focus-restore trick

# A 1s poll can miss a download that starts and finishes faster than that (a
# small package on a fast connection can come and go in well under a second -
# this is exactly what happened to a ~213MB file while a ~2.8GB one next to
# it was still comfortably caught). A FileSystemWatcher fires the instant the
# file is created, so even a sub-second download gets its start time recorded
# before it vanishes. The watcher's own events fire on a background thread,
# so they only ever push into these thread-safe queues; all actual mutation
# of $script:Downloads still happens on the UI thread inside Update-Downloads.
$script:CreatedQueue = New-Object 'System.Collections.Concurrent.ConcurrentQueue[hashtable]'
$script:RemovedQueue = New-Object 'System.Collections.Concurrent.ConcurrentQueue[hashtable]'
$script:TokenQueue   = New-Object 'System.Collections.Concurrent.ConcurrentQueue[hashtable]'   # .token writes - catches completions too fast for the .download watcher to see at all
$script:LastReconcile = [datetime]::MinValue
$script:RecentlyCompletedNames = @{}    # Name -> last-completed time, so a .token event doesn't double-log something the .download watcher already logged
$script:LastTokenLogged        = @{}    # Name -> last-logged time, dedupes repeat Created+Changed events for the same token write
$script:LastTokenEventSeen     = $null  # raw last-seen token event, regardless of dedup outcome - for Diagnostics

try {
    $script:Watcher = New-Object System.IO.FileSystemWatcher
    $script:Watcher.Path = $script:CacheDir
    $script:Watcher.IncludeSubdirectories = $true
    $script:Watcher.InternalBufferSize = 65536   # bigger buffer, less chance of dropped events during a burst
    $script:Watcher.NotifyFilter = [System.IO.NotifyFilters]'FileName, LastWrite'
    # Filter left as '*.*' and matched manually below - built-in Filter
    # matching against Renamed events (old name vs. new name) is inconsistent
    # across .NET versions, so we check extensions ourselves instead.
    $script:Watcher.Filter = '*.*'

    # IMPORTANT: a Register-ObjectEvent's -Action block is only auto-executed
    # by the engine when the runspace goes idle between pipeline commands.
    # This entire script IS one single continuously-running pipeline (it ends
    # by blocking forever inside Application.Run()), so that idle moment never
    # arrives and -Action blocks here would never actually run. Instead we
    # register bare subscriptions (no -Action) and manually pump their queued
    # events ourselves every tick via Get-Event/Remove-Event, which works
    # regardless of pipeline/idle state since we're the ones calling it.
    Unregister-Event -SourceIdentifier 'FlexAppWatcherCreated' -ErrorAction SilentlyContinue
    Unregister-Event -SourceIdentifier 'FlexAppWatcherDeleted' -ErrorAction SilentlyContinue
    Unregister-Event -SourceIdentifier 'FlexAppWatcherRenamed' -ErrorAction SilentlyContinue
    Unregister-Event -SourceIdentifier 'FlexAppWatcherTokenCreated' -ErrorAction SilentlyContinue
    Unregister-Event -SourceIdentifier 'FlexAppWatcherTokenChanged' -ErrorAction SilentlyContinue

    Register-ObjectEvent -InputObject $script:Watcher -EventName Created -SourceIdentifier 'FlexAppWatcherCreated' | Out-Null
    Register-ObjectEvent -InputObject $script:Watcher -EventName Deleted -SourceIdentifier 'FlexAppWatcherDeleted' | Out-Null
    Register-ObjectEvent -InputObject $script:Watcher -EventName Renamed -SourceIdentifier 'FlexAppWatcherRenamed' | Out-Null
    Register-ObjectEvent -InputObject $script:Watcher -EventName Created -SourceIdentifier 'FlexAppWatcherTokenCreated' | Out-Null
    Register-ObjectEvent -InputObject $script:Watcher -EventName Changed -SourceIdentifier 'FlexAppWatcherTokenChanged' | Out-Null

    $script:Watcher.EnableRaisingEvents = $true
    Write-Log "Watcher started on: $($script:CacheDir)"
} catch {
    $script:Watcher = $null   # cache dir missing/inaccessible at startup - Update-Downloads still works via polling alone
    Write-Log "Watcher setup FAILED: $($_.Exception.Message)"
}

# Manually pumps every queued watcher event into our own thread-safe queues.
# Called once per tick from Update-Downloads - see the big comment above for
# why this replaces relying on Register-ObjectEvent's automatic -Action.
function Receive-WatcherEvents {
    foreach ($sourceId in 'FlexAppWatcherCreated', 'FlexAppWatcherDeleted', 'FlexAppWatcherRenamed', 'FlexAppWatcherTokenCreated', 'FlexAppWatcherTokenChanged') {
        $events = Get-Event -SourceIdentifier $sourceId -ErrorAction SilentlyContinue
        foreach ($evt in $events) {
            $args = $evt.SourceEventArgs
            $time = $evt.TimeGenerated

            # Log every raw event unconditionally, regardless of whether it
            # matches .download/.token below - this is the ground truth of
            # what's actually happening in the folder, independent of our
            # own filtering/tracking logic.
            if ($sourceId -eq 'FlexAppWatcherRenamed') {
                Write-Log "RAW EVENT [$sourceId]: '$($args.OldFullPath)' -> '$($args.FullPath)'"
            } else {
                Write-Log "RAW EVENT [$sourceId]: '$($args.FullPath)'"
            }

            switch ($sourceId) {
                'FlexAppWatcherCreated' {
                    if ($args.FullPath -like '*.download') {
                        $script:CreatedQueue.Enqueue(@{ Path = $args.FullPath; Time = $time })
                    }
                }
                'FlexAppWatcherDeleted' {
                    if ($args.FullPath -like '*.download') {
                        $script:RemovedQueue.Enqueue(@{ Path = $args.FullPath; Time = $time })
                    }
                }
                'FlexAppWatcherRenamed' {
                    if ($args.OldFullPath -like '*.download') {
                        $script:RemovedQueue.Enqueue(@{ Path = $args.OldFullPath; Time = $time })
                    }
                    if ($args.FullPath -like '*.download') {
                        $script:CreatedQueue.Enqueue(@{ Path = $args.FullPath; Time = $time })
                    }
                    if ($args.FullPath -like '*.token') {
                        $script:LastTokenEventSeen = @{ Path = $args.FullPath; Time = $time; Kind = 'Renamed' }
                        $script:TokenQueue.Enqueue(@{ Path = $args.FullPath; Time = $time })
                    }
                }
                'FlexAppWatcherTokenCreated' {
                    if ($args.FullPath -like '*.token') {
                        $script:LastTokenEventSeen = @{ Path = $args.FullPath; Time = $time; Kind = 'Created' }
                        $script:TokenQueue.Enqueue(@{ Path = $args.FullPath; Time = $time })
                    }
                }
                'FlexAppWatcherTokenChanged' {
                    if ($args.FullPath -like '*.token') {
                        $script:LastTokenEventSeen = @{ Path = $args.FullPath; Time = $time; Kind = 'Changed' }
                        $script:TokenQueue.Enqueue(@{ Path = $args.FullPath; Time = $time })
                    }
                }
            }
            Remove-Event -EventIdentifier $evt.EventIdentifier
        }
    }
}

# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------
function Format-DisplayName {
    param([string]$RawName)
    # The .download suffix is already stripped by the caller; this handles the
    # installer extension underneath it (e.g. "7-Zip.exe" -> "7-Zip"), then
    # turns hyphens into spaces for a cleaner display name (e.g. "Rancher-Desktop" -> "Rancher Desktop").
    $name = $RawName -replace '\.(exe|msi|msix|appx|zip)$', ''
    $name = $name -replace '-', ' '
    return $name.Trim()
}

function Format-Bytes {
    param([double]$Bytes)
    if ($Bytes -ge 1GB) { return ('{0:N2} GB' -f ($Bytes / 1GB)) }
    if ($Bytes -ge 1MB) { return ('{0:N1} MB' -f ($Bytes / 1MB)) }
    if ($Bytes -ge 1KB) { return ('{0:N0} KB' -f ($Bytes / 1KB)) }
    return ('{0:N0} B' -f $Bytes)
}

function Format-MB {
    param([double]$Bytes)
    return ('{0:N0} MB' -f ($Bytes / 1MB))
}

function Format-Seconds {
    param([double]$Seconds)
    if ($Seconds -le 0 -or [double]::IsInfinity($Seconds) -or [double]::IsNaN($Seconds)) { return '--' }
    $ts = [TimeSpan]::FromSeconds($Seconds)
    if ($ts.TotalHours -ge 1) { return ('{0:h\:mm\:ss}' -f $ts) }
    return ('{0:m\:ss}' -f $ts)
}

# ---------------------------------------------------------------------------
# Shared tracking helpers - used by both the real-time event queue drain and
# the periodic reconciliation poll, so both paths behave identically.
# ---------------------------------------------------------------------------
function Add-TrackedDownload {
    param([string]$Path, [datetime]$StartTime, [Nullable[int64]]$KnownSize = $null)
    if ($script:Downloads.ContainsKey($Path)) { return }

    # The .download file is pre-allocated at its final size the instant it's
    # created, so we can just read that off the local file directly - no need
    # for a remote lookup against the source share at all.
    $totalSize = $KnownSize
    if (-not $totalSize) {
        try { $totalSize = (Get-Item -LiteralPath $Path -ErrorAction Stop).Length } catch { $totalSize = $null }
    }

    $baseName = [System.IO.Path]::GetFileNameWithoutExtension((Split-Path $Path -Leaf))  # strips ".download"
    $displayName = Format-DisplayName -RawName $baseName
    $d = @{
        Name      = $displayName
        FilePath  = $Path
        StartTime = $StartTime
        Status    = 'Downloading'
        TotalSize = $totalSize
        SpeedBps  = 0.0
    }
    $script:Downloads[$Path] = $d
    $script:NewDownloadDetected = $true
    Write-Log "TRACKED new download: '$displayName' path='$Path' start=$($StartTime.ToString('HH:mm:ss.fff')) size=$totalSize"
}

function Complete-TrackedDownload {
    param([string]$Path, [datetime]$EndTime)
    if (-not $script:Downloads.ContainsKey($Path)) { return }

    $d = $script:Downloads[$Path]
    $duration = ($EndTime - $d.StartTime).TotalSeconds
    if ($duration -lt 0) { $duration = 0 }
    Write-Log "COMPLETED (.download): '$($d.Name)' path='$Path' duration=$([Math]::Round($duration,2))s"

    $script:History.Insert(0, @{
        Id          = [guid]::NewGuid().ToString()
        Name        = $d.Name
        DurationSec = $duration
        CompletedAt = $EndTime
    })
    if ($script:History.Count -gt 200) {
        $script:History.RemoveAt($script:History.Count - 1)
    }

    $script:Downloads.Remove($Path)
    $script:RecentlyCompletedNames[$d.Name] = $EndTime
}

function Get-AppNameFromTokenPath {
    param([string]$Path)
    $fileName = Split-Path $Path -Leaf
    # Token files are named "<appfile>.s-<SID>.token", e.g.
    # "nomacs.exe.s-1-5-21-3263492410-1156889656-679387848-1390.token"
    $baseName = $fileName -replace '\.s-\d+(-\d+)+\.token$', ''
    if ($baseName -eq $fileName) { return $null }   # didn't match the expected pattern - ignore
    return Format-DisplayName -RawName $baseName
}

function Add-InstantHistoryEntry {
    param([string]$Name, [datetime]$Time)

    Write-Log "TOKEN EVENT parsed name='$Name' time=$($Time.ToString('HH:mm:ss.fff'))"

    # Already actively tracked via the .download watcher - its own completion
    # will log this properly with a real duration, so don't double up here.
    foreach ($d in $script:Downloads.Values) {
        if ($d.Name -eq $Name) {
            Write-Log "  -> skipped: '$Name' already actively tracked via .download"
            return
        }
    }
    # Already completed via the .download watcher very recently - same story.
    if ($script:RecentlyCompletedNames.ContainsKey($Name) -and
        ($Time - $script:RecentlyCompletedNames[$Name]).TotalSeconds -lt 15) {
        Write-Log "  -> skipped: '$Name' completed via .download within the last 15s"
        return
    }
    # Multiple token events (Created + Changed) can fire for the same single
    # write - don't log more than one entry per name within a short window.
    if ($script:LastTokenLogged.ContainsKey($Name) -and
        ($Time - $script:LastTokenLogged[$Name]).TotalSeconds -lt 5) {
        Write-Log "  -> skipped: '$Name' already logged from a token event within the last 5s"
        return
    }
    $script:LastTokenLogged[$Name] = $Time

    # We only have a completion timestamp here, not a start time - this was
    # too fast for the .download watcher to see at all, so duration is
    # genuinely unknown rather than a real measurement.
    $script:History.Insert(0, @{
        Id          = [guid]::NewGuid().ToString()
        Name        = $Name
        DurationSec = $null
        CompletedAt = $Time
    })
    if ($script:History.Count -gt 200) {
        $script:History.RemoveAt($script:History.Count - 1)
    }
    $script:NewDownloadDetected = $true
    Write-Log "  -> LOGGED to History: '$Name'"
}

# ---------------------------------------------------------------------------
# Core scan / update logic - called on every timer tick
# ---------------------------------------------------------------------------
# Note: FlexApp One pre-allocates the .download file at its final size, so
# file-size growth doesn't reflect real progress. Instead we track how long
# the .download file has existed (StartTime -> now while active, StartTime ->
# disappearance for the final duration shown in History).
#
# Detection is event-driven (FileSystemWatcher, see State section above) so
# that even a download which starts and finishes faster than our poll
# interval still gets caught. A full-directory poll still runs every few
# seconds as a safety net in case an event ever gets dropped (e.g. a very
# large burst overflowing the watcher's buffer) or to catch anything already
# in progress before this app started watching.
function Update-Downloads {
    $now = Get-Date

    # Manually pump the watcher's queued events (see Receive-WatcherEvents for
    # why this can't just rely on Register-ObjectEvent's automatic -Action).
    Receive-WatcherEvents

    # Drain real-time events first - this is what catches very fast downloads
    # that a periodic poll alone could step right over.
    $item = $null
    while ($script:CreatedQueue.TryDequeue([ref]$item)) {
        Add-TrackedDownload -Path $item.Path -StartTime $item.Time
    }
    while ($script:RemovedQueue.TryDequeue([ref]$item)) {
        Complete-TrackedDownload -Path $item.Path -EndTime $item.Time
    }
    while ($script:TokenQueue.TryDequeue([ref]$item)) {
        $name = Get-AppNameFromTokenPath -Path $item.Path
        if ($name) { Add-InstantHistoryEntry -Name $name -Time $item.Time }
    }

    # Periodic reconciliation safety net
    if (($now - $script:LastReconcile).TotalSeconds -ge 3) {
        $script:LastReconcile = $now
        $seenKeys = New-Object System.Collections.Generic.HashSet[string]

        try {
            $files = Get-ChildItem -LiteralPath $script:CacheDir -Filter '*.download' -Recurse -File -ErrorAction SilentlyContinue
        } catch {
            $files = @()
        }

        foreach ($f in $files) {
            [void]$seenKeys.Add($f.FullName)
            # Use the file's own creation time when we catch it late via the
            # poll, rather than "now" - gives an accurate elapsed/duration
            # even for something the watcher's events happened to miss.
            Add-TrackedDownload -Path $f.FullName -StartTime $f.CreationTime -KnownSize $f.Length
        }

        foreach ($key in @($script:Downloads.Keys)) {
            if (-not $seenKeys.Contains($key)) {
                Complete-TrackedDownload -Path $key -EndTime $now
            }
        }
    }

    # Sample the ProfileUnity client service's real write-throughput once per
    # tick and apply it to every currently active download (this service
    # handles all FlexApp downloads, so if more than one runs at once this
    # will reflect their combined speed rather than a single file's speed).
    if ($script:Downloads.Count -gt 0) {
        $speed = Get-MonitoredServiceSpeedBps
        foreach ($d in $script:Downloads.Values) { $d.SpeedBps = $speed }
    }
}

# ---------------------------------------------------------------------------
# UI - flyout panel
# ---------------------------------------------------------------------------
$script:Flyout = New-Object System.Windows.Forms.Form
$script:Flyout.FormBorderStyle = 'None'
$script:Flyout.StartPosition   = 'Manual'
$script:Flyout.ShowInTaskbar   = $false
$script:Flyout.TopMost         = $true
$script:Flyout.BackColor       = [System.Drawing.Color]::FromArgb(32, 34, 38)
$script:Flyout.Width           = 380
$script:Flyout.Height          = 380

$script:TopBarHeight = 32

$topBar = New-Object System.Windows.Forms.Panel
$topBar.Location = New-Object System.Drawing.Point(0, 0)
$topBar.Size = New-Object System.Drawing.Size($script:Flyout.Width, $script:TopBarHeight)
$script:Flyout.Controls.Add($topBar)

$titleLabel = New-Object System.Windows.Forms.Label
$titleLabel.Text = 'FlexApp One Downloads'
$titleLabel.ForeColor = [System.Drawing.Color]::White
$titleLabel.Font = New-Object System.Drawing.Font('Segoe UI', 10, [System.Drawing.FontStyle]::Bold)
$titleLabel.AutoSize = $true
$titleLabel.Location = New-Object System.Drawing.Point(8, 7)
$topBar.Controls.Add($titleLabel)

$clearHistoryButton = New-Object System.Windows.Forms.Button
$clearHistoryButton.Text = 'Clear history'
$clearHistoryButton.FlatStyle = 'Flat'
$clearHistoryButton.FlatAppearance.BorderSize = 0
$clearHistoryButton.ForeColor = [System.Drawing.Color]::LightGray
$clearHistoryButton.BackColor = $script:Flyout.BackColor
$clearHistoryButton.Font = New-Object System.Drawing.Font('Segoe UI', 8)
$clearHistoryButton.Size = New-Object System.Drawing.Size(90, 24)
$clearHistoryButton.Location = New-Object System.Drawing.Point(($topBar.Width - 90 - 8), 4)
$clearHistoryButton.Add_Click({
    $script:History.Clear()
    Update-FlyoutUI
})
$topBar.Controls.Add($clearHistoryButton)

# Explicit bounds (not Dock='Fill') so this can never overlap the top bar
# regardless of z-order - Dock='Fill' next to a Dock='Top' sibling is a
# well-known WinForms trap where the fill area doesn't always respect the
# top-docked control's reserved space.
$listPanel = New-Object System.Windows.Forms.Panel
$listPanel.Location = New-Object System.Drawing.Point(0, $script:TopBarHeight)
$listPanel.Size = New-Object System.Drawing.Size($script:Flyout.Width, ($script:Flyout.Height - $script:TopBarHeight))
$listPanel.AutoScroll = $true
$listPanel.BackColor = $script:Flyout.BackColor
$script:Flyout.Controls.Add($listPanel)

# Small helper: builds one row panel. $Kind is 'active' or 'history' - active
# rows show a live elapsed-time counter + marquee bar, history rows show the
# final duration and a completion timestamp.
function New-DownloadRow {
    param([string]$Kind, [string]$Name, [string]$DetailText, [string]$DetailText2, [string]$SizeText)

    $height = if ($Kind -eq 'active') { 78 } else { 40 }

    $panel = New-Object System.Windows.Forms.Panel
    $panel.Width  = $listPanel.ClientSize.Width - 4
    $panel.Height = $height
    $panel.Margin = New-Object System.Windows.Forms.Padding(0, 0, 0, 6)
    $panel.BackColor = [System.Drawing.Color]::FromArgb(44, 46, 51)

    $nameLabel = New-Object System.Windows.Forms.Label
    $nameLabel.ForeColor = [System.Drawing.Color]::White
    $nameLabel.Font = New-Object System.Drawing.Font('Segoe UI', 8.5, [System.Drawing.FontStyle]::Bold)
    $nameLabel.AutoEllipsis = $true
    $nameLabel.Text = $Name
    $nameLabel.Location = New-Object System.Drawing.Point(8, 5)
    $nameLabel.Size = New-Object System.Drawing.Size(($panel.Width - 16), 18)
    $panel.Controls.Add($nameLabel)

    if ($Kind -eq 'active') {
        $bar = New-Object System.Windows.Forms.ProgressBar
        $bar.Style = 'Marquee'
        $bar.MarqueeAnimationSpeed = 30
        $bar.Location = New-Object System.Drawing.Point(8, 27)
        $bar.Size = New-Object System.Drawing.Size(($panel.Width - 16), 10)
        $panel.Controls.Add($bar)
        $detailTop = 42
    } else {
        $detailTop = 24
    }

    $detailLabel = New-Object System.Windows.Forms.Label
    $detailLabel.ForeColor = [System.Drawing.Color]::Gainsboro
    $detailLabel.Font = New-Object System.Drawing.Font('Segoe UI', 8)
    $detailLabel.Text = $DetailText
    $detailLabel.Location = New-Object System.Drawing.Point(8, $detailTop)
    $detailLabel.Size = New-Object System.Drawing.Size(($panel.Width - 16), 16)
    $panel.Controls.Add($detailLabel)

    if ($Kind -eq 'active' -and $DetailText2) {
        $detailLabel2 = New-Object System.Windows.Forms.Label
        $detailLabel2.ForeColor = [System.Drawing.Color]::FromArgb(120, 190, 255)
        $detailLabel2.Font = New-Object System.Drawing.Font('Segoe UI', 8)
        $detailLabel2.Text = $DetailText2
        $detailLabel2.Location = New-Object System.Drawing.Point(8, ($detailTop + 18))
        $detailLabel2.Size = New-Object System.Drawing.Size(($panel.Width - 16), 16)
        $panel.Controls.Add($detailLabel2)
    }

    if ($SizeText) {
        $sizeLabel = New-Object System.Windows.Forms.Label
        $sizeLabel.ForeColor = [System.Drawing.Color]::DimGray
        $sizeLabel.Font = New-Object System.Drawing.Font('Segoe UI', 7.5)
        $sizeLabel.Text = $SizeText
        $sizeLabel.TextAlign = 'TopRight'
        $sizeLabel.Location = New-Object System.Drawing.Point(8, 4)
        $sizeLabel.Size = New-Object System.Drawing.Size(($panel.Width - 16), 16)
        $panel.Controls.Add($sizeLabel)
    }

    return $panel
}

function New-SectionHeader {
    param([string]$Text)
    $lbl = New-Object System.Windows.Forms.Label
    $lbl.Text = $Text
    $lbl.ForeColor = [System.Drawing.Color]::Gray
    $lbl.Font = New-Object System.Drawing.Font('Segoe UI', 8, [System.Drawing.FontStyle]::Bold)
    $lbl.Width = $listPanel.ClientSize.Width - 4
    $lbl.Height = 18
    $lbl.Margin = New-Object System.Windows.Forms.Padding(0, 4, 0, 4)
    return $lbl
}

# Full rebuild each refresh - simplest way to keep active/history sections in
# sync, and cheap enough at a 1s tick for the handful of rows this ever holds.
function Update-FlyoutUI {
    $listPanel.SuspendLayout()
    $listPanel.Controls.Clear()

    $y = 0
    $now = Get-Date

    if ($script:Downloads.Count -gt 0) {
        $header = New-SectionHeader -Text 'ACTIVE'
        $header.Top = $y
        $listPanel.Controls.Add($header)
        $y += $header.Height

        foreach ($key in $script:Downloads.Keys) {
            $d = $script:Downloads[$key]
            $elapsed = ($now - $d.StartTime).TotalSeconds

            $sizePart = if ($d.TotalSize) { Format-MB $d.TotalSize } else { 'size unknown' }

            # Educated guess: assume the current write speed holds roughly
            # steady for the whole download, estimate total time from that,
            # then subtract what's already elapsed. We don't know exactly how
            # many bytes of *this* file are down yet (the process counter is
            # cumulative for the whole process), so this is an approximation,
            # not an exact remaining-bytes calculation.
            $speedLine = $null
            if ($d.SpeedBps -gt 1KB -and $d.TotalSize) {
                $estimatedTotalSec = $d.TotalSize / $d.SpeedBps
                $etaSec = $estimatedTotalSec - $elapsed
                if ($etaSec -lt 0) { $etaSec = 0 }
                $speedLine = "$(Format-Bytes $d.SpeedBps)/s  -  ETA ~$(Format-Seconds $etaSec)"
            }

            $row = New-DownloadRow -Kind 'active' -Name $d.Name `
                -DetailText "$(Format-Seconds $elapsed) elapsed  -  $sizePart" `
                -DetailText2 $speedLine
            $row.Top = $y
            $listPanel.Controls.Add($row)
            $y += $row.Height + 6
        }
    }

    if ($script:History.Count -gt 0) {
        $header = New-SectionHeader -Text 'HISTORY'
        $header.Top = $y
        $listPanel.Controls.Add($header)
        $y += $header.Height

        foreach ($h in $script:History) {
            $detailText = if ($null -ne $h.DurationSec) {
                "Completed in $(Format-Seconds $h.DurationSec)  -  $($h.CompletedAt.ToString('HH:mm:ss'))"
            } else {
                "Completed  -  $($h.CompletedAt.ToString('HH:mm:ss'))"
            }
            $row = New-DownloadRow -Kind 'history' -Name $h.Name -DetailText $detailText
            $row.Top = $y
            $listPanel.Controls.Add($row)
            $y += $row.Height + 6
        }
    }

    if ($script:Downloads.Count -eq 0 -and $script:History.Count -eq 0) {
        $empty = New-Object System.Windows.Forms.Label
        $empty.Text = 'No downloads yet'
        $empty.ForeColor = [System.Drawing.Color]::Gainsboro
        $empty.Font = New-Object System.Drawing.Font('Segoe UI', 9)
        $empty.AutoSize = $true
        $empty.Location = New-Object System.Drawing.Point(4, 4)
        $listPanel.Controls.Add($empty)
    }

    $listPanel.ResumeLayout()
}

function Show-Flyout {
    param([switch]$Activate)

    $wa = [System.Windows.Forms.Screen]::PrimaryScreen.WorkingArea
    $script:Flyout.Left = $wa.Right - $script:Flyout.Width - 8
    $script:Flyout.Top  = $wa.Bottom - $script:Flyout.Height - 8
    Update-FlyoutUI

    if ($Activate) {
        $script:Flyout.Show()
        $script:Flyout.Activate()
    } else {
        # Auto-popup: remember whatever window currently has focus and hand
        # focus straight back to it after showing, so we don't interrupt typing.
        $prevForeground = [IntPtr]::Zero
        try {
            $prevForeground = [FlexAppMonitor.Win32Focus]::GetForegroundWindow()
        } catch {
            Write-Log "Show-Flyout: GetForegroundWindow failed: $($_.Exception.Message)"
        }
        $script:SuppressNextDeactivate = $true
        $script:Flyout.Show()
        try {
            [void][FlexAppMonitor.Win32Focus]::SetForegroundWindow($prevForeground)
        } catch {
            Write-Log "Show-Flyout: SetForegroundWindow failed: $($_.Exception.Message)"
        }
    }
}

$script:Flyout.Add_Deactivate({
    if ($script:SuppressNextDeactivate) {
        $script:SuppressNextDeactivate = $false
        return
    }
    $script:Flyout.Hide()
})

# ---------------------------------------------------------------------------
# Tray icon
# ---------------------------------------------------------------------------
function New-TrayIcon {
    $bmp = New-Object System.Drawing.Bitmap 32, 32
    $g = [System.Drawing.Graphics]::FromImage($bmp)
    $g.SmoothingMode = 'AntiAlias'
    $g.Clear([System.Drawing.Color]::Transparent)
    $brush = New-Object System.Drawing.SolidBrush ([System.Drawing.Color]::FromArgb(0, 120, 215))
    $g.FillEllipse($brush, 1, 1, 30, 30)
    $pen = New-Object System.Drawing.Pen ([System.Drawing.Color]::White), 2.5
    $g.DrawLine($pen, 16, 8, 16, 21)
    $g.DrawLine($pen, 9, 15, 16, 22)
    $g.DrawLine($pen, 23, 15, 16, 22)
    $g.DrawLine($pen, 8, 25, 24, 25)
    $g.Dispose()
    $hicon = $bmp.GetHicon()
    return [System.Drawing.Icon]::FromHandle($hicon)
}

$script:TrayIcon = New-Object System.Windows.Forms.NotifyIcon
$script:TrayIcon.Icon = New-TrayIcon
$script:TrayIcon.Text = 'FlexApp Download Monitor'
$script:TrayIcon.Visible = $true

$menu = New-Object System.Windows.Forms.ContextMenuStrip
$itemShow = $menu.Items.Add('Show downloads')
$itemRefresh = $menu.Items.Add('Refresh now')
[void]$menu.Items.Add('-')
$itemDiagnostics = $menu.Items.Add('Diagnostics...')
$itemOpenLog = $menu.Items.Add('Open log file')
[void]$menu.Items.Add('-')
$itemExit = $menu.Items.Add('Exit')
$script:TrayIcon.ContextMenuStrip = $menu

$itemShow.Add_Click({ Show-Flyout -Activate })
$itemRefresh.Add_Click({ Update-Downloads; Update-FlyoutUI })

$itemDiagnostics.Add_Click({
    Update-Downloads   # force a fresh sample before reporting

    $lines = New-Object System.Collections.Generic.List[string]
    $lines.Add("Log file: $($script:LogPath)")
    $lines.Add("Monitored PID: $($script:MonitoredPid)")
    $lines.Add("Write counter attached: $([bool]$script:WriteCounter)")
    $lines.Add("Watcher active: $([bool]$script:Watcher -and $script:Watcher.EnableRaisingEvents)")
    if ($script:LastTokenEventSeen) {
        $lines.Add("Last raw token event: $($script:LastTokenEventSeen.Kind) at $($script:LastTokenEventSeen.Time.ToString('HH:mm:ss.fff')) - $($script:LastTokenEventSeen.Path)")
    } else {
        $lines.Add("Last raw token event: (none seen yet)")
    }
    $lines.Add("History entries: $($script:History.Count)")
    $lines.Add("")

    if ($script:Downloads.Count -eq 0) {
        $lines.Add("No active downloads right now.")
    } else {
        foreach ($d in $script:Downloads.Values) {
            $lines.Add("Name: $($d.Name)")
            $lines.Add("  TotalSize: $(if ($d.TotalSize) { Format-Bytes $d.TotalSize } else { '(unknown)' })")
            $lines.Add("  SpeedBps (raw): $([Math]::Round($d.SpeedBps, 1)) B/s")
            $lines.Add("")
        }
    }

    [System.Windows.Forms.MessageBox]::Show(
        ($lines -join "`n"), 'FlexApp Download Monitor - Diagnostics', 'OK', 'Information') | Out-Null
})
$itemOpenLog.Add_Click({
    if (-not (Test-Path -LiteralPath $script:LogPath)) {
        Write-Log "(log file created)"
    }
    try {
        Start-Process -FilePath 'notepad.exe' -ArgumentList "`"$($script:LogPath)`""
    } catch {
        [System.Windows.Forms.MessageBox]::Show(
            "Could not open the log file:`n$($script:LogPath)`n`n$($_.Exception.Message)",
            'FlexApp Download Monitor', 'OK', 'Warning') | Out-Null
    }
})
$itemExit.Add_Click({
    Write-Log "=== FlexApp Download Monitor exiting ==="
    $script:Timer.Stop()
    if ($script:Watcher) {
        $script:Watcher.EnableRaisingEvents = $false
        Unregister-Event -SourceIdentifier 'FlexAppWatcherCreated' -ErrorAction SilentlyContinue
        Unregister-Event -SourceIdentifier 'FlexAppWatcherDeleted' -ErrorAction SilentlyContinue
        Unregister-Event -SourceIdentifier 'FlexAppWatcherRenamed' -ErrorAction SilentlyContinue
        Unregister-Event -SourceIdentifier 'FlexAppWatcherTokenCreated' -ErrorAction SilentlyContinue
        Unregister-Event -SourceIdentifier 'FlexAppWatcherTokenChanged' -ErrorAction SilentlyContinue
        $script:Watcher.Dispose()
    }
    $script:TrayIcon.Visible = $false
    $script:TrayIcon.Dispose()
    [System.Windows.Forms.Application]::Exit()
})

$script:TrayIcon.Add_MouseClick({
    param($sender, $e)
    if ($e.Button -eq [System.Windows.Forms.MouseButtons]::Left) {
        if ($script:Flyout.Visible) { $script:Flyout.Hide() } else { Show-Flyout -Activate }
    }
})

# ---------------------------------------------------------------------------
# Timer loop
# ---------------------------------------------------------------------------
$script:Timer = New-Object System.Windows.Forms.Timer
$script:Timer.Interval = $script:PollIntervalMs
$script:Timer.Add_Tick({
    $script:NewDownloadDetected = $false
    Update-Downloads

    $count = $script:Downloads.Count
    if ($count -eq 0) {
        $script:TrayIcon.Text = 'FlexApp Download Monitor - idle'
    } elseif ($count -eq 1) {
        $only = $script:Downloads.Values | Select-Object -First 1
        $sizeStr = if ($only.TotalSize) { " ($(Format-MB $only.TotalSize))" } else { '' }
        $tip = "FlexApp Download Monitor - $($only.Name)$sizeStr"
        $script:TrayIcon.Text = if ($tip.Length -gt 63) { $tip.Substring(0, 60) + '...' } else { $tip }
    } else {
        $script:TrayIcon.Text = "FlexApp Download Monitor - $count active"
    }

    if ($script:NewDownloadDetected -and -not $script:Flyout.Visible) {
        Write-Log "Auto-popup trigger: NewDownloadDetected=True, Flyout.Visible=$($script:Flyout.Visible) - calling Show-Flyout"
        try {
            # Pop open automatically without stealing focus from whatever the user is doing
            Show-Flyout
        } catch {
            Write-Log "Auto-popup FAILED: $($_.Exception.Message)"
        }
    } elseif ($script:NewDownloadDetected) {
        Write-Log "Auto-popup SKIPPED: NewDownloadDetected=True but Flyout.Visible was already $($script:Flyout.Visible)"
        Update-FlyoutUI
    } elseif ($script:Flyout.Visible) {
        Update-FlyoutUI
    }
})
$script:Timer.Start()

# Initial pass so the flyout has data the first time it's opened
Update-Downloads

[System.Windows.Forms.Application]::EnableVisualStyles()
[System.Windows.Forms.Application]::Run()
