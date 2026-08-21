#Requires -Version 7.0

<#
.SYNOPSIS
    Parses a FlexApp .package.xml sidecar into the flexAppXml inventory block.
.DESCRIPTION
    Schema confirmed from two real samples (winscp, OBS-Studio) - see
    PLAN.md's resolved assumption 2 for the full rationale behind each field
    below, including why DisplayName and VersionMajor/Minor/Build/Revision
    are treated as unreliable rather than authoritative.

    Deliberately DOES NOT read or return <Icon>, <license>, or <CallToHome>.
    <license> carries a named individual's email/phone (Liquidware's own
    FlexApp One product-license contact, unrelated to the packaged app) and
    a signature/serial - this must never reach the inventory JSON, since the
    whole point of Stage 1 is that nothing but that JSON leaves the machine.
    <Icon> is dropped because it's a large base64 blob irrelevant to
    component resolution.
#>

function Read-FlexAppPackageXml {
    [CmdletBinding()]
    param(
        [Parameter(Mandatory)]
        [string]$XmlPath
    )

    if (-not (Test-Path -LiteralPath $XmlPath)) {
        return $null
    }

    try {
        [xml]$xml = Get-Content -LiteralPath $XmlPath -Raw -Encoding UTF8
    }
    catch {
        Write-Warning "Failed to parse package XML '$XmlPath': $_"
        return $null
    }

    $pkg = $xml.Package
    if (-not $pkg) {
        Write-Warning "Package XML '$XmlPath' has no <Package> root element."
        return $null
    }

    $historyRaw = @()
    if ($pkg.History -and $pkg.History.string) {
        $historyRaw = @($pkg.History.string)
    }

    $shortcutTargets = @()
    if ($pkg.Links -and $pkg.Links.Link) {
        $shortcutTargets = @($pkg.Links.Link |
            ForEach-Object { $_.Target } |
            Where-Object { $_ })
    }

    $installerIds = @()
    if ($pkg.Installers -and $pkg.Installers.string) {
        foreach ($installerCommand in @($pkg.Installers.string)) {
            # winget-style "--id <PackageId>"; extend this pattern set as other
            # package managers (choco, msiexec /i, etc.) turn up in real samples.
            if ($installerCommand -match '--id\s+([\w.\-]+)') {
                $installerIds += $Matches[1]
            }
        }
    }

    $versionFields = $null
    $major = 0; $minor = 0; $build = 0; $revision = 0
    [void][int]::TryParse($pkg.VersionMajor, [ref]$major)
    [void][int]::TryParse($pkg.VersionMinor, [ref]$minor)
    [void][int]::TryParse($pkg.VersionBuild, [ref]$build)
    [void][int]::TryParse($pkg.VersionRevision, [ref]$revision)
    if ($major -ne 0 -or $minor -ne 0 -or $build -ne 0 -or $revision -ne 0) {
        $versionFields = "$major.$minor.$build.$revision"
    }

    $sizeInGb = $null
    [double]$sizeInGbParsed = 0
    if ([double]::TryParse($pkg.SizeInGb, [ref]$sizeInGbParsed)) { $sizeInGb = $sizeInGbParsed }

    $actualSizeInBytes = $null
    [int64]$actualSizeParsed = 0
    if ([int64]::TryParse($pkg.ActualSizeInBytes, [ref]$actualSizeParsed)) { $actualSizeInBytes = $actualSizeParsed }

    [PSCustomObject]@{
        uuid                           = $pkg.Uuid
        displayName                    = $pkg.DisplayName
        packageType                    = $pkg.PackageType
        sizeInGb                       = $sizeInGb
        actualSizeInBytes              = $actualSizeInBytes
        dateCreated                    = $pkg.DateCreated
        dateModified                   = $pkg.DateModified
        historyRaw                     = $historyRaw
        versionMajorMinorBuildRevision = $versionFields
        shortcutTargets                = $shortcutTargets
        installerIds                   = $installerIds
    }
}
