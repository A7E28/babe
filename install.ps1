param(
    [switch]$Force
)

$ErrorActionPreference = "Stop"

[Net.ServicePointManager]::SecurityProtocol = [Net.ServicePointManager]::SecurityProtocol -bor [Net.SecurityProtocolType]::Tls12

$Repo = "A7E28/babe"
$ManifestUrl = "https://github.com/$Repo/releases/latest/download/latest.json"

$LocalAppData = $env:LOCALAPPDATA
if ([string]::IsNullOrWhiteSpace($LocalAppData)) {
    $LocalAppData = Join-Path $HOME "AppData\Local"
}
$InstallDir = Join-Path $LocalAppData "babe\bin"
$TargetExe = Join-Path $InstallDir "babe.exe"

$Arch = "amd64"
if ($env:PROCESSOR_ARCHITECTURE -eq "ARM64" -or $env:PROCESSOR_ARCHITEW6432 -eq "ARM64") {
    $Arch = "arm64"
}
$PlatformKey = "windows-$Arch"

Write-Host "--- babe Installer (windows/$Arch) ---" -ForegroundColor Cyan

Write-Host "[INFO] Fetching latest release manifest from GitHub..."
$Manifest = Invoke-RestMethod -Uri $ManifestUrl -Headers @{
    "User-Agent" = "babe-installer"
    "Accept"     = "application/json"
}

$Tag = "$($Manifest.tag_name)".Trim()
if ([string]::IsNullOrWhiteSpace($Tag)) {
    throw "Release manifest is missing tag_name"
}

$Asset = $Manifest.assets.$PlatformKey
if (-not $Asset) {
    throw "No release asset found for platform $PlatformKey in release $Tag"
}

$AssetName = "$($Asset.name)".Trim()
$DownloadUrl = "$($Asset.url)".Trim()
$ExpectedHash = "$($Asset.sha256)".Trim().ToLower()

if ([string]::IsNullOrWhiteSpace($AssetName) -or [string]::IsNullOrWhiteSpace($DownloadUrl)) {
    throw "Release manifest entry for $PlatformKey is incomplete"
}

$Uri = [System.Uri]$DownloadUrl
$TrustedHost = (
    $Uri.Scheme -eq "https" -and (
        ($Uri.Host -ieq "github.com" -and $Uri.AbsolutePath.StartsWith("/$Repo/releases/download/")) -or
        $Uri.Host -ieq "objects.githubusercontent.com" -or
        $Uri.Host -ieq "release-assets.githubusercontent.com"
    )
)
if (-not $TrustedHost) {
    throw "Untrusted download URL in manifest: $DownloadUrl"
}

if ($ExpectedHash -notmatch '^[0-9a-f]{64}$') {
    throw "Invalid or missing SHA-256 checksum in manifest for $AssetName"
}

if (-not (Test-Path $InstallDir)) {
    New-Item -ItemType Directory -Force -Path $InstallDir | Out-Null
}

Get-ChildItem -Path $InstallDir -Filter "babe.exe*.old" -ErrorAction SilentlyContinue | ForEach-Object {
    Remove-Item -Force $_.FullName -ErrorAction SilentlyContinue
}

$NeedDownload = $true
if ((Test-Path $TargetExe) -and (-not $Force)) {
    $CurrentHash = (Get-FileHash -Path $TargetExe -Algorithm SHA256).Hash.ToLower()
    if ($CurrentHash -eq $ExpectedHash) {
        Write-Host "[OK] babe $Tag is already installed and up to date at:" -ForegroundColor Green
        Write-Host "     $TargetExe"
        $NeedDownload = $false
    }
}

if ($NeedDownload) {
    $TempFile = Join-Path $InstallDir ".babe-install-$PID.tmp"
    if (Test-Path $TempFile) {
        Remove-Item -Force $TempFile -ErrorAction SilentlyContinue
    }

    try {
        Write-Host "[INFO] Downloading $AssetName ($Tag)..."
        Invoke-WebRequest -Uri $DownloadUrl -OutFile $TempFile -UseBasicParsing -Headers @{ "User-Agent" = "babe-installer" }

        $ActualHash = (Get-FileHash -Path $TempFile -Algorithm SHA256).Hash.ToLower()
        if ($ActualHash -ne $ExpectedHash) {
            throw "SHA256 checksum mismatch! Expected $ExpectedHash, got $ActualHash"
        }
        Write-Host "[OK] Verified SHA256: $ActualHash" -ForegroundColor Green

        $OldFile = "$TargetExe.$PID.old"
        $MovedOld = $false
        if (Test-Path $TargetExe) {
            Move-Item -Force -Path $TargetExe -Destination $OldFile
            $MovedOld = $true
        }
        try {
            Move-Item -Force -Path $TempFile -Destination $TargetExe
        }
        catch {
            if ($MovedOld -and (Test-Path $OldFile) -and (-not (Test-Path $TargetExe))) {
                Move-Item -Force -Path $OldFile -Destination $TargetExe -ErrorAction SilentlyContinue
            }
            throw
        }
        if ($MovedOld) {
            Remove-Item -Force $OldFile -ErrorAction SilentlyContinue
        }

        Write-Host "[OK] Installed babe $Tag to $TargetExe" -ForegroundColor Green
    }
    finally {
        if (Test-Path $TempFile) {
            Remove-Item -Force $TempFile -ErrorAction SilentlyContinue
        }
    }
}

# Register in HKCU\Environment\Path
$RegKey = [Microsoft.Win32.Registry]::CurrentUser.OpenSubKey("Environment", $true)
if ($null -eq $RegKey) {
    $RegKey = [Microsoft.Win32.Registry]::CurrentUser.CreateSubKey("Environment")
}

$RawPath = $RegKey.GetValue("Path", "", [Microsoft.Win32.RegistryValueOptions]::DoNotExpandEnvironmentNames)
$NormTarget = $InstallDir.TrimEnd('\', '/')
$AlreadyInReg = $false

foreach ($Seg in ($RawPath -split ';')) {
    $CleanSeg = $Seg.Trim().TrimEnd('\', '/')
    if ($CleanSeg -ieq $NormTarget) {
        $AlreadyInReg = $true
        break
    }
}

if ($AlreadyInReg) {
    Write-Host "[OK] User PATH already contains $InstallDir" -ForegroundColor Green
}
else {
    $NewPath = if ([string]::IsNullOrWhiteSpace($RawPath)) { $InstallDir } else { "$RawPath;$InstallDir" }
    $RegKey.SetValue("Path", $NewPath, [Microsoft.Win32.RegistryValueKind]::ExpandString)
    Write-Host "[OK] Added $InstallDir to User PATH." -ForegroundColor Green
}
$RegKey.Close()

# Ensure active session PATH includes InstallDir immediately
$InSessionPath = $false
foreach ($Seg in ($env:PATH -split ';')) {
    if ($Seg.Trim().TrimEnd('\', '/') -ieq $NormTarget) {
        $InSessionPath = $true
        break
    }
}
if (-not $InSessionPath) {
    $env:PATH = "$env:PATH;$InstallDir"
}

Write-Host "[INFO] Done. Run 'babe' or 'babe doctor' from your terminal." -ForegroundColor Cyan
