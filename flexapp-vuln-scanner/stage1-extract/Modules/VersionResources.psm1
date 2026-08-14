#Requires -Version 7.0

<#
.SYNOPSIS
    Identity resolution for native PE binaries and .NET assemblies.
.DESCRIPTION
    Priority order per PLAN.md: a managed (.NET) assembly is checked first,
    since its assembly manifest is a more precisely-defined version signal
    than the Win32 resource compilers also embed alongside it (the two
    frequently disagree - AssemblyVersion vs. AssemblyFileVersion). Falling
    that, the Win32 version resource is used for native binaries.

    Reads metadata only - never loads or executes the target file. PE/COFF
    and assembly-manifest parsing here is via System.Reflection.Metadata's
    PEReader/MetadataReader, which is pure managed code and available
    cross-platform (not a Win32-only API), even though the files themselves
    are Windows binaries.
#>

function Get-DotNetAssemblyIdentity {
    [CmdletBinding()]
    param(
        [Parameter(Mandatory)]
        [string]$Path
    )

    try {
        $stream = [System.IO.File]::OpenRead($Path)
        try {
            $peReader = [System.Reflection.PortableExecutable.PEReader]::new($stream)
            try {
                if (-not $peReader.HasMetadata) {
                    return $null
                }

                # GetMetadataReader() is an extension method on the static class
                # System.Reflection.Metadata.PEReaderExtensions, not an instance
                # method on PEReader itself - PowerShell's method binder does not
                # resolve extension methods via instance dot-syntax the way the C#
                # compiler does, so calling $peReader.GetMetadataReader() throws
                # "does not contain a method named 'GetMetadataReader'" for every
                # real assembly (confirmed on a live Paint.NET test: 311/311
                # managed .NET files silently fell back to pe-version-resource).
                # Must invoke it as a static call instead.
                $metadataReader = [System.Reflection.Metadata.PEReaderExtensions]::GetMetadataReader($peReader)
                $assemblyDef = $metadataReader.GetAssemblyDefinition()

                $name = $metadataReader.GetString($assemblyDef.Name)
                $version = $assemblyDef.Version.ToString()

                $publicKeyToken = $null
                if (-not $assemblyDef.PublicKey.IsNil) {
                    $publicKeyBytes = $metadataReader.GetBlobBytes($assemblyDef.PublicKey)
                    if ($publicKeyBytes.Length -gt 0) {
                        $sha1 = [System.Security.Cryptography.SHA1]::Create()
                        try {
                            $hash = $sha1.ComputeHash($publicKeyBytes)
                            # Public key token is the last 8 bytes of the SHA-1 hash, reversed.
                            $tokenBytes = $hash[($hash.Length - 8)..($hash.Length - 1)]
                            [array]::Reverse($tokenBytes)
                            $publicKeyToken = [System.BitConverter]::ToString($tokenBytes).Replace('-', '').ToLowerInvariant()
                        }
                        finally {
                            $sha1.Dispose()
                        }
                    }
                }

                # Also grab the Win32 file-version resource, if present, purely as a
                # cross-check - AssemblyVersion and FileVersion often disagree, and
                # that disagreement is itself worth recording, not hiding.
                $fileVersionFromResource = $null
                try {
                    $fvi = [System.Diagnostics.FileVersionInfo]::GetVersionInfo($Path)
                    if ($fvi.FileVersion) { $fileVersionFromResource = $fvi.FileVersion }
                }
                catch { }

                return [PSCustomObject]@{
                    method  = 'dotnet-manifest'
                    vendor  = $null
                    product = $name
                    version = $version
                    raw     = @{
                        assemblyName            = $name
                        assemblyVersion         = $version
                        publicKeyToken          = $publicKeyToken
                        fileVersionFromResource = $fileVersionFromResource
                    }
                }
            }
            finally {
                $peReader.Dispose()
            }
        }
        finally {
            $stream.Dispose()
        }
    }
    catch {
        # Not a PE file, no metadata, or some other read failure - not a
        # managed assembly as far as this function is concerned.
        return $null
    }
}

function Get-PEVersionResourceIdentity {
    [CmdletBinding()]
    param(
        [Parameter(Mandatory)]
        [string]$Path
    )

    try {
        $fvi = [System.Diagnostics.FileVersionInfo]::GetVersionInfo($Path)
    }
    catch {
        return $null
    }

    if (-not $fvi.ProductName -and -not $fvi.FileDescription -and -not $fvi.ProductVersion -and -not $fvi.FileVersion) {
        # No version resource at all - common for stripped/vendored binaries;
        # this is exactly the case string-signature scanning exists for.
        return $null
    }

    $version = if ($fvi.ProductVersion) { $fvi.ProductVersion } else { $fvi.FileVersion }

    [PSCustomObject]@{
        method  = 'pe-version-resource'
        vendor  = $fvi.CompanyName
        product = $fvi.ProductName
        version = $version
        raw     = @{
            companyName      = $fvi.CompanyName
            productName      = $fvi.ProductName
            productVersion   = $fvi.ProductVersion
            fileVersion      = $fvi.FileVersion
            originalFilename = $fvi.OriginalFilename
            fileDescription  = $fvi.FileDescription
        }
    }
}

Export-ModuleMember -Function Get-DotNetAssemblyIdentity, Get-PEVersionResourceIdentity
