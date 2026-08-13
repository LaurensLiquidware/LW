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
        # own package-manager footprint (install-state sentinel files,
        # cached HTTP API responses, logs, helper scripts) got swept into
        # the VHDX alongside the actual app - none of it is part of
        # Paint.NET itself. ~65 of that package's 130 unresolved files
        # were this, in one path prefix.
        #
        # NARROWED 2026-08-13 after a real Tor Browser scan: the original
        # rule matched anything under "\chocolatey\", which wrongly
        # excluded ALL 186 real application files (firefox.exe, nss3.dll,
        # softokn3.dll, ...) for a "portable" Chocolatey package -
        # Chocolatey's "lib\<pkg>\tools\" convention is sometimes its own
        # management scaffolding (MSI installers, install scripts) and
        # sometimes IS the actual installed application, depending on how
        # that package is authored. Scoped down to only Chocolatey's own
        # management subfolders, which are never app payload regardless of
        # package layout; the package manifest/installer-script name
        # patterns below (`*.nuspec`/`*.nupkg`/`chocolateyInstall.ps1`/
        # `chocolateyUninstall.ps1`/`*.ignore`) catch Chocolatey's own
        # files that can appear anywhere, including inside "tools\".
        @{ Reason = 'package-manager-path'; Contains = @('\chocolatey\.chocolatey\', '\chocolatey\extensions\', '\chocolatey\logs\', '\ChocolateyHttpCache\') },
        # Added 2026-08-13, same Tor Browser narrowing: a Chocolatey
        # "extension" package (name ends in ".extension") duplicates its
        # own payload into a nested "extensions\" folder inside its own
        # lib\<pkg>\ directory, mirroring the top-level \chocolatey\
        # extensions\ layout already excluded above - found live on a
        # Notepad++ scan whose capture machine also had
        # chocolatey-core.extension/chocolatey-compatibility.extension
        # installed. Never app payload, regardless of which app is being
        # scanned.
        @{ Reason = 'package-manager-path'; Contains = @('.extension\extensions\') },
        # Added 2026-08-13 from a real Remix-H1-DROOG (Jonker ERP) scan:
        # 868 of 968 "unresolved" files (90%) lived entirely under this
        # one report-designs folder - JasperReports compiled report files
        # (.jasper) and their locale-variant .properties sidecars
        # (Aannemer.properties/_en.properties/_bam.properties). The
        # *.jasper* name pattern below catches the report files themselves
        # regardless of folder; this folder rule is needed for the sidecar
        # .properties files, which aren't identifiable by extension alone.
        @{ Reason = 'report-design-folder'; Contains = @('\TClickRapporten\') },
        # Added 2026-08-13, same Remix-H1-DROOG scan: two more noise
        # categories from the same package's remaining unresolved files -
        # a "\Wijzigingen\" (Dutch: "changes") folder of plain-text
        # release notes/changelogs (35 files), and a "\Log\<module>\"
        # runtime log-output folder (28 files). Neither is a component.
        @{ Reason = 'changelog-folder'; Contains = @('\Wijzigingen\') },
        @{ Reason = 'log-folder'; Contains = @('\Log\') },
        # Same scan: Oracle's own JRE usage-telemetry files
        # ("$env:LOCALAPPDATA\Oracle\Java\.oracle_jre_usage\<hash>.timestamp")
        # - not part of the packaged app, just an artifact of any machine
        # that has ever run a JRE. Only 2 occurrences in this package, but
        # unambiguous and likely to recur on any Java app's capture.
        @{ Reason = 'jre-telemetry'; Contains = @('\.oracle_jre_usage\') },
        # Added 2026-08-13: appinstall.cap/printers.bak/DisableShortPaths/
        # Suppress.ACL had shown up in a top-level "\Data\" folder (sibling
        # to "\Volumes\C\...", not inside it) on every single one of 9
        # different real packages tested so far (OBS Studio, 7-Zip,
        # Paint.NET, Notepad++, Python, Remix-H1-DROOG, Tor Browser,
        # Firefox, Chromium) - always these exact 4 filenames, always
        # unresolved (identity: null every time). This is FlexApp/
        # ProfileUnity's own package-capture scaffolding, not app content -
        # scoped to the exact folder+filename combination (not a bare
        # filename or folder-name match) so a real app's own "data" folder
        # elsewhere in the tree (e.g. OBS Studio's own
        # "obs-studio\data\obs-studio\...") is never touched.
        @{ Reason = 'flexapp-capture-scaffolding'; Contains = @('\Data\appinstall.cap', '\Data\printers.bak', '\Data\DisableShortPaths', '\Data\Suppress.ACL') }
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
        @{ Reason = 'readme-license-text'; Patterns = @('License.txt', 'Readme.txt', 'Third Party Notices.txt', 'VERIFICATION.txt') },
        # Added 2026-08-13, same Paint.NET scan: pure .NET runtime
        # target-framework config (rollForward policy, framework version) -
        # never a component. Deliberately NOT excluding the sibling
        # *.deps.json here - that one is a real dependency lockfile (exact
        # NuGet package name + version pairs) worth parsing for identity
        # resolution rather than discarding as noise; see PLAN.md.
        @{ Reason = 'dotnet-runtimeconfig'; Patterns = @('*.runtimeconfig.json') },
        # Added 2026-08-13, same Remix-H1-DROOG scan: JasperReports
        # compiled report-design files - never a component, regardless of
        # which app embeds JasperReports or what folder it keeps its
        # designs in. Wildcarded on both sides to also catch the
        # date-stamped backup copies found live (Bon.jasper.2013-07-18,
        # Bon.jasper_20221129, Bon.jasper-29-03-2015), which a plain
        # ".jasper" extension rule would miss - GetExtension() only
        # returns the file's last dot-segment, and for those it's the
        # date suffix, not ".jasper".
        @{ Reason = 'report-design-file'; Patterns = @('*.jasper*') },
        # Added 2026-08-13 from the same Tor Browser scan, alongside the
        # package-manager-path narrowing above: Chocolatey's own package
        # manifest/archive and installer-script files, which can sit
        # directly inside a portable package's "tools\" folder next to the
        # real application - a folder-wide exclude there would have caught
        # the app payload too, so these are matched by filename instead.
        # "*.ignore" is Chocolatey's own convention for telling its shim
        # generator to skip a given exe (firefox.exe.ignore,
        # updater.exe.ignore, ...) - never a component itself. Widened
        # from two specific hook-script names to "chocolatey*.ps1" after a
        # Notepad++ scan surfaced a third one (chocolateyBeforeModify.ps1)
        # - Chocolatey has several conventionally-named hook scripts
        # (Install/Uninstall/BeforeModify/...) and "chocolatey" as a
        # script-name prefix is unambiguous, never a coincidental real
        # app script name.
        @{ Reason = 'package-manager-file'; Patterns = @('*.nuspec', '*.nupkg', 'chocolatey*.ps1', '*.ignore') }
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
        # files). Renamed from the original 'dotnet-xmldoc-file' after a
        # later Notepad++ scan (a native C++ app, zero managed code) showed
        # the same rule correctly excluding 108 .xml files that were never
        # .NET doc comments at all - autoCompletion/theme/langs-model XML
        # config. The exclusion was right both times; the old name was
        # misleadingly .NET-specific for what's really a general "no
        # third-party component ships its identity as bare .xml" rule.
        @{ Reason = 'xml-data-file'; Extensions = @('.xml') },
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
