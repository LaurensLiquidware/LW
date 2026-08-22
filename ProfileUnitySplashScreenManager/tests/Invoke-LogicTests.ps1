<#
.SYNOPSIS
    Logic tests for ProfileUnity SplashScreen Logo Manager.
.DESCRIPTION
    Evidence for sections 1 and 2 of the Sparks Tool Project Review Checklist:
    exercises the manifest, timestamp and file-path logic against a temporary
    fixture, with double-byte / non-Latin / wildcard-metacharacter filenames and
    under a range of cultures.

    The functions under test are lifted straight out of the application with the
    PowerShell parser, so there is no second copy of the logic to drift: if the
    app changes, these tests run against the change.

    Nothing here touches the real ProfileUnity folder, ProgramData, or the
    clipboard, and no WPF type is loaded -- so it runs on Windows PowerShell 5.1,
    PowerShell 7 on Windows, and PowerShell 7 on Linux alike.

    This covers the logic only. The UI, the clipboard import, the dimension read
    (System.Drawing) and the splash-screen preview still need a manual pass on
    Windows.
.EXAMPLE
    pwsh -File .\Invoke-LogicTests.ps1
#>

param(
    [string]$AppScript = (Join-Path (Split-Path -Parent $PSScriptRoot) 'Set-ProfileUnitySplashScreenLogo.ps1')
)

$ErrorActionPreference = 'Stop'

if (-not (Test-Path -LiteralPath $AppScript)) { throw "Application script not found: $AppScript" }

# --- lift the functions under test out of the app via the real parser ---------
$parseErrors = $null; $parseTokens = $null
$ast = [System.Management.Automation.Language.Parser]::ParseFile(
    (Resolve-Path -LiteralPath $AppScript).Path, [ref]$parseTokens, [ref]$parseErrors)
if ($parseErrors -and $parseErrors.Count) {
    Write-Host "The application script does not parse:" -ForegroundColor Red
    $parseErrors | ForEach-Object { Write-Host ("  line {0}: {1}" -f $_.Extent.StartLineNumber, $_.Message) }
    exit 1
}
Write-Host "$([IO.Path]::GetFileName($AppScript)) parses clean ($($parseTokens.Count) tokens)"

$wanted = @('Get-TimestampString','ConvertTo-SortableDate','Get-Manifest','Save-Manifest','Get-CurrentMeta',
            'Get-AllLiveLogos','Get-ExistingLiveLogo','Archive-CurrentLogo','Set-NewLogo',
            'Restore-FromHistoryEntry','Remove-HistoryEntry')
$found = @($ast.FindAll({ $args[0] -is [System.Management.Automation.Language.FunctionDefinitionAst] }, $false) |
           Where-Object { $wanted -contains $_.Name })
$missingFns = @($wanted | Where-Object { $n = $_; -not ($found | Where-Object { $_.Name -eq $n }) })
if ($missingFns.Count) { throw "These functions are no longer in the app script: $($missingFns -join ', ')" }

$extracted = Join-Path ([IO.Path]::GetTempPath()) ("pu-functions-" + [guid]::NewGuid().ToString('N').Substring(0,8) + ".ps1")
Set-Content -LiteralPath $extracted -Value (($found | ForEach-Object { $_.Extent.Text }) -join "`n`n") -Encoding UTF8
Write-Host "Extracted $($found.Count) functions under test"

$pass = 0; $fail = 0
function Check([string]$name, [bool]$ok, [string]$detail = '') {
    if ($ok) { $script:pass++; Write-Host "  PASS  $name" }
    else     { $script:fail++; Write-Host "  FAIL  $name  $detail" }
}

# --- test fixture: stand up the script's data/target layout in a temp tree ----
$root = Join-Path ([IO.Path]::GetTempPath()) ("putest-" + [guid]::NewGuid().ToString('N').Substring(0,8))
$script:TargetDir   = Join-Path $root 'ClientNET'
$script:DataDir     = Join-Path $root 'Data'
$script:HistoryDir  = Join-Path $DataDir 'History'
$script:ManifestPath = Join-Path $DataDir 'manifest.json'
$script:CurrentMetaPath = Join-Path $DataDir 'current.json'
$script:LogoBaseName = 'client-custom-logo-300x86'
$script:AllowedExtensions = @('.bmp','.jpg','.jpeg','.gif','.png','.tif','.tiff')
$script:InvariantCulture = [System.Globalization.CultureInfo]::InvariantCulture
$script:TimestampFormat  = 'yyyy-MM-dd HH:mm:ss'
foreach ($d in @($TargetDir, $DataDir, $HistoryDir)) { New-Item -ItemType Directory -Path $d -Force | Out-Null }

. $extracted

# A 1x1 PNG, so files are real images rather than arbitrary bytes
$pngBytes = [Convert]::FromBase64String('iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8DwHwAFAAH/q842iQAAAABJRU5ErkJggg==')

Write-Host "`n=== 1. Timestamp round trip across cultures (S2 / checklist section 2) ==="
$cultures = @('en-US','de-DE','ja-JP','fi-FI','sv-SE','ar-SA')
$orig = [System.Threading.Thread]::CurrentThread.CurrentCulture
foreach ($c in $cultures) {
    try {
        [System.Threading.Thread]::CurrentThread.CurrentCulture = [System.Globalization.CultureInfo]::new($c)
        $sep = [System.Globalization.CultureInfo]::CurrentCulture.DateTimeFormat.TimeSeparator
        $written = Get-TimestampString
        $back    = ConvertTo-SortableDate $written
        $roundOk = ($back -ne [datetime]::MinValue) -and ($written -match '^\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}$')
        Check "$c (time separator '$sep') writes '$written' and parses back" $roundOk "got '$written' -> $back"

        # And prove the OLD behaviour was genuinely broken under this culture
        $oldStyle = (Get-Date).ToString('yyyy-MM-dd HH:mm:ss')
        $oldParses = $true
        try { $null = [datetime]$oldStyle } catch { $oldParses = $false }
        if ($sep -ne ':') {
            Check "$c : pre-fix format '$oldStyle' would NOT have parsed (bug confirmed real)" (-not $oldParses)
        }
    } finally {
        [System.Threading.Thread]::CurrentThread.CurrentCulture = $orig
    }
}

Write-Host "`n=== 2. Manifest sorts correctly with mixed/legacy timestamp formats ==="
$mixed = @(
    [pscustomobject]@{ Id='a'; StoredFile='a.png'; OriginalName='oldest'; Extension='.png'; DateArchived='2026-01-01 08:00:00' }
    [pscustomobject]@{ Id='b'; StoredFile='b.png'; OriginalName='legacy-dot-separator'; Extension='.png'; DateArchived='2026-06-01 09.30.00' }
    [pscustomobject]@{ Id='c'; StoredFile='c.png'; OriginalName='newest'; Extension='.png'; DateArchived='2026-08-22 14:39:00' }
    [pscustomobject]@{ Id='d'; StoredFile='d.png'; OriginalName='garbage'; Extension='.png'; DateArchived='not a date at all' }
)
Save-Manifest $mixed
$sorted = @(Get-Manifest) | Sort-Object { ConvertTo-SortableDate $_.DateArchived } -Descending
Check "sort did not throw on a legacy '.' separator or on garbage" ($sorted.Count -eq 4)
Check "newest entry sorts first" ($sorted[0].OriginalName -eq 'newest') "got '$($sorted[0].OriginalName)'"
Check "unparseable entry sorts last rather than taking the grid down" ($sorted[-1].OriginalName -eq 'garbage') "got '$($sorted[-1].OriginalName)'"

Write-Host "`n=== 3. Non-Latin filenames survive the manifest round trip (section 1) ==="
$names = @('日本語データ.png','简体中文.png','한국어.png','Данные.png','Ångström café naïve.png','logo[1].png')
$entries = @()
foreach ($n in $names) {
    $entries += [pscustomobject]@{ Id=[guid]::NewGuid().ToString(); StoredFile="stored-$n"; OriginalName=$n; Extension='.png'; DateArchived=(Get-TimestampString) }
}
Save-Manifest $entries
$read = @(Get-Manifest)
foreach ($n in $names) {
    Check "'$n' survived write -> read" (@($read | Where-Object { $_.OriginalName -eq $n }).Count -eq 1)
}
$rawBytes = [IO.File]::ReadAllBytes($ManifestPath)
Check "manifest.json is valid UTF-8 on disk (no mojibake)" ([Text.Encoding]::UTF8.GetString($rawBytes).Contains('日本語データ.png'))

Write-Host "`n=== 3b. C6 -- clearing the manifest actually clears it ==="
Save-Manifest @([pscustomobject]@{ Id='only'; StoredFile='only.png'; OriginalName='only.png'; Extension='.png'; DateArchived=(Get-TimestampString) })
Check "one entry saved" ((Get-Manifest).Count -eq 1) "count=$((Get-Manifest).Count)"
Save-Manifest @()
Check "Save-Manifest @() empties the file (was a phantom-row bug)" ((Get-Manifest).Count -eq 0) "count=$((Get-Manifest).Count)"
Check "manifest.json holds a JSON array, not stale content" (((Get-Content -LiteralPath $ManifestPath -Raw).Trim()) -eq '[]')
Save-Manifest @([pscustomobject]@{ Id='s'; StoredFile='s.png'; OriginalName='single.png'; Extension='.png'; DateArchived=(Get-TimestampString) })
Check "a single entry serialises as a JSON array" (((Get-Content -LiteralPath $ManifestPath -Raw).Trim()).StartsWith('['))

Write-Host "`n=== 4. Wildcard metacharacters in filenames (S1 / the -LiteralPath fix) ==="
Save-Manifest @()
Get-ChildItem -LiteralPath $HistoryDir -File | Remove-Item -Force
Remove-Item -LiteralPath $CurrentMetaPath -Force -ErrorAction SilentlyContinue
$bracketSrc = Join-Path $root 'logo[1].png'
[IO.File]::WriteAllBytes($bracketSrc, $pngBytes)
Check "fixture 'logo[1].png' exists on disk" ([IO.File]::Exists($bracketSrc))
Check "Test-Path -LiteralPath finds it (bare -Path would not)" (Test-Path -LiteralPath $bracketSrc)
Check "bare -Path demonstrably does NOT find it (bug confirmed real)" (-not (Test-Path -Path $bracketSrc))
$applied = Set-NewLogo -SourcePath $bracketSrc
Check "Set-NewLogo copied 'logo[1].png' into the target folder" (Test-Path -LiteralPath $applied) "returned '$applied'"
$meta = Get-CurrentMeta
Check "current.json recorded the bracketed original name" ($meta.OriginalName -eq 'logo[1].png') "got '$($meta.OriginalName)'"

Write-Host "`n=== 5. Non-Latin source filename through the real write path ==="
$cjkSrc = Join-Path $root '会社ロゴ.png'
[IO.File]::WriteAllBytes($cjkSrc, $pngBytes)
$applied2 = Set-NewLogo -SourcePath $cjkSrc
Check "Set-NewLogo applied a CJK-named source" (Test-Path -LiteralPath $applied2)
$m = @(Get-Manifest)
Check "previous logo was archived (history has 1 entry)" ($m.Count -eq 1) "count=$($m.Count)"
Check "archived entry kept the bracketed original name" ($m[0].OriginalName -eq 'logo[1].png') "got '$($m[0].OriginalName)'"
$archivedFile = Join-Path $HistoryDir $m[0].StoredFile
Check "archived file exists on disk under its sanitised name" (Test-Path -LiteralPath $archivedFile) "expected $archivedFile"
Check "CJK name preserved by the \w sanitiser" ((Get-CurrentMeta).OriginalName -eq '会社ロゴ.png')

Write-Host "`n=== 6. C1 -- setting the live logo as its own source is refused ==="
$live = Get-ExistingLiveLogo
$threw = $false
try { Set-NewLogo -SourcePath $live.FullName } catch { $threw = $true; $msg = $_.Exception.Message }
Check "Set-NewLogo threw instead of destroying the live logo" $threw
Check "the live logo is still present afterwards" ((Get-AllLiveLogos).Count -eq 1) "count=$((Get-AllLiveLogos).Count)"
if ($threw) { Check "error names the cause" ($msg -like '*already the live splash logo*') "msg='$msg'" }

Write-Host "`n=== 7. C2 -- multiple stray live logos are all archived, none lost ==="
[IO.File]::WriteAllBytes((Join-Path $TargetDir "$LogoBaseName.jpg"), $pngBytes)
[IO.File]::WriteAllBytes((Join-Path $TargetDir "$LogoBaseName.bmp"), $pngBytes)
Check "three live logo files now present" ((Get-AllLiveLogos).Count -eq 3) "count=$((Get-AllLiveLogos).Count)"
$before = @(Get-Manifest).Count
$newSrc = Join-Path $root 'replacement.png'
[IO.File]::WriteAllBytes($newSrc, $pngBytes)
$null = Set-NewLogo -SourcePath $newSrc
Check "exactly one live logo remains after Set" ((Get-AllLiveLogos).Count -eq 1) "count=$((Get-AllLiveLogos).Count)"
Check "all three strays were archived (manifest grew by 3)" ((@(Get-Manifest).Count - $before) -eq 3) "grew by $((@(Get-Manifest).Count - $before))"
$distinct = @(Get-Manifest | Select-Object -ExpandProperty StoredFile | Sort-Object -Unique).Count
Check "no archived file overwrote another (all StoredFile names unique)" ($distinct -eq @(Get-Manifest).Count)

Write-Host "`n=== 8. Restore round trip ==="
$hist = @(Get-Manifest) | Sort-Object { ConvertTo-SortableDate $_.DateArchived } -Descending | Select-Object -First 1
$restored = Restore-FromHistoryEntry $hist
Check "Restore produced a live logo" (Test-Path -LiteralPath $restored)
Check "Restore recorded the original name" ((Get-CurrentMeta).OriginalName -eq $hist.OriginalName)
$cnt = @(Get-Manifest).Count
Remove-HistoryEntry $hist
Check "Remove-HistoryEntry dropped exactly one manifest entry" ((@(Get-Manifest).Count) -eq ($cnt - 1))
Check "Remove-HistoryEntry deleted the backing file" (-not (Test-Path -LiteralPath (Join-Path $HistoryDir $hist.StoredFile)))

Remove-Item -LiteralPath $root -Recurse -Force -ErrorAction SilentlyContinue
Write-Host "`n================================"
Write-Host "  PASS: $pass    FAIL: $fail"
Write-Host "================================"
if ($fail -gt 0) { exit 1 }

Remove-Item -LiteralPath $extracted -Force -ErrorAction SilentlyContinue
