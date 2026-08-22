<#
.SYNOPSIS
    ProfileUnity SplashScreen Logo Manager (Liquidware-branded)
.DESCRIPTION
    GUI tool to set the ProfileUnity client splash screen logo.
    Drops the chosen image into the ProfileUnity Client.NET folder as
    client-custom-logo-300x86.<ext> (per Liquidware KB 12914471137293),
    and keeps a history of previously-used logos so you can restore them.

    Visual language follows the Liquidware / Stratusphere UX design system:
    brand blue #0061A0, zinc-based neutrals, Segoe UI (the design system's
    own documented fallback for Inter in a non-web context), 4px/8px radii,
    flat whisper-shadow cards, and the pale hex-honeycomb brand texture.
.NOTES
    Requires admin rights to write to C:\Program Files\ProfileUnity\Client.NET
    Self-elevates automatically.
#>

param(
    [string]$TargetDir = 'C:\Program Files\ProfileUnity\Client.NET'
)

# ---------------------------------------------------------------------------
# Self-elevate
# ---------------------------------------------------------------------------
$currentIdentity = [Security.Principal.WindowsIdentity]::GetCurrent()
$currentPrincipal = New-Object Security.Principal.WindowsPrincipal($currentIdentity)
if (-not $currentPrincipal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)) {
    $argList = @('-NoProfile', '-ExecutionPolicy', 'Bypass', '-File', "`"$PSCommandPath`"", '-TargetDir', "`"$TargetDir`"")
    Start-Process -FilePath 'powershell.exe' -Verb RunAs -ArgumentList $argList
    exit
}

Add-Type -AssemblyName PresentationFramework
Add-Type -AssemblyName PresentationCore
Add-Type -AssemblyName WindowsBase
Add-Type -AssemblyName System.Windows.Forms
Add-Type -AssemblyName System.Drawing

# ---------------------------------------------------------------------------
# Config / data locations
# ---------------------------------------------------------------------------
$AllowedExtensions = @('.bmp', '.jpg', '.jpeg', '.gif', '.png', '.tif', '.tiff')
$LogoBaseName      = 'client-custom-logo-300x86'
$DataDir           = Join-Path $env:ProgramData 'Liquidware\ProfileUnitySplashScreenLogoManager'
$HistoryDir        = Join-Path $DataDir 'History'
$ManifestPath      = Join-Path $DataDir 'manifest.json'
$CurrentMetaPath   = Join-Path $DataDir 'current.json'

foreach ($d in @($DataDir, $HistoryDir)) {
    if (-not (Test-Path $d)) { New-Item -ItemType Directory -Path $d -Force | Out-Null }
}

# ---------------------------------------------------------------------------
# Embedded Liquidware brand assets (extracted from the Stratusphere UX
# design system: assets/logo-primary-light.svg, assets/logo.svg,
# assets/lwl-hex.png -- rasterized to PNG so they can be embedded inline)
# ---------------------------------------------------------------------------
$Base64_LogoHeader = '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'
$Base64_AppIcon    = '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'
$Base64_HexBg      = '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'

function ConvertFrom-Base64Image([string]$b64) {
    $bytes = [Convert]::FromBase64String($b64)
    $ms = New-Object IO.MemoryStream(,$bytes)
    $img = New-Object System.Windows.Media.Imaging.BitmapImage
    $img.BeginInit()
    $img.CacheOption = [System.Windows.Media.Imaging.BitmapCacheOption]::OnLoad
    $img.StreamSource = $ms
    $img.EndInit()
    $img.Freeze()
    return $img
}

# ---------------------------------------------------------------------------
# Manifest helpers
# ---------------------------------------------------------------------------
function Get-Manifest {
    if (Test-Path $ManifestPath) {
        try {
            $raw = Get-Content $ManifestPath -Raw
            if ([string]::IsNullOrWhiteSpace($raw)) { return @() }
            $items = $raw | ConvertFrom-Json
            return @($items)
        } catch { return @() }
    }
    return @()
}

function Save-Manifest($items) {
    @($items) | ConvertTo-Json -Depth 5 | Set-Content -Path $ManifestPath -Encoding UTF8
}

function Get-CurrentMeta {
    if (Test-Path $CurrentMetaPath) {
        try { return (Get-Content $CurrentMetaPath -Raw | ConvertFrom-Json) } catch { return $null }
    }
    return $null
}

function Get-ExistingLiveLogo {
    if (-not (Test-Path $TargetDir)) { return $null }
    Get-ChildItem -Path $TargetDir -Filter "$LogoBaseName.*" -File -ErrorAction SilentlyContinue | Select-Object -First 1
}

# Archive whatever is currently live into history, then remove it
function Archive-CurrentLogo {
    $existing = Get-ExistingLiveLogo
    if (-not $existing) { return }

    $meta = Get-CurrentMeta
    $originalName = if ($meta -and $meta.OriginalName) { $meta.OriginalName } else { "unknown-original$($existing.Extension)" }

    $timestamp  = Get-Date -Format 'yyyyMMdd-HHmmss'
    $storedFile = "{0}__{1}{2}" -f $timestamp, ([IO.Path]::GetFileNameWithoutExtension($originalName) -replace '[^\w\-]', '_'), $existing.Extension
    Copy-Item -Path $existing.FullName -Destination (Join-Path $HistoryDir $storedFile) -Force

    $manifest = @(Get-Manifest)
    $manifest += [pscustomobject]@{
        Id           = [guid]::NewGuid().ToString()
        StoredFile   = $storedFile
        OriginalName = $originalName
        Extension    = $existing.Extension
        DateArchived = (Get-Date).ToString('yyyy-MM-dd HH:mm:ss')
    }
    Save-Manifest $manifest

    Remove-Item -Path $existing.FullName -Force
    Remove-Item -Path $CurrentMetaPath -Force -ErrorAction SilentlyContinue
}

function Set-NewLogo([string]$SourcePath) {
    if (-not (Test-Path $TargetDir)) {
        throw "Target directory not found: $TargetDir. Is the ProfileUnity Client installed here?"
    }
    $ext = [IO.Path]::GetExtension($SourcePath).ToLowerInvariant()
    if ($AllowedExtensions -notcontains $ext) {
        throw "Unsupported file type '$ext'. Allowed: $($AllowedExtensions -join ', ')"
    }
    $normExt = switch ($ext) {
        '.jpeg' { '.jpg' }
        '.tiff' { '.tif' }
        default { $ext }
    }

    Archive-CurrentLogo

    $targetFile = Join-Path $TargetDir "$LogoBaseName$normExt"
    Copy-Item -Path $SourcePath -Destination $targetFile -Force

    $meta = [pscustomobject]@{
        OriginalName   = [IO.Path]::GetFileName($SourcePath)
        StoredFileName = "$LogoBaseName$normExt"
        DateSet        = (Get-Date).ToString('yyyy-MM-dd HH:mm:ss')
    }
    $meta | ConvertTo-Json | Set-Content -Path $CurrentMetaPath -Encoding UTF8

    return $targetFile
}

function Restore-FromHistoryEntry($entry) {
    $srcPath = Join-Path $HistoryDir $entry.StoredFile
    if (-not (Test-Path $srcPath)) { throw "History file is missing on disk: $($entry.StoredFile)" }
    if (-not (Test-Path $TargetDir)) { throw "Target directory not found: $TargetDir" }

    Archive-CurrentLogo

    $targetFile = Join-Path $TargetDir "$LogoBaseName$($entry.Extension)"
    Copy-Item -Path $srcPath -Destination $targetFile -Force

    $meta = [pscustomobject]@{
        OriginalName   = $entry.OriginalName
        StoredFileName = "$LogoBaseName$($entry.Extension)"
        DateSet        = (Get-Date).ToString('yyyy-MM-dd HH:mm:ss')
    }
    $meta | ConvertTo-Json | Set-Content -Path $CurrentMetaPath -Encoding UTF8

    return $targetFile
}

function Remove-HistoryEntry($entry) {
    $manifest = @(Get-Manifest | Where-Object { $_.Id -ne $entry.Id })
    Save-Manifest $manifest
    $p = Join-Path $HistoryDir $entry.StoredFile
    if (Test-Path $p) { Remove-Item $p -Force }
}

function Get-ImageDimensionsText([string]$path) {
    try {
        $img = [System.Drawing.Image]::FromFile($path)
        $w = $img.Width; $h = $img.Height
        $img.Dispose()
        return "$w x $h"
    } catch { return 'unknown' }
}

# Saves whatever image is currently on the clipboard (e.g. from a browser's
# "Copy image" context menu command) to a temp PNG file. Returns the path,
# or $null if there's no image on the clipboard.
function Save-ClipboardImageToFile {
    try {
        if (-not [System.Windows.Clipboard]::ContainsImage()) { return $null }
        $img = [System.Windows.Clipboard]::GetImage()
        if (-not $img) { return $null }
        $tempPath = Join-Path $env:TEMP ("pu-logo-search-{0}.png" -f (Get-Date -Format 'yyyyMMdd-HHmmss'))
        $encoder = New-Object System.Windows.Media.Imaging.PngBitmapEncoder
        $encoder.Frames.Add([System.Windows.Media.Imaging.BitmapFrame]::Create($img))
        $stream = [IO.File]::Open($tempPath, [IO.FileMode]::Create)
        $encoder.Save($stream)
        $stream.Close()
        return $tempPath
    } catch {
        return $null
    }
}

# ---------------------------------------------------------------------------
# XAML UI -- Liquidware / Stratusphere UX brand tokens applied:
#   Primary 600/700/800 #0061A0/#005084/#003F67, Surface (zinc) neutrals,
#   4px (sm) radius on buttons/fields, 8px (lg) radius on cards,
#   14px base type / 16px header, 48px fixed header bar, whisper-soft
#   card shadow, Good/Poor (#16A34A / #DC2626) reused as status feedback.
# ---------------------------------------------------------------------------
[xml]$xaml = @'
<Window xmlns="http://schemas.microsoft.com/winfx/2006/xaml/presentation"
        xmlns:x="http://schemas.microsoft.com/winfx/2006/xaml"
        Title="ProfileUnity SplashScreen Logo Manager" Height="700" Width="920"
        WindowStartupLocation="CenterScreen" FontFamily="Segoe UI" FontSize="14"
        Background="#FAFAFA">
    <Window.Resources>
        <SolidColorBrush x:Key="Primary500" Color="#0072BC"/>
        <SolidColorBrush x:Key="Primary600" Color="#0061A0"/>
        <SolidColorBrush x:Key="Primary700" Color="#005084"/>
        <SolidColorBrush x:Key="Primary800" Color="#003F67"/>
        <SolidColorBrush x:Key="Primary50"  Color="#F2F8FC"/>
        <SolidColorBrush x:Key="Surface0"   Color="#FFFFFF"/>
        <SolidColorBrush x:Key="Surface50"  Color="#FAFAFA"/>
        <SolidColorBrush x:Key="Surface100" Color="#F4F4F5"/>
        <SolidColorBrush x:Key="Surface200" Color="#E4E4E7"/>
        <SolidColorBrush x:Key="Surface300" Color="#D4D4D8"/>
        <SolidColorBrush x:Key="Surface500" Color="#71717A"/>
        <SolidColorBrush x:Key="Surface800" Color="#27272A"/>
        <SolidColorBrush x:Key="GoodColor"  Color="#16A34A"/>
        <SolidColorBrush x:Key="PoorColor"  Color="#DC2626"/>

        <!-- Primary filled button: brand blue, darkens one step on hover, another on press -->
        <Style x:Key="PrimaryButton" TargetType="Button">
            <Setter Property="Background" Value="{StaticResource Primary600}"/>
            <Setter Property="Foreground" Value="White"/>
            <Setter Property="FontWeight" Value="Normal"/>
            <Setter Property="Padding" Value="16,9"/>
            <Setter Property="BorderThickness" Value="0"/>
            <Setter Property="Cursor" Value="Hand"/>
            <Setter Property="Template">
                <Setter.Value>
                    <ControlTemplate TargetType="Button">
                        <Border x:Name="Bd" Background="{TemplateBinding Background}" CornerRadius="4" Padding="{TemplateBinding Padding}">
                            <ContentPresenter HorizontalAlignment="Center" VerticalAlignment="Center"/>
                        </Border>
                        <ControlTemplate.Triggers>
                            <Trigger Property="IsMouseOver" Value="True">
                                <Setter TargetName="Bd" Property="Background" Value="{StaticResource Primary700}"/>
                            </Trigger>
                            <Trigger Property="IsPressed" Value="True">
                                <Setter TargetName="Bd" Property="Background" Value="{StaticResource Primary800}"/>
                            </Trigger>
                            <Trigger Property="IsEnabled" Value="False">
                                <Setter TargetName="Bd" Property="Opacity" Value="0.5"/>
                            </Trigger>
                        </ControlTemplate.Triggers>
                    </ControlTemplate>
                </Setter.Value>
            </Setter>
        </Style>

        <!-- Secondary / outline button: neutral surface wash on hover, matches nav hover behaviour -->
        <Style x:Key="SecondaryButton" TargetType="Button">
            <Setter Property="Background" Value="Transparent"/>
            <Setter Property="Foreground" Value="{StaticResource Surface800}"/>
            <Setter Property="Padding" Value="16,9"/>
            <Setter Property="BorderBrush" Value="{StaticResource Surface300}"/>
            <Setter Property="BorderThickness" Value="1"/>
            <Setter Property="Cursor" Value="Hand"/>
            <Setter Property="Template">
                <Setter.Value>
                    <ControlTemplate TargetType="Button">
                        <Border x:Name="Bd" Background="{TemplateBinding Background}" BorderBrush="{TemplateBinding BorderBrush}" BorderThickness="{TemplateBinding BorderThickness}" CornerRadius="4" Padding="{TemplateBinding Padding}">
                            <ContentPresenter HorizontalAlignment="Center" VerticalAlignment="Center"/>
                        </Border>
                        <ControlTemplate.Triggers>
                            <Trigger Property="IsMouseOver" Value="True">
                                <Setter TargetName="Bd" Property="Background" Value="{StaticResource Surface200}"/>
                            </Trigger>
                            <Trigger Property="IsEnabled" Value="False">
                                <Setter TargetName="Bd" Property="Opacity" Value="0.5"/>
                            </Trigger>
                        </ControlTemplate.Triggers>
                    </ControlTemplate>
                </Setter.Value>
            </Setter>
        </Style>

        <Style TargetType="DataGridColumnHeader">
            <Setter Property="Background" Value="{StaticResource Surface100}"/>
            <Setter Property="Foreground" Value="{StaticResource Surface800}"/>
            <Setter Property="FontWeight" Value="SemiBold"/>
            <Setter Property="Padding" Value="10,8"/>
            <Setter Property="BorderBrush" Value="{StaticResource Surface300}"/>
            <Setter Property="BorderThickness" Value="0,0,0,1"/>
            <Setter Property="HorizontalContentAlignment" Value="Left"/>
        </Style>

        <Style x:Key="{x:Type DataGridRow}" TargetType="DataGridRow">
            <Setter Property="Background" Value="Transparent"/>
            <Style.Triggers>
                <Trigger Property="IsMouseOver" Value="True">
                    <Setter Property="Background" Value="{StaticResource Surface100}"/>
                </Trigger>
                <Trigger Property="IsSelected" Value="True">
                    <Setter Property="Background" Value="{StaticResource Primary50}"/>
                </Trigger>
            </Style.Triggers>
        </Style>
    </Window.Resources>

    <Grid>
        <Grid.RowDefinitions>
            <RowDefinition Height="48"/>
            <RowDefinition Height="*"/>
        </Grid.RowDefinitions>

        <!-- Fixed 48px brand header bar -->
        <Border Grid.Row="0" Background="{StaticResource Primary600}">
            <DockPanel LastChildFill="False" Margin="16,0">
                <Image Name="ImgLogo" DockPanel.Dock="Left" Height="22" Width="65" VerticalAlignment="Center" Stretch="Uniform" SnapsToDevicePixels="True"/>
                <Border DockPanel.Dock="Left" Width="1" Height="20" Background="#33FFFFFF" Margin="14,0"/>
                <TextBlock DockPanel.Dock="Left" Text="ProfileUnity SplashScreen Logo" Foreground="White" FontSize="16" VerticalAlignment="Center"/>
            </DockPanel>
        </Border>

        <!-- Content canvas: hex brand texture bleeds subtly from the top-left corner -->
        <Grid Grid.Row="1" Background="{StaticResource Surface50}" ClipToBounds="True">
            <Image Name="ImgHexBg" Width="220" Height="150" HorizontalAlignment="Left" VerticalAlignment="Top"
                   Stretch="Uniform" Opacity="0.8" IsHitTestVisible="False"/>

            <Grid Margin="20">
                <Grid.RowDefinitions>
                    <RowDefinition Height="Auto"/>
                    <RowDefinition Height="Auto"/>
                    <RowDefinition Height="*"/>
                    <RowDefinition Height="Auto"/>
                    <RowDefinition Height="Auto"/>
                </Grid.RowDefinitions>

                <!-- Current logo card -->
                <Border Grid.Row="0" Background="{StaticResource Surface0}" BorderBrush="{StaticResource Surface300}"
                        BorderThickness="1" CornerRadius="8" Padding="16" Margin="0,0,0,20">
                    <Border.Effect>
                        <DropShadowEffect Color="Black" Opacity="0.10" BlurRadius="6" ShadowDepth="1" Direction="270"/>
                    </Border.Effect>
                    <Grid>
                        <Grid.ColumnDefinitions>
                            <ColumnDefinition Width="140"/>
                            <ColumnDefinition Width="*"/>
                        </Grid.ColumnDefinitions>
                        <Border Grid.Column="0" BorderBrush="{StaticResource Surface300}" BorderThickness="1"
                                CornerRadius="4" Background="{StaticResource Surface50}" Width="130" Height="60"
                                HorizontalAlignment="Left">
                            <Image Name="ImgCurrent" Stretch="Uniform" Margin="4"/>
                        </Border>
                        <StackPanel Grid.Column="1" Margin="16,0,0,0" VerticalAlignment="Center">
                            <TextBlock Text="Current Splash Logo" FontWeight="Medium" FontSize="14" Foreground="{StaticResource Surface800}"/>
                            <TextBlock Name="TxtCurrentInfo" Margin="0,4,0,0" TextWrapping="Wrap" Foreground="{StaticResource Surface500}" FontSize="13"/>

                            <StackPanel Orientation="Horizontal" Margin="0,12,0,0">
                                <TextBlock Text="Search Images:" VerticalAlignment="Center" FontSize="13" Foreground="{StaticResource Surface800}" Margin="0,0,8,0"/>
                                <Border BorderBrush="{StaticResource Surface300}" BorderThickness="1" CornerRadius="4" Background="{StaticResource Surface0}">
                                    <TextBox Name="TxtSearchQuery" Width="220" Padding="8,6" BorderThickness="0" Background="Transparent" FontSize="13" VerticalContentAlignment="Center"/>
                                </Border>
                                <Button Name="BtnSearchImages" Content="Search" Style="{StaticResource SecondaryButton}" Margin="8,0,0,0"/>                            </StackPanel>

                            <StackPanel Orientation="Horizontal" Margin="0,10,0,0">
                                <Button Name="BtnBrowse" Content="Browse..." Style="{StaticResource SecondaryButton}" Margin="0,0,8,0"/>
                                <Button Name="BtnImportClipboard" Content="Import from Clipboard" Style="{StaticResource SecondaryButton}" Margin="0,0,8,0"/>
                                <Button Name="BtnSet" Content="Set as Splash Logo" Style="{StaticResource PrimaryButton}" Margin="0,0,8,0" IsEnabled="False"/>
                                <Button Name="BtnPreviewSplash" Content="Preview Splash Screen" Style="{StaticResource SecondaryButton}"/>
                            </StackPanel>
                        </StackPanel>
                    </Grid>
                </Border>

                <TextBlock Grid.Row="1" Text="Logo History" FontWeight="Medium" FontSize="14" Foreground="{StaticResource Surface800}" Margin="0,0,0,8"/>

                <Border Grid.Row="2" BorderBrush="{StaticResource Surface300}" BorderThickness="1" CornerRadius="8" Background="{StaticResource Surface0}">
                    <DataGrid Name="GridHistory" AutoGenerateColumns="False" IsReadOnly="True" BorderThickness="0"
                              RowHeight="30" GridLinesVisibility="Horizontal" HorizontalGridLinesBrush="{StaticResource Surface200}"
                              SelectionMode="Single" CanUserAddRows="False" Background="Transparent" RowBackground="Transparent"
                              FontSize="13" HeadersVisibility="Column">
                        <DataGrid.Columns>
                            <DataGridTextColumn Header="Date Archived" Binding="{Binding DateArchived}" Width="170"/>
                            <DataGridTextColumn Header="Original File Name" Binding="{Binding OriginalName}" Width="*"/>
                            <DataGridTextColumn Header="Ext" Binding="{Binding Extension}" Width="70"/>
                        </DataGrid.Columns>
                    </DataGrid>
                </Border>

                <StackPanel Grid.Row="3" Orientation="Horizontal" Margin="0,12,0,0">
                    <Button Name="BtnRestore" Content="Restore Selected" Style="{StaticResource SecondaryButton}" Margin="0,0,8,0"/>
                    <Button Name="BtnDelete" Content="Delete Selected From History" Style="{StaticResource SecondaryButton}"/>
                </StackPanel>

                <TextBlock Grid.Row="4" Name="TxtStatus" Margin="0,12,0,0" Foreground="{StaticResource GoodColor}" TextWrapping="Wrap" FontSize="13"/>
            </Grid>
        </Grid>
    </Grid>
</Window>
'@

$reader = New-Object System.Xml.XmlNodeReader $xaml
$window = [Windows.Markup.XamlReader]::Load($reader)

$window.Icon    = ConvertFrom-Base64Image $Base64_AppIcon

$ImgLogo         = $window.FindName('ImgLogo')
$ImgHexBg        = $window.FindName('ImgHexBg')
$ImgCurrent      = $window.FindName('ImgCurrent')
$TxtCurrentInfo  = $window.FindName('TxtCurrentInfo')
$TxtSearchQuery  = $window.FindName('TxtSearchQuery')
$BtnSearchImages = $window.FindName('BtnSearchImages')
$BtnBrowse       = $window.FindName('BtnBrowse')
$BtnImportClipboard = $window.FindName('BtnImportClipboard')
$BtnSet          = $window.FindName('BtnSet')
$BtnPreviewSplash = $window.FindName('BtnPreviewSplash')
$GridHistory     = $window.FindName('GridHistory')
$BtnRestore      = $window.FindName('BtnRestore')
$BtnDelete       = $window.FindName('BtnDelete')
$TxtStatus       = $window.FindName('TxtStatus')

$ImgLogo.Source  = ConvertFrom-Base64Image $Base64_LogoHeader
$ImgHexBg.Source = ConvertFrom-Base64Image $Base64_HexBg

$GoodBrush    = New-Object System.Windows.Media.SolidColorBrush ([System.Windows.Media.Color]::FromRgb(0x16,0xA3,0x4A))
$PoorBrush    = New-Object System.Windows.Media.SolidColorBrush ([System.Windows.Media.Color]::FromRgb(0xDC,0x26,0x26))
$FairBrush    = New-Object System.Windows.Media.SolidColorBrush ([System.Windows.Media.Color]::FromRgb(0xCA,0x8A,0x04))
$MutedBrush   = New-Object System.Windows.Media.SolidColorBrush ([System.Windows.Media.Color]::FromRgb(0x71,0x71,0x7A))

# Path to the ProfileUnity client-side init executable, used to trigger a live
# splash screen preview so the logo can be checked in context.
$CtxInitExePath = Join-Path $TargetDir 'LwL.ProfileUnity.Client.CtxInit.exe'

# Path of a logo the user has browsed to but not yet committed as the live logo
$script:PendingLogoPath = $null

function Show-Status([string]$msg, [bool]$isError = $false) {
    $TxtStatus.Foreground = if ($isError) { $PoorBrush } else { $GoodBrush }
    $TxtStatus.Text = $msg
}

function Load-ImagePreview([System.Windows.Controls.Image]$imgControl, [string]$path) {
    if (-not $path -or -not (Test-Path $path)) {
        $imgControl.Source = $null
        return
    }
    try {
        $bmp = New-Object System.Windows.Media.Imaging.BitmapImage
        $bmp.BeginInit()
        $bmp.CacheOption = [System.Windows.Media.Imaging.BitmapCacheOption]::OnLoad
        $bmp.UriSource = New-Object System.Uri($path)
        $bmp.EndInit()
        $bmp.Freeze()
        $imgControl.Source = $bmp
    } catch {
        $imgControl.Source = $null
    }
}

function Refresh-UI {
    # Any browsed-but-uncommitted preview is discarded whenever we refresh to
    # reflect the actual live state on disk.
    $script:PendingLogoPath = $null
    $BtnSet.IsEnabled = $false
    $TxtCurrentInfo.Foreground = $MutedBrush

    if (-not (Test-Path $TargetDir)) {
        $TxtCurrentInfo.Text = "Target directory not found. Is the ProfileUnity Client installed here?"
        $ImgCurrent.Source = $null
        $BtnBrowse.IsEnabled = $false
        $BtnRestore.IsEnabled = $false
    } else {
        $BtnBrowse.IsEnabled = $true
        $existing = Get-ExistingLiveLogo
        if ($existing) {
            $meta = Get-CurrentMeta
            $dims = Get-ImageDimensionsText $existing.FullName
            $originalNote = if ($meta -and $meta.OriginalName) { " (original file: $($meta.OriginalName))" } else { '' }
            $dimsWarning = if ($dims -ne '300 x 86') { "  Note: recommended size is 300x86 -- this file is $dims." } else { '' }
            $TxtCurrentInfo.Text = "$($existing.Name)$originalNote`nSet $(if ($meta) { $meta.DateSet } else { 'unknown' })$dimsWarning"
            Load-ImagePreview $ImgCurrent $existing.FullName
        } else {
            $TxtCurrentInfo.Text = "No custom splash logo is currently set. The default ProfileUnity logo is in use."
            $ImgCurrent.Source = $null
        }
        $BtnRestore.IsEnabled = $true
    }

    $manifest = @(Get-Manifest) | Sort-Object { [datetime]$_.DateArchived } -Descending
    $GridHistory.ItemsSource = $manifest
}

function Set-PendingPreview([string]$path) {
    $ext = [IO.Path]::GetExtension($path).ToLowerInvariant()
    if ($AllowedExtensions -notcontains $ext) {
        Show-Status "Unsupported file type '$ext'. Allowed: $($AllowedExtensions -join ', ')" $true
        return $false
    }
    $script:PendingLogoPath = $path
    Load-ImagePreview $ImgCurrent $script:PendingLogoPath

    $dims = Get-ImageDimensionsText $script:PendingLogoPath
    $dimsNote = if ($dims -ne '300 x 86') { "  Recommended size is 300x86 -- this file is $dims." } else { '' }
    $TxtCurrentInfo.Text = "Previewing: $([IO.Path]::GetFileName($script:PendingLogoPath))$dimsNote`nNot yet set -- click 'Set as Splash Logo' to apply."
    $TxtCurrentInfo.Foreground = $FairBrush

    $BtnSet.IsEnabled = $true
    return $true
}

$BtnSearchImages.Add_Click({
    $query = $TxtSearchQuery.Text.Trim()
    if ([string]::IsNullOrWhiteSpace($query)) {
        Show-Status 'Enter a search term first.' $true
        return
    }
    $encoded = [Uri]::EscapeDataString($query)
    $url = "https://www.google.com/search?tbm=isch&q=$encoded"
    try {
        Start-Process $url
        Show-Status "Opened image search for '$query' in your browser. Right-click an image, choose 'Copy image', then click 'Import from Clipboard' below."
    } catch {
        Show-Status "An error occurred: $($_.Exception.Message)" $true
    }
})
$TxtSearchQuery.Add_KeyDown({
    param($src, $e)
    if ($e.Key -eq [System.Windows.Input.Key]::Enter) {
        $query = $TxtSearchQuery.Text.Trim()
        if ([string]::IsNullOrWhiteSpace($query)) {
            Show-Status 'Enter a search term first.' $true
            return
        }
        $encoded = [Uri]::EscapeDataString($query)
        $url = "https://www.google.com/search?tbm=isch&q=$encoded"
        try {
            Start-Process $url
            Show-Status "Opened image search for '$query' in your browser. Right-click an image, choose 'Copy image', then click 'Import from Clipboard' below."
        } catch {
            Show-Status "An error occurred: $($_.Exception.Message)" $true
        }
    }
})

$BtnBrowse.Add_Click({
    $dlg = New-Object System.Windows.Forms.OpenFileDialog
    $dlg.Filter = 'Image files (*.bmp;*.jpg;*.jpeg;*.gif;*.png;*.tif;*.tiff)|*.bmp;*.jpg;*.jpeg;*.gif;*.png;*.tif;*.tiff|All files (*.*)|*.*'
    $dlg.Title = 'Select splash screen logo image'
    if ($dlg.ShowDialog() -eq [System.Windows.Forms.DialogResult]::OK) {
        if (Set-PendingPreview $dlg.FileName) {
            Show-Status 'Previewing selected file. Not yet applied.'
        }
    }
})

$BtnImportClipboard.Add_Click({
    $tempPath = Save-ClipboardImageToFile
    if (-not $tempPath) {
        Show-Status "No image found on the clipboard. Right-click an image in your browser and choose 'Copy image' first." $true
        return
    }
    if (Set-PendingPreview $tempPath) {
        Show-Status 'Previewing image imported from clipboard. Not yet applied.'
    }
})

$BtnSet.Add_Click({
    if (-not $script:PendingLogoPath) { Show-Status 'Browse for a logo first.' $true; return }
    try {
        $result = Set-NewLogo -SourcePath $script:PendingLogoPath
        Show-Status "Splash logo updated: $result"
        Refresh-UI
    } catch {
        Show-Status "An error occurred: $($_.Exception.Message)" $true
    }
})

$BtnPreviewSplash.Add_Click({
    if (-not (Test-Path $CtxInitExePath)) {
        Show-Status "Preview executable not found: $CtxInitExePath" $true
        return
    }
    try {
        Start-Process -FilePath $CtxInitExePath
        Show-Status 'Launched splash screen preview.'
    } catch {
        Show-Status "An error occurred: $($_.Exception.Message)" $true
    }
})

$BtnRestore.Add_Click({
    $sel = $GridHistory.SelectedItem
    if (-not $sel) { Show-Status 'Select a history entry first.' $true; return }
    $confirm = [System.Windows.MessageBox]::Show(
        "Restore '$($sel.OriginalName)' as the live splash logo?`nThe current logo will be moved into history.",
        'Confirm Restore', 'YesNo', 'Question')
    if ($confirm -ne 'Yes') { return }
    try {
        $result = Restore-FromHistoryEntry $sel
        Show-Status "Restored: $result"
        Refresh-UI
    } catch {
        Show-Status "An error occurred: $($_.Exception.Message)" $true
    }
})

$BtnDelete.Add_Click({
    $sel = $GridHistory.SelectedItem
    if (-not $sel) { Show-Status 'Select a history entry first.' $true; return }
    $confirm = [System.Windows.MessageBox]::Show(
        "Permanently delete '$($sel.OriginalName)' ($($sel.DateArchived)) from history?",
        'Confirm Delete', 'YesNo', 'Warning')
    if ($confirm -ne 'Yes') { return }
    Remove-HistoryEntry $sel
    Show-Status 'Removed from history.'
    Refresh-UI
})

Refresh-UI
$window.ShowDialog() | Out-Null
