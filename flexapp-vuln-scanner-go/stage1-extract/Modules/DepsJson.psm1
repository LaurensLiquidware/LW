#Requires -Version 7.0

<#
.SYNOPSIS
    Identity resolution for .NET Core/5+ dependency lockfiles (*.deps.json).
.DESCRIPTION
    A deps.json's `.libraries` section lists every NuGet package name and
    exact resolved version the app was built against, keyed
    "<PackageName>/<Version>" - the same "highest confidence available"
    signal jar-pom-properties already provides for Java fat jars. Skips the
    "project" entry (the app/referenced project itself, not a NuGet
    dependency); everything else ("package", "runtimepack", "reference", ...)
    is a real resolved dependency worth an identity.

    A single deps.json can name dozens of dependencies, some of which
    (IL-linked/trimmed) never ship as an actual DLL - this is often the ONLY
    identity signal available for those. Returns one synthetic component per
    library entry, same shape as Get-JarIdentity/Get-AsarPackageIdentities'
    nested-component results.
#>

function Get-DepsJsonIdentities {
    [CmdletBinding()]
    param(
        [Parameter(Mandatory)]
        [string]$Path
    )

    $results = [System.Collections.Generic.List[object]]::new()

    try {
        $json = Get-Content -LiteralPath $Path -Raw -Encoding UTF8 | ConvertFrom-Json -ErrorAction Stop
    }
    catch {
        return $results
    }

    if (-not $json.libraries) { return $results }

    $sourceFile = [System.IO.Path]::GetFileName($Path)

    foreach ($prop in $json.libraries.PSObject.Properties) {
        $entry = $prop.Value
        if ($entry.type -eq 'project') { continue }

        $key = $prop.Name
        $slash = $key.LastIndexOf('/')
        if ($slash -lt 0) { continue }

        $name = $key.Substring(0, $slash)
        $version = $key.Substring($slash + 1)
        if (-not $name -or -not $version) { continue }

        $results.Add([PSCustomObject]@{
            entryPath = "$sourceFile!/$key"
            identity  = [PSCustomObject]@{
                method  = 'dotnet-deps-json'
                vendor  = $null
                product = $name
                version = $version
                raw     = @{ name = $name; version = $version; type = $entry.type; sourceFile = $sourceFile }
            }
        }) | Out-Null
    }

    return $results
}

Export-ModuleMember -Function Get-DepsJsonIdentities
