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
        @{ Reason = 'os-system-path'; Contains = @('\Windows\', '\WinSxS\', '\System32\', '\SysWOW64\') }
    )

    NamePatternRules = @(
        @{ Reason = 'resource-only-assembly'; Patterns = @('*.resources.dll') },
        @{ Reason = 'debug-symbols';          Patterns = @('*.pdb') },
        @{ Reason = 'compiled-help';          Patterns = @('*.chm', '*.hlp') },
        @{ Reason = 'windows-manifest';       Patterns = @('*.manifest') }
    )

    ExtensionRules = @(
        @{ Reason = 'font-file';        Extensions = @('.ttf', '.otf', '.woff', '.woff2', '.fon') },
        @{ Reason = 'image-icon-file';  Extensions = @('.png', '.jpg', '.jpeg', '.gif', '.bmp', '.ico', '.svg') },
        @{ Reason = 'media-file';       Extensions = @('.mp3', '.wav', '.mp4', '.avi') },
        @{ Reason = 'help-content';     Extensions = @('.pdf') }
    )

    # Satellite localization assemblies live in culture-code subfolders
    # (en-US, ja-JP, ...) as "<AssemblyName>.resources.dll" - the resource
    # DLL rule above already catches the filename, this catches the same
    # pattern if it ever shows up without that exact suffix.
    CultureFolderRegex = '[\\/][a-z]{2}(-[A-Za-z]{2,4})?[\\/][^\\/]+\.resources\.dll$'
}
