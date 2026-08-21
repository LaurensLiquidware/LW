#Requires -Version 7.0

<#
.SYNOPSIS
    Identity resolution for JAR/WAR files, including nested (fat-jar) components.
.DESCRIPTION
    Priority per PLAN.md: META-INF/maven/*/*/pom.properties is the highest-
    confidence signal available anywhere in this pipeline (clean
    groupId:artifactId:version), checked before MANIFEST.MF. Recurses into
    nested jars (Spring Boot BOOT-INF/lib/*.jar, WAR WEB-INF/lib/*.jar, or
    any *.jar/*.war entry anywhere in the archive) up to a depth limit, since
    a single physical .jar/.war file can bundle many distinct components.

    A shaded/uber jar built by maven-shade-plugin often merges classes but
    still retains each dependency's own META-INF/maven/*/*/pom.properties
    WITHOUT a literal nested .jar file existing - that case is handled by
    enumerating every pom.properties in the archive, not just the container's
    "own" one.
#>

function Read-JavaPropertiesEntry {
    param([System.IO.Compression.ZipArchiveEntry]$Entry)

    $result = @{}
    $stream = $Entry.Open()
    try {
        $reader = [System.IO.StreamReader]::new($stream, [System.Text.Encoding]::UTF8)
        try {
            while ($null -ne ($line = $reader.ReadLine())) {
                $trimmed = $line.Trim()
                if ($trimmed.Length -eq 0 -or $trimmed.StartsWith('#') -or $trimmed.StartsWith('!')) { continue }
                $sepIndex = $trimmed.IndexOfAny(@('=', ':'))
                if ($sepIndex -lt 0) { continue }
                $key = $trimmed.Substring(0, $sepIndex).Trim()
                $value = $trimmed.Substring($sepIndex + 1).Trim()
                $result[$key] = $value
            }
        }
        finally { $reader.Dispose() }
    }
    finally { $stream.Dispose() }
    return $result
}

function Read-JavaManifestEntry {
    param([System.IO.Compression.ZipArchiveEntry]$Entry)

    $result = @{}
    $lastKey = $null
    $stream = $Entry.Open()
    try {
        $reader = [System.IO.StreamReader]::new($stream, [System.Text.Encoding]::UTF8)
        try {
            while ($null -ne ($line = $reader.ReadLine())) {
                if ($line.Length -eq 0) { $lastKey = $null; continue }
                if ($line.StartsWith(' ') -and $lastKey) {
                    # MANIFEST.MF continuation line (word-wrap): append without the leading space.
                    $result[$lastKey] += $line.Substring(1)
                    continue
                }
                if ($line -match '^([\w.-]+):\s?(.*)$') {
                    $lastKey = $Matches[1]
                    $result[$lastKey] = $Matches[2]
                }
            }
        }
        finally { $reader.Dispose() }
    }
    finally { $stream.Dispose() }
    return $result
}

function Get-JarComponentsFromStream {
    param(
        [System.IO.Stream]$Stream,
        [string]$Label,
        [int]$Depth,
        [int]$MaxDepth
    )

    $results = [System.Collections.Generic.List[object]]::new()

    try {
        $zip = [System.IO.Compression.ZipArchive]::new($Stream, [System.IO.Compression.ZipArchiveMode]::Read, $true)
    }
    catch {
        return $results
    }

    try {
        $pomEntries = @($zip.Entries | Where-Object { $_.FullName -match '^META-INF/maven/[^/]+/[^/]+/pom\.properties$' })
        $ownIdentitySet = $false

        foreach ($pomEntry in $pomEntries) {
            $props = Read-JavaPropertiesEntry -Entry $pomEntry
            if ($props.ContainsKey('groupId') -and $props.ContainsKey('artifactId') -and $props.ContainsKey('version')) {
                $identity = [PSCustomObject]@{
                    method  = 'jar-pom-properties'
                    vendor  = $props['groupId']
                    product = $props['artifactId']
                    version = $props['version']
                    raw     = @{
                        groupId           = $props['groupId']
                        artifactId        = $props['artifactId']
                        version           = $props['version']
                        pomPropertiesPath = $pomEntry.FullName
                    }
                }
                # Exactly one pom.properties in the archive is treated as describing
                # the container itself (the common single-library-jar case). Multiple
                # pom.properties (shaded/uber jar) each describe a bundled dependency
                # instead, none of which is "the container".
                $isContainerIdentity = ($pomEntries.Count -eq 1) -and -not $ownIdentitySet
                if ($isContainerIdentity) { $ownIdentitySet = $true }

                $results.Add([PSCustomObject]@{
                    isContainerIdentity = $isContainerIdentity
                    entryPath           = "$Label!/$($pomEntry.FullName)"
                    identity            = $identity
                }) | Out-Null
            }
        }

        if (-not $ownIdentitySet) {
            $manifestEntry = $zip.GetEntry('META-INF/MANIFEST.MF')
            if ($manifestEntry) {
                $manifest = Read-JavaManifestEntry -Entry $manifestEntry
                $product = if ($manifest['Implementation-Title']) { $manifest['Implementation-Title'] } else { $manifest['Bundle-SymbolicName'] }
                $version = if ($manifest['Implementation-Version']) { $manifest['Implementation-Version'] } else { $manifest['Bundle-Version'] }
                if ($product -or $version) {
                    $results.Add([PSCustomObject]@{
                        isContainerIdentity = $true
                        entryPath           = "$Label!/META-INF/MANIFEST.MF"
                        identity            = [PSCustomObject]@{
                            method  = 'jar-manifest'
                            vendor  = $manifest['Implementation-Vendor']
                            product = $product
                            version = $version
                            raw     = $manifest
                        }
                    }) | Out-Null
                }
            }
        }

        if ($Depth -lt $MaxDepth) {
            $nestedEntries = @($zip.Entries | Where-Object { $_.FullName -match '\.(jar|war)$' })
            foreach ($nestedEntry in $nestedEntries) {
                $nestedLabel = "$Label!/$($nestedEntry.FullName)"
                $ms = [System.IO.MemoryStream]::new()
                try {
                    $entryStream = $nestedEntry.Open()
                    try { $entryStream.CopyTo($ms) } finally { $entryStream.Dispose() }
                    $nestedBytes = $ms.ToArray()
                    $sha256 = [System.Security.Cryptography.SHA256]::Create()
                    try {
                        $nestedHash = [System.BitConverter]::ToString($sha256.ComputeHash($nestedBytes)).Replace('-', '').ToLowerInvariant()
                    }
                    finally { $sha256.Dispose() }

                    $ms.Position = 0
                    $nestedResults = Get-JarComponentsFromStream -Stream $ms -Label $nestedLabel -Depth ($Depth + 1) -MaxDepth $MaxDepth
                    $nestedContainerIdentity = $nestedResults | Where-Object { $_.isContainerIdentity } | Select-Object -First 1

                    $results.Add([PSCustomObject]@{
                        isContainerIdentity = $false
                        entryPath           = $nestedLabel
                        sizeBytes           = $nestedBytes.Length
                        sha256              = $nestedHash
                        identity            = if ($nestedContainerIdentity) { $nestedContainerIdentity.identity } else { $null }
                    }) | Out-Null

                    foreach ($r in $nestedResults) {
                        if (-not $r.isContainerIdentity) { $results.Add($r) | Out-Null }
                    }
                }
                catch {
                    # Corrupt or unreadable nested archive entry - skip it, don't fail the parent jar.
                }
                finally {
                    $ms.Dispose()
                }
            }
        }
    }
    finally {
        $zip.Dispose()
    }

    return $results
}

function Get-JarIdentity {
    <#
    .SYNOPSIS
        Resolves a .jar/.war file's own identity plus any nested components.
    .OUTPUTS
        [PSCustomObject]@{ ContainerIdentity = <identity or $null>; NestedComponents = <array of synthetic file-like records> }
    #>
    [CmdletBinding()]
    param(
        [Parameter(Mandatory)]
        [string]$JarPath,

        [int]$MaxDepth = 5
    )

    try {
        $stream = [System.IO.File]::OpenRead($JarPath)
    }
    catch {
        return [PSCustomObject]@{ ContainerIdentity = $null; NestedComponents = @() }
    }

    try {
        $label = [System.IO.Path]::GetFileName($JarPath)
        $allResults = Get-JarComponentsFromStream -Stream $stream -Label $label -Depth 0 -MaxDepth $MaxDepth
        $containerIdentity = ($allResults | Where-Object { $_.isContainerIdentity } | Select-Object -First 1).identity
        $nestedComponents = @($allResults | Where-Object { -not $_.isContainerIdentity })

        [PSCustomObject]@{
            ContainerIdentity = $containerIdentity
            NestedComponents  = $nestedComponents
        }
    }
    finally {
        $stream.Dispose()
    }
}

Export-ModuleMember -Function Get-JarIdentity
