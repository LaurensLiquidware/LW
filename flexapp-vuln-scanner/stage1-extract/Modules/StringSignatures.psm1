#Requires -Version 7.0

<#
.SYNOPSIS
    Last-resort identity resolution via known banner-string patterns.
.DESCRIPTION
    Only meaningful for files that survived exclusion but weren't resolved
    by any higher-confidence method (PE, .NET, jar, node, python). Scans up
    to -MaxScanBytes of the file's raw content, decoded as Latin-1 so every
    byte maps to exactly one char with no decode exceptions and no offset
    drift versus the original bytes.
#>

function Import-StringSignatures {
    [CmdletBinding()]
    param(
        [Parameter(Mandatory)]
        [string]$ConfigPath
    )

    $data = Import-PowerShellDataFile -LiteralPath $ConfigPath
    return @($data.Signatures)
}

function Get-StringSignatureIdentity {
    [CmdletBinding()]
    param(
        [Parameter(Mandatory)]
        [string]$Path,

        [Parameter(Mandatory)]
        [array]$Signatures,

        [long]$MaxScanBytes = 50MB
    )

    try {
        $info = [System.IO.FileInfo]::new($Path)
        $bytesToRead = [Math]::Min($info.Length, $MaxScanBytes)
        if ($bytesToRead -le 0) { return $null }

        $buffer = New-Object byte[] $bytesToRead
        $stream = [System.IO.File]::Open($Path, [System.IO.FileMode]::Open, [System.IO.FileAccess]::Read, [System.IO.FileShare]::ReadWrite)
        try {
            $totalRead = 0
            while ($totalRead -lt $bytesToRead) {
                $read = $stream.Read($buffer, $totalRead, $bytesToRead - $totalRead)
                if ($read -le 0) { break }
                $totalRead += $read
            }
        }
        finally {
            $stream.Dispose()
        }

        $text = [System.Text.Encoding]::Latin1.GetString($buffer, 0, $totalRead)
    }
    catch {
        return $null
    }

    foreach ($signature in $Signatures) {
        $match = [regex]::Match($text, $signature.Pattern)
        if ($match.Success) {
            $version = $match.Groups[[int]$signature.VersionGroup].Value
            $method = if ($signature.Method) { $signature.Method } else { 'string-signature' }
            return [PSCustomObject]@{
                method  = $method
                vendor  = $null
                product = $signature.Name
                version = $version
                raw     = @{
                    signatureName = $signature.Name
                    matchedText   = $match.Value
                }
            }
        }
    }

    return $null
}

Export-ModuleMember -Function Import-StringSignatures, Get-StringSignatureIdentity
