@{
    # Transparent, inspectable noise filtering (PLAN.md "noise reduction").
    # First matching rule wins; the reason string is what shows up in the
    # coverage report's exclusion breakdown - keep these specific enough to
    # be meaningful there, not just "excluded".
    #
    # This is a path/name heuristic only (the PoC's stated stretch goal of
    # hashing against a known-good clean-Windows-install set is not
    # implemented here) - see PLAN.md's "Noise reduction" section.

    PathContainsRules = @(
        @{ Reason = 'os-system-path'; Contains = @('\Windows\', '\WinSxS\', '\System32\', '\SysWOW64\') },
        # Added 2026-08-13 from a real 7-Zip scan: 93 of 101 "unresolved"
        # files (92%) were per-language translation .txt files under a
        # \Lang\ folder - not components. Moved that package's honest
        # coverage number from 5.6% to 42.9%. Scoped broadly by folder
        # name (any file under it), not just .txt, since a localization
        # folder essentially never contains an actual third-party
        # component - and this generalizes beyond 7-Zip's specific
        # "Lang" naming to the other common conventions apps use.
        @{ Reason = 'localization-file'; Contains = @('\Lang\', '\Locale\', '\Locales\', '\i18n\', '\l10n\') },
        # Added 2026-08-13 from a real Paint.NET scan: that package was
        # installed via Chocolatey on the capture machine, and Chocolatey's
        # own package-manager footprint (nuspec/nupkg, install-state
        # sentinel files, cached HTTP API responses, logs, helper scripts)
        # got swept into the VHDX alongside the actual app - none of it is
        # part of Paint.NET itself. ~65 of that package's 130 unresolved
        # files were this, in one path prefix.
        @{ Reason = 'package-manager-path'; Contains = @('\chocolatey\', '\ChocolateyHttpCache\') }
    )

    NamePatternRules = @(
        @{ Reason = 'resource-only-assembly'; Patterns = @('*.resources.dll') },
        @{ Reason = 'debug-symbols';          Patterns = @('*.pdb') },
        @{ Reason = 'compiled-help';          Patterns = @('*.chm', '*.hlp') },
        @{ Reason = 'windows-manifest';       Patterns = @('*.manifest') },
        # Added 2026-08-13 from the same Paint.NET scan: bundled-plugin
        # license/readme text (AvifFileType, DDSFileTypePlus, JpegXLFileType,
        # WebPFileType) plus the app's own root License.txt - documentation,
        # not components.
        @{ Reason = 'readme-license-text'; Patterns = @('License.txt', 'Readme.txt', 'Third Party Notices.txt', 'VERIFICATION.txt') }
    )

    ExtensionRules = @(
        @{ Reason = 'font-file';        Extensions = @('.ttf', '.otf', '.woff', '.woff2', '.fon') },
        @{ Reason = 'image-icon-file';  Extensions = @('.png', '.jpg', '.jpeg', '.gif', '.bmp', '.ico', '.svg') },
        @{ Reason = 'media-file';       Extensions = @('.mp3', '.wav', '.mp4', '.avi') },
        @{ Reason = 'help-content';     Extensions = @('.pdf') },
        # Added 2026-08-13 from a real OBS Studio scan: 1656 of 1826
        # "unresolved" files in that package were .ini config/locale files
        # alone (90%), with .pak/.effect adding more - none of these are
        # third-party components, they're app settings/data. Excluding them
        # moved that package's honest coverage number from 3.7% to 53.0%.
        @{ Reason = 'config-file';       Extensions = @('.ini') },
        @{ Reason = 'resource-pack-file'; Extensions = @('.pak') },
        @{ Reason = 'shader-effect-file'; Extensions = @('.effect') },
        # Added 2026-08-13 from the same Paint.NET scan: one .xml per
        # PaintDotNet.*.dll - IntelliSense doc-comment files the .NET
        # compiler emits alongside an assembly, carrying no version/identity
        # information of their own (16 of that package's 130 unresolved
        # files).
        @{ Reason = 'dotnet-xmldoc-file'; Extensions = @('.xml') },
        # Same scan: uncompiled localization data, same spirit as the
        # \Lang\ folder rule above but flat-named instead of folder-based
        # (PaintDotNet.Strings.3.<culture>.resources) - ~40 of that
        # package's 130 unresolved files.
        @{ Reason = 'dotnet-resource-data'; Extensions = @('.resources', '.resx') },
        @{ Reason = 'shell-shortcut';    Extensions = @('.lnk') }
    )

    # Satellite localization assemblies live in culture-code subfolders
    # (en-US, ja-JP, ...) as "<AssemblyName>.resources.dll" - the resource
    # DLL rule above already catches the filename, this catches the same
    # pattern if it ever shows up without that exact suffix.
    CultureFolderRegex = '[\\/][a-z]{2}(-[A-Za-z]{2,4})?[\\/][^\\/]+\.resources\.dll$'
}
