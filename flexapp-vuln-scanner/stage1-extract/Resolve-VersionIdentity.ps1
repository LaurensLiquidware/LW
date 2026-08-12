#Requires -Version 7.0

<#
.SYNOPSIS
    Exclusion filtering and identity resolution dispatch.
.DESCRIPTION
    Test-FileExclusion applies ExclusionRules.psd1's transparent, inspectable
    rules - run BEFORE identity resolution so exclusion decides what's even
    worth spending a string-signature scan on (see StringSignatures.psm1's
    scan cost note).

    Resolve-ComponentIdentity dispatches by extension/filename in the
    priority order from PLAN.md: PE/.NET, then Java, then Node/Electron,
    then Python, then string-signature scanning as a last resort. Jar and
    asar containers can yield MORE than one component from a single physical
    file (fat jars, Electron packages) - those come back as ExtraComponents,
    synthetic file-like records using the "<real path>!/<inner path>"
    convention, which the caller appends to the files[] array alongside the
    physical file's own record.
#>

Import-Module (Join-Path $PSScriptRoot 'Modules/VersionResources.psm1') -Force
Import-Module (Join-Path $PSScriptRoot 'Modules/JavaManifest.psm1') -Force
Import-Module (Join-Path $PSScriptRoot 'Modules/NodeAsar.psm1') -Force
Import-Module (Join-Path $PSScriptRoot 'Modules/PythonDist.psm1') -Force
Import-Module (Join-Path $PSScriptRoot 'Modules/StringSignatures.psm1') -Force

function Test-FileExclusion {
    [CmdletBinding()]
    param(
        [Parameter(Mandatory)]
        [string]$RelativePath,

        [Parameter(Mandatory)]
        [hashtable]$Rules
    )

    $normalized = $RelativePath -replace '/', '\'
    $normalizedForRegex = $RelativePath -replace '\\', '/'

    foreach ($rule in $Rules.PathContainsRules) {
        foreach ($needle in $rule.Contains) {
            if ($normalized -like "*$needle*") {
                return [PSCustomObject]@{ Excluded = $true; Reason = $rule.Reason }
            }
        }
    }

    $fileName = Split-Path -Path $normalized -Leaf
    foreach ($rule in $Rules.NamePatternRules) {
        foreach ($pattern in $rule.Patterns) {
            if ($fileName -like $pattern) {
                return [PSCustomObject]@{ Excluded = $true; Reason = $rule.Reason }
            }
        }
    }

    $ext = [System.IO.Path]::GetExtension($fileName).ToLowerInvariant()
    foreach ($rule in $Rules.ExtensionRules) {
        if ($rule.Extensions -contains $ext) {
            return [PSCustomObject]@{ Excluded = $true; Reason = $rule.Reason }
        }
    }

    if ($Rules.CultureFolderRegex -and $normalizedForRegex -match $Rules.CultureFolderRegex) {
        return [PSCustomObject]@{ Excluded = $true; Reason = 'satellite-culture-resources' }
    }

    return [PSCustomObject]@{ Excluded = $false; Reason = $null }
}

function New-SyntheticFileRecord {
    param(
        [string]$RelativePath,
        [Nullable[int64]]$SizeBytes,
        $Sha256, # deliberately untyped - a [string] type constraint here silently coerces $null to "" in PowerShell's parameter binding
        [string]$ComponentType,
        [object]$Identity
    )

    [PSCustomObject]@{
        relativePath    = $RelativePath
        sizeBytes       = $SizeBytes
        sha256          = $Sha256
        excluded        = $false
        exclusionReason = $null
        componentType   = $ComponentType
        identity        = $Identity
        readError       = $null
    }
}

function Resolve-ComponentIdentity {
    <#
    .OUTPUTS
        [PSCustomObject]@{ Identity = <identity or $null for this physical file>; ExtraComponents = <array of synthetic file records> }
    #>
    [CmdletBinding()]
    param(
        [Parameter(Mandatory)]
        [string]$AbsolutePath,

        [Parameter(Mandatory)]
        [string]$RelativePath,

        [Parameter(Mandatory)]
        [string]$ComponentType,

        [Parameter(Mandatory)]
        [array]$StringSignatures,

        [int]$JarMaxDepth = 5
    )

    $leafName = [System.IO.Path]::GetFileName($AbsolutePath)
    $ext = [System.IO.Path]::GetExtension($AbsolutePath).ToLowerInvariant()

    # Nested entries from Get-JarIdentity/Get-AsarPackageIdentities are
    # labeled starting from the container's own leaf name (they have no
    # knowledge of where that file actually lives in the volume) - strip
    # that leaf-name prefix and graft on the real relative path instead.
    function ConvertTo-RootedRelativePath {
        param([string]$EntryPath)
        $suffix = $EntryPath.Substring($leafName.Length)
        return "$RelativePath$suffix"
    }

    switch -Regex ($ext) {
        '^\.(exe|dll|sys|ocx)$' {
            $identity = Get-DotNetAssemblyIdentity -Path $AbsolutePath
            if (-not $identity) { $identity = Get-PEVersionResourceIdentity -Path $AbsolutePath }
            if (-not $identity) { $identity = Get-StringSignatureIdentity -Path $AbsolutePath -Signatures $StringSignatures }
            return [PSCustomObject]@{ Identity = $identity; ExtraComponents = @() }
        }
        '^\.(jar|war)$' {
            $jarResult = Get-JarIdentity -JarPath $AbsolutePath -MaxDepth $JarMaxDepth
            $extra = @($jarResult.NestedComponents | ForEach-Object {
                New-SyntheticFileRecord `
                    -RelativePath (ConvertTo-RootedRelativePath -EntryPath $_.entryPath) `
                    -SizeBytes $_.sizeBytes `
                    -Sha256 $_.sha256 `
                    -ComponentType 'jar' `
                    -Identity $_.identity
            })
            return [PSCustomObject]@{ Identity = $jarResult.ContainerIdentity; ExtraComponents = $extra }
        }
    }

    if ($leafName -ieq 'package.json') {
        return [PSCustomObject]@{ Identity = (Get-NodePackageIdentity -Path $AbsolutePath); ExtraComponents = @() }
    }

    if ($ext -eq '.asar') {
        $asarResults = Get-AsarPackageIdentities -AsarPath $AbsolutePath
        $extra = @($asarResults | ForEach-Object {
            New-SyntheticFileRecord `
                -RelativePath (ConvertTo-RootedRelativePath -EntryPath $_.entryPath) `
                -SizeBytes $_.sizeBytes `
                -Sha256 $null `
                -ComponentType 'node-package' `
                -Identity $_.identity
        })
        return [PSCustomObject]@{ Identity = $null; ExtraComponents = $extra }
    }

    if ($leafName -ieq 'METADATA' -or $leafName -ieq 'PKG-INFO') {
        return [PSCustomObject]@{ Identity = (Get-PythonDistIdentity -Path $AbsolutePath); ExtraComponents = @() }
    }

    if ($ComponentType -eq 'unknown') {
        $identity = Get-StringSignatureIdentity -Path $AbsolutePath -Signatures $StringSignatures
        return [PSCustomObject]@{ Identity = $identity; ExtraComponents = @() }
    }

    return [PSCustomObject]@{ Identity = $null; ExtraComponents = @() }
}
