#Requires -Version 7.0

<#
.SYNOPSIS
    Walks a mounted/extracted FlexApp volume and returns one record per file.
.DESCRIPTION
    Per PLAN.md's resolved scale assumption (100 MB - 15 GB real packages):
    hashing streams a fixed-size buffer rather than fully buffering a file,
    and the walk runs with bounded parallelism rather than serially.

    Per PLAN.md's build order, this pass does NOT resolve version identity
    and does NOT apply the exclusion ruleset yet - both are Stage 1's next
    increment. Every file gets componentType guessed from its extension/name
    only, identity is always null, and excluded is always false here.

    Never crashes on a single bad file (locked, zero-byte, permission
    denied, pathological long path) - failures are recorded in readError
    and the walk continues.
#>

function Get-FileInventoryRecord {
    [CmdletBinding()]
    param(
        [Parameter(Mandatory)]
        [string]$RootPath,

        [int]$ThrottleLimit = [Math]::Max(1, [Environment]::ProcessorCount)
    )

    $rootFull = (Resolve-Path -LiteralPath $RootPath).ProviderPath

    $enumOptions = [System.IO.EnumerationOptions]::new()
    $enumOptions.RecurseSubdirectories = $true
    $enumOptions.IgnoreInaccessible = $true
    $enumOptions.AttributesToSkip = [System.IO.FileAttributes]::ReparsePoint

    try {
        $allFiles = [System.IO.Directory]::EnumerateFiles($rootFull, '*', $enumOptions)
    }
    catch {
        Write-Warning "Failed to enumerate '$rootFull': $_"
        $allFiles = @()
    }

    $rootFullForTrim = $rootFull.TrimEnd('\')

    # Helper functions are redefined inline below because -Parallel runs each
    # iteration in its own runspace, which can't see functions defined in the
    # caller's scope.
    $allFiles | ForEach-Object -ThrottleLimit $ThrottleLimit -Parallel {
        $fullPath = $_
        $rootFullForTrim = $using:rootFullForTrim

        function Get-ComponentTypeGuessLocal {
            param([string]$FileName)
            $ext = [System.IO.Path]::GetExtension($FileName)
            switch -Regex ($ext) {
                '^\.(exe|dll|sys|ocx)$' { return 'pe-native' }
                '^\.(jar|war)$'         { return 'jar' }
            }
            if ($FileName -ieq 'package.json') { return 'node-package' }
            if ($FileName -ieq 'MANIFEST.MF' -or $FileName -ieq 'pom.properties') { return 'jar' }
            if ($FileName -ieq 'METADATA' -or $FileName -ieq 'PKG-INFO') { return 'python-dist' }
            if ($FileName -like '*.deps.json') { return 'dotnet-package' }
            return 'unknown'
        }

        function Get-LongPathSafeLocal {
            param([string]$Path)
            if ($Path.Length -ge 260 -and -not $Path.StartsWith('\\?\')) {
                if ($Path.StartsWith('\\')) { return '\\?\UNC\' + $Path.Substring(2) }
                return '\\?\' + $Path
            }
            return $Path
        }

        $relativePath = $fullPath
        if ($fullPath.StartsWith($rootFullForTrim)) {
            $relativePath = $fullPath.Substring($rootFullForTrim.Length).TrimStart('\')
        }

        $fileName = [System.IO.Path]::GetFileName($fullPath)
        $componentType = Get-ComponentTypeGuessLocal -FileName $fileName

        $sizeBytes = $null
        $sha256 = $null
        $readError = $null

        try {
            $safePath = Get-LongPathSafeLocal -Path $fullPath
            $info = [System.IO.FileInfo]::new($safePath)
            $sizeBytes = $info.Length

            $stream = [System.IO.File]::Open($safePath, [System.IO.FileMode]::Open, [System.IO.FileAccess]::Read, [System.IO.FileShare]::ReadWrite)
            try {
                $hasher = [System.Security.Cryptography.IncrementalHash]::CreateHash([System.Security.Cryptography.HashAlgorithmName]::SHA256)
                try {
                    $buffer = New-Object byte[] (1MB)
                    while (($bytesRead = $stream.Read($buffer, 0, $buffer.Length)) -gt 0) {
                        $hasher.AppendData($buffer, 0, $bytesRead)
                    }
                    $hashBytes = $hasher.GetHashAndReset()
                    $sha256 = [System.BitConverter]::ToString($hashBytes).Replace('-', '').ToLowerInvariant()
                }
                finally {
                    $hasher.Dispose()
                }
            }
            finally {
                $stream.Dispose()
            }
        }
        catch {
            $readError = $_.Exception.Message
        }

        [PSCustomObject]@{
            relativePath   = $relativePath
            sizeBytes      = $sizeBytes
            sha256         = $sha256
            excluded       = $false
            exclusionReason = $null
            componentType  = $componentType
            identity       = $null
            readError      = $readError
        }
    }
}
