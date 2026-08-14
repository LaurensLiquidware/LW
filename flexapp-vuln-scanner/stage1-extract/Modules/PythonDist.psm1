#Requires -Version 7.0

<#
.SYNOPSIS
    Identity resolution for Python distributions via dist-info/egg-info metadata.
.DESCRIPTION
    Parses the RFC 822-style header block at the top of a *.dist-info/METADATA
    or *.egg-info/PKG-INFO file. Only Name/Version are needed for identity;
    the rest of the file is a free-text long description and is ignored.
#>

function Get-PythonDistIdentity {
    [CmdletBinding()]
    param(
        [Parameter(Mandatory)]
        [string]$Path
    )

    try {
        $reader = [System.IO.StreamReader]::new($Path, [System.Text.Encoding]::UTF8)
    }
    catch {
        return $null
    }

    try {
        $name = $null
        $version = $null

        while ($null -ne ($line = $reader.ReadLine())) {
            # The long description follows a blank line and can contain
            # arbitrary text that looks like headers - stop once we hit it.
            if ($line.Length -eq 0) { break }

            if (-not $name -and $line -match '^Name:\s*(.+)$') { $name = $Matches[1].Trim() }
            elseif (-not $version -and $line -match '^Version:\s*(.+)$') { $version = $Matches[1].Trim() }

            if ($name -and $version) { break }
        }

        if (-not $name -and -not $version) { return $null }

        [PSCustomObject]@{
            method  = 'python-dist-info'
            vendor  = $null
            product = $name
            version = $version
            raw     = @{ name = $name; version = $version; sourceFile = [System.IO.Path]::GetFileName($Path) }
        }
    }
    finally {
        $reader.Dispose()
    }
}

Export-ModuleMember -Function Get-PythonDistIdentity
