@{
    # Vendored native library banner patterns, used as a last-resort identity
    # signal for binaries with no Win32 version resource (PLAN.md priority
    # method 6). Deliberately NOT YAML: stage 1 is dependency-free PowerShell,
    # and a YAML config would require pulling in a third-party module just to
    # read it - a PSD1 (native PowerShell data file) needs nothing extra,
    # stays human-editable, and is just as inspectable as YAML would be.
    #
    # Each entry:
    #   Name         - vendor/product label used as identity.product
    #   Pattern      - .NET regex, matched against the file's bytes decoded as
    #                  Latin-1 (1 byte -> 1 char, so it never throws on binary
    #                  content and byte offsets line up 1:1 with the pattern)
    #   VersionGroup - regex capture group index holding the version string
    #   Method       - identity.method to report; defaults to 'string-signature'
    #                  if omitted. The Electron entries use 'electron-embedded'
    #                  since that's a distinct value in the inventory schema.
    Signatures = @(
        @{
            Name         = 'OpenSSL'
            Pattern      = 'OpenSSL\s+(\d+\.\d+\.\d+[a-z]?)\s+\d{1,2}\s+\w+\s+\d{4}'
            VersionGroup = 1
        },
        @{
            Name         = 'zlib'
            Pattern      = 'zlib\s+(\d+\.\d+\.\d+)'
            VersionGroup = 1
        },
        @{
            Name         = 'libcurl'
            Pattern      = 'libcurl/(\d+\.\d+\.\d+)'
            VersionGroup = 1
        },
        @{
            Name         = 'SQLite'
            Pattern      = 'SQLite\s+(?:version\s+)?(\d+\.\d+\.\d+)'
            VersionGroup = 1
        },
        # Added 2026-08-13: found live on a real Firefox scan that the bare
        # "Chrome/X.X.X.X" pattern is a false-positive magnet - Firefox
        # embeds a Chrome-User-Agent-spoofing string (site-compatibility
        # overrides) inside browser\omni.ja that matched this exact
        # pattern, wrongly attributing "Electron Chromium 67.0.3396.87" (and
        # a decade of real Chrome CVEs going back to 2012) to an app that
        # doesn't use Chromium at all - Firefox is Gecko-based. A genuine
        # Electron app's own default User-Agent always carries an adjacent
        # "Electron/<version>" token right after the Chrome/ segment
        # (e.g. "Chrome/91.0.4472.124 Electron/13.1.7 Safari/537.36") -
        # requiring it distinguishes a real embedded-Chromium signal from
        # an unrelated Chrome-shaped string appearing in unrelated content.
        @{
            Name         = 'Electron Chromium'
            Pattern      = 'Chrome/(\d+\.\d+\.\d+\.\d+)\s+Electron/\d+\.\d+\.\d+'
            VersionGroup = 1
            Method       = 'electron-embedded'
        },
        @{
            Name         = 'Electron Node.js'
            Pattern      = 'Node\.js\s*v?(\d+\.\d+\.\d+)'
            VersionGroup = 1
            Method       = 'electron-embedded'
        }
    )
}
