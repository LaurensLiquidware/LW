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
        @{
            Name         = 'Electron Chromium'
            Pattern      = 'Chrome/(\d+\.\d+\.\d+\.\d+)'
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
