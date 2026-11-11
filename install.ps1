param(
    [switch]$Force
)

$ErrorActionPreference = "Stop"

$Repo = "A7E28/babe"
$ApiUrl = "https://api.github.com/repos/$Repo/releases/latest"

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

Write-Host "--- babe Installer (windows/$Arch) ---" -ForegroundColor Cyan

Write-Host "[INFO] Querying latest release from GitHub..."
$Release = Invoke-RestMethod -Uri $ApiUrl -Headers @{ "User-Agent" = "babe-installer" }
$Tag = $Release.tag_name
$AssetName = "babe-$Tag-windows-$Arch.exe"

$BinAsset = $Release.assets | Where-Object { $_.name -ieq $AssetName } | Select-Object -First 1
if (-not $BinAsset) {
    throw "No release asset found matching $AssetName in release $Tag"
}

$ExpectedHash = ""
$SumAsset = $Release.assets | Where-Object { $_.name -ieq "checksums.txt" } | Select-Object -First 1
if ($SumAsset) {
    $SumContent = Invoke-RestMethod -Uri $SumAsset.browser_download_url -Headers @{ "User-Agent" = "babe-installer" }
    foreach ($Line in ($SumContent -split "`r?`n")) {
        $Parts = $Line.Trim() -split "\s+"
        if ($Parts.Count -ge 2) {
            $FileName = $Parts[1].TrimStart('*')
            if ($FileName -ieq $AssetName) {
                $ExpectedHash = $Parts[0].ToLower()
                break
            }
        }
    }
}

if (-not (Test-Path $InstallDir)) {
    New-Item -ItemType Directory -Force -Path $InstallDir | Out-Null
}

$NeedDownload = $true
if ((Test-Path $TargetExe) -and (-not $Force) -and ($ExpectedHash -ne "")) {
    $CurrentHash = (Get-FileHash -Path $TargetExe -Algorithm SHA256).Hash.ToLower()
    if ($CurrentHash -eq $ExpectedHash) {
        Write-Host "[OK] babe $Tag is already installed and up to date at:" -ForegroundColor Green
        Write-Host "     $TargetExe"
        $NeedDownload = $false
    }
}

if ($NeedDownload) {
    $TempFile = Join-Path $InstallDir ".babe-install.tmp"
    if (Test-Path $TempFile) {
        Remove-Item -Force $TempFile -ErrorAction SilentlyContinue
    }

    Write-Host "[INFO] Downloading $AssetName ($Tag)..."
    Invoke-WebRequest -Uri $BinAsset.browser_download_url -OutFile $TempFile -Headers @{ "User-Agent" = "babe-installer" }

    if ($ExpectedHash -ne "") {
        $ActualHash = (Get-FileHash -Path $TempFile -Algorithm SHA256).Hash.ToLower()
        if ($ActualHash -ne $ExpectedHash) {
            Remove-Item -Force $TempFile -ErrorAction SilentlyContinue
            throw "SHA256 checksum mismatch! Expected $ExpectedHash, got $ActualHash"
        }
        Write-Host "[OK] Verified SHA256: $ActualHash" -ForegroundColor Green
    }

    $OldFile = "$TargetExe.old"
    if (Test-Path $OldFile) {
        Remove-Item -Force $OldFile -ErrorAction SilentlyContinue
    }
    if (Test-Path $TargetExe) {
        Move-Item -Force -Path $TargetExe -Destination $OldFile
    }
    Move-Item -Force -Path $TempFile -Destination $TargetExe
    if (Test-Path $OldFile) {
        Remove-Item -Force $OldFile -ErrorAction SilentlyContinue
    }

    Write-Host "[OK] Installed babe $Tag to $TargetExe" -ForegroundColor Green
}

# Register in HKCU\Environment\Path preserving REG_EXPAND_SZ
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
} else {
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
