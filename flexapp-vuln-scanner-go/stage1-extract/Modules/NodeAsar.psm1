#Requires -Version 7.0

<#
.SYNOPSIS
    Identity resolution for Node/Electron packages, including inside app.asar.
.DESCRIPTION
    Plain package.json files are trivial JSON reads. app.asar (Electron's
    packed-archive format) needs a small from-scratch binary reader, since
    there's no .NET library for it: the header is Chromium's nested "Pickle"
    framing - a 4-byte wrapper, a header-size field, an inner JSON-pickle
    size field, a JSON string length field, then the UTF-8 JSON directory
    listing itself. Byte layout was confirmed empirically against a real
    file produced by the reference `asar` npm package during development:

      offset 0-3:   constant wrapper size (always 4, ignored)
      offset 4-7:   H = header block size (LE uint32)
      offset 8+H:   start of the concatenated file-data section
      within the H-byte header block:
        +0..3: J = inner pickle payload size (ignored - informational)
        +4..7: L = JSON string byte length (LE uint32)
        +8..(8+L): UTF-8 JSON text (the directory listing)

    Only locates and parses package.json entries inside the archive - this
    does not extract arbitrary files, since identity resolution is all this
    project needs from asar.

    Electron's bundled Chromium/Node versions are NOT resolved here - that's
    a string-signature scan (see StringSignatures.psm1, method
    'electron-embedded'), run against the main executable, not the archive.
#>

function ConvertTo-NullableString {
    # package.json is supposed to use string name/version fields, but real
    # packages (found via a live test: some of OBS Studio's internal
    # plugin package.json files) use a bare numeric version like
    # `"version": 9` instead of `"9"`. ConvertFrom-Json happily returns that
    # as a PowerShell int, which then violates the inventory schema's
    # "version is string|null" contract. Cast explicitly rather than assume
    # JSON-typed-as-expected; guard $null separately since [string]$null
    # becomes "" in PowerShell, not $null.
    param($Value)
    if ($null -eq $Value) { return $null }
    return [string]$Value
}

function Get-NodePackageIdentity {
    [CmdletBinding()]
    param(
        [Parameter(Mandatory)]
        [string]$Path
    )

    try {
        $json = Get-Content -LiteralPath $Path -Raw -Encoding UTF8 | ConvertFrom-Json -ErrorAction Stop
    }
    catch {
        return $null
    }

    if (-not $json.name -and -not $json.version) { return $null }

    $name = ConvertTo-NullableString -Value $json.name
    $version = ConvertTo-NullableString -Value $json.version

    [PSCustomObject]@{
        method  = 'node-package-json'
        vendor  = $null
        product = $name
        version = $version
        raw     = @{ name = $name; version = $version; sourceFile = [System.IO.Path]::GetFileName($Path) }
    }
}

function Read-AsarHeader {
    param([Parameter(Mandatory)][string]$AsarPath)

    $stream = [System.IO.File]::OpenRead($AsarPath)
    try {
        $reader = [System.IO.BinaryReader]::new($stream)
        [void]$reader.ReadUInt32()                       # outer pickle wrapper size - not needed
        $headerSize = $reader.ReadUInt32()
        $headerBuf = $reader.ReadBytes($headerSize)
        if ($headerBuf.Length -lt 8) { throw "asar header block too short" }

        $jsonLength = [System.BitConverter]::ToUInt32($headerBuf, 4)
        if (8 + $jsonLength -gt $headerBuf.Length) { throw "asar header JSON length out of range" }

        $json = [System.Text.Encoding]::UTF8.GetString($headerBuf, 8, $jsonLength)
        $header = $json | ConvertFrom-Json

        [PSCustomObject]@{
            Header     = $header
            DataOffset = [int64]8 + [int64]$headerSize
        }
    }
    finally {
        $stream.Dispose()
    }
}

function Get-AsarFileEntries {
    param(
        [Parameter(Mandatory)]
        $HeaderNode,

        [string]$PathPrefix = '',

        [Parameter(Mandatory)]
        [int64]$DataOffset
    )

    $entries = [System.Collections.Generic.List[object]]::new()
    if (-not $HeaderNode.files) { return $entries }

    foreach ($prop in $HeaderNode.files.PSObject.Properties) {
        $childPath = if ($PathPrefix) { "$PathPrefix/$($prop.Name)" } else { $prop.Name }
        $child = $prop.Value

        if ($child.PSObject.Properties.Match('files').Count -gt 0 -and $child.files) {
            foreach ($e in (Get-AsarFileEntries -HeaderNode $child -PathPrefix $childPath -DataOffset $DataOffset)) {
                $entries.Add($e) | Out-Null
            }
        }
        elseif ($null -ne $child.offset) {
            $entries.Add([PSCustomObject]@{
                Path           = $childPath
                Size           = [int64]$child.size
                AbsoluteOffset = $DataOffset + [int64]$child.offset
            }) | Out-Null
        }
        # else: unpacked/symlink entry with no offset - not a regular packed file, skip.
    }

    return $entries
}

function Get-AsarPackageIdentities {
    [CmdletBinding()]
    param(
        [Parameter(Mandatory)]
        [string]$AsarPath
    )

    $results = [System.Collections.Generic.List[object]]::new()

    try {
        $headerInfo = Read-AsarHeader -AsarPath $AsarPath
    }
    catch {
        return $results
    }

    $entries = Get-AsarFileEntries -HeaderNode $headerInfo.Header -DataOffset $headerInfo.DataOffset
    $packageJsonEntries = $entries | Where-Object { [System.IO.Path]::GetFileName($_.Path) -eq 'package.json' }
    $archiveLabel = [System.IO.Path]::GetFileName($AsarPath)

    foreach ($entry in $packageJsonEntries) {
        try {
            $stream = [System.IO.File]::OpenRead($AsarPath)
            try {
                [void]$stream.Seek($entry.AbsoluteOffset, [System.IO.SeekOrigin]::Begin)
                $buffer = New-Object byte[] $entry.Size
                $totalRead = 0
                while ($totalRead -lt $entry.Size) {
                    $read = $stream.Read($buffer, $totalRead, $entry.Size - $totalRead)
                    if ($read -le 0) { break }
                    $totalRead += $read
                }
            }
            finally {
                $stream.Dispose()
            }

            $json = [System.Text.Encoding]::UTF8.GetString($buffer, 0, $totalRead) | ConvertFrom-Json -ErrorAction Stop
            if (-not $json.name -and -not $json.version) { continue }

            $name = ConvertTo-NullableString -Value $json.name
            $version = ConvertTo-NullableString -Value $json.version

            $results.Add([PSCustomObject]@{
                entryPath = "$archiveLabel!/$($entry.Path)"
                sizeBytes = $entry.Size
                identity  = [PSCustomObject]@{
                    method  = 'node-package-json'
                    vendor  = $null
                    product = $name
                    version = $version
                    raw     = @{ name = $name; version = $version; asarPath = $entry.Path }
                }
            }) | Out-Null
        }
        catch {
            # Corrupt or unreadable entry - skip it, don't fail the whole archive.
        }
    }

    return $results
}

Export-ModuleMember -Function Get-NodePackageIdentity, Get-AsarPackageIdentities
