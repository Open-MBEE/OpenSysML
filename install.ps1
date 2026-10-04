<#
.SYNOPSIS
Installs the OpenSysML command-line tools from a GitHub release.

.DESCRIPTION
Windows PowerShell 5.1 or PowerShell 7, on Windows:

    irm https://opensysml.org/install.ps1 | iex

With options, save it first (a piped script takes none; the environment
variables below configure it instead):

    irm https://opensysml.org/install.ps1 -OutFile install.ps1
    .\install.ps1 -Version v0.9.1 -Tools sysml

The release's bundle archive (opensysml-windows-amd64.zip; the tar.gz on Linux
and macOS under PowerShell 7) is downloaded with its SHA256SUMS.txt manifest,
every download is checked against the manifest before anything is installed,
and the installed binaries are run once to confirm they report the release they
came from. When a release carries the Authenticode-signed Windows build
(opensysml-windows-amd64-signed.zip, checked against SHA256SUMS-windows-signed.txt),
that is the one installed.

The Windows installer (opensysml-<x.y.z>-windows-amd64.msi on the release page)
remains the alternative that also bundles the Z3 solver and installs for every
user; this script installs for the current user and touches only the user PATH.

Needs: PowerShell 5.1+ (Windows) or PowerShell 7 (elsewhere, with tar).

.PARAMETER Version
Release tag (v0.9.1), `latest` (default) or `nightly`. [OPENSYSML_VERSION]

.PARAMETER Tools
Subset of sysml, sysml-lsp, sysml-grpc, or `all`; default sysml and sysml-lsp.
[OPENSYSML_TOOLS, comma-separated]

.PARAMETER InstallDir
Where the binaries go; default $env:LOCALAPPDATA\Programs\OpenSysML on Windows,
~/.local/bin elsewhere. [OPENSYSML_INSTALL_DIR]

.PARAMETER Os
Stage another platform's build instead of this machine's (windows, linux or
darwin); it is not run.

.PARAMETER Arch
Architecture of the build (amd64 or arm64); default this machine's.

.PARAMETER BaseUrl
Where the releases are, for a mirror; default
https://github.com/Open-MBEE/OpenSysML/releases. [OPENSYSML_DOWNLOAD_BASE]

.PARAMETER NoPath
Leave the user PATH alone (Windows only; elsewhere PATH is never edited).

.PARAMETER DryRun
Print what would be installed and from where, then exit.

.LINK
https://github.com/Open-MBEE/OpenSysML/blob/main/docs/guide/01-install.md
#>
# Progress is written to the host on purpose: it is for the person running the
# installer, not for a pipeline, and `irm | iex` has no caller to receive output.
[Diagnostics.CodeAnalysis.SuppressMessageAttribute('PSAvoidUsingWriteHost', '')]
[CmdletBinding()]
param(
    [string]$Version = $(if ($env:OPENSYSML_VERSION) { $env:OPENSYSML_VERSION } else { 'latest' }),
    [string[]]$Tools = $(if ($env:OPENSYSML_TOOLS) { $env:OPENSYSML_TOOLS -split ',' } else { @('sysml', 'sysml-lsp') }),
    [string]$InstallDir = $env:OPENSYSML_INSTALL_DIR,
    [ValidateSet('', 'windows', 'linux', 'darwin')]
    [string]$Os = '',
    [ValidateSet('', 'amd64', 'arm64')]
    [string]$Arch = '',
    [string]$BaseUrl = $(if ($env:OPENSYSML_DOWNLOAD_BASE) { $env:OPENSYSML_DOWNLOAD_BASE } else { 'https://github.com/Open-MBEE/OpenSysML/releases' }),
    [switch]$NoPath,
    [switch]$DryRun
)

Set-StrictMode -Version 2.0
$ErrorActionPreference = 'Stop'
# Invoke-WebRequest on Windows PowerShell 5.1 is slow while it draws progress.
$ProgressPreference = 'SilentlyContinue'

$RepoUrl = 'https://github.com/Open-MBEE/OpenSysML'
$GoInstallHint = "'go install github.com/Open-MBEE/OpenSysML/cmd/sysml@latest' builds one for this machine"

# Run as a file the script exits with the message; piped into Invoke-Expression it
# throws instead, since exit there would close the caller's session.
$RunAsFile = [bool]$PSCommandPath
function Fail([string]$Message) {
    if (-not $RunAsFile) { throw "error: $Message" }
    [Console]::Error.WriteLine("error: $Message")
    exit 1
}

# --- what is being installed ------------------------------------------------

switch -Regex ($Version) {
    '^(latest|nightly)$' { break }
    '^v\d+\.\d+\.\d+' { break }
    '^\d+\.\d+\.\d+' { $Version = "v$Version"; break }
    default { Fail "-Version must be a release tag like v0.9.1, 'latest' or 'nightly', not '$Version'" }
}

$Tools = @($Tools | ForEach-Object { $_.Trim() } | Where-Object { $_ })
if ($Tools -contains 'all') { $Tools = @('sysml', 'sysml-lsp', 'sysml-grpc') }
if ($Tools.Count -eq 0) { Fail '-Tools names nothing to install' }
foreach ($tool in $Tools) {
    if ($tool -notin @('sysml', 'sysml-lsp', 'sysml-grpc')) {
        Fail "unknown tool '$tool'; the released tools are sysml, sysml-lsp and sysml-grpc"
    }
}
$WantGrpc = $Tools -contains 'sysml-grpc'

$BaseUrl = $BaseUrl.TrimEnd('/')

# --- the platform -----------------------------------------------------------

# $IsWindows exists only from PowerShell 6; Windows PowerShell is always on Windows.
function Test-PlatformFlag([string]$Name) { [bool](Get-Variable -Name $Name -ValueOnly -ErrorAction SilentlyContinue) }
$HostIsWindows = ($PSVersionTable.PSEdition -eq 'Desktop') -or ($PSVersionTable.PSVersion.Major -lt 6) -or (Test-PlatformFlag IsWindows)
$HostOs = if ($HostIsWindows) { 'windows' } elseif (Test-PlatformFlag IsMacOS) { 'darwin' } elseif (Test-PlatformFlag IsLinux) { 'linux' } else { '' }

$HostArch = ''
try {
    $osArch = [System.Runtime.InteropServices.RuntimeInformation]::OSArchitecture.ToString()
} catch {
    $osArch = $env:PROCESSOR_ARCHITECTURE
}
switch -Regex ($osArch) {
    '^(X64|AMD64)$' { $HostArch = 'amd64' }
    '^(Arm64|ARM64)$' { $HostArch = 'arm64' }
}

if (-not $Os) {
    $Os = $HostOs
    if (-not $Os) { Fail "unsupported operating system: releases are built for Windows, Linux and macOS; see $RepoUrl#install for building from source" }
}
if (-not $Arch) {
    $Arch = $HostArch
    if (-not $Arch) { Fail "unsupported architecture '$osArch': releases are built for amd64 and arm64; $GoInstallHint" }
}
if ($Os -eq 'windows' -and $Arch -ne 'amd64') {
    Fail "no windows/$Arch build is published; $GoInstallHint"
}
$Platform = "$Os-$Arch"
$IsHostPlatform = ($Os -eq $HostOs) -and ($Arch -eq $HostArch)
$Exe = if ($Os -eq 'windows') { '.exe' } else { '' }

# --- where it goes ----------------------------------------------------------

if (-not $InstallDir) {
    if ($HostIsWindows) {
        $InstallDir = Join-Path $env:LOCALAPPDATA 'Programs\OpenSysML'
    } else {
        $InstallDir = Join-Path $HOME '.local/bin'
    }
}

# --- which release ----------------------------------------------------------

if ($BaseUrl -like 'https://*' -or $BaseUrl -like 'http://*') {
    try {
        [Net.ServicePointManager]::SecurityProtocol = [Net.ServicePointManager]::SecurityProtocol -bor [Net.SecurityProtocolType]::Tls12
    } catch {
        $null = $_  # the runtime already speaks TLS 1.2 or later
    }
}

function Resolve-Latest {
    # The redirect GitHub serves for /releases/latest names the tag before anything
    # is downloaded; a mirror without it still works through latest/download/.
    try {
        $request = [System.Net.WebRequest]::Create("$BaseUrl/latest")
        $request.Method = 'HEAD'
        $request.AllowAutoRedirect = $false
        $response = $request.GetResponse()
        try {
            $location = $response.Headers['Location']
        } finally {
            $response.Close()
        }
        if ($location -match '/tag/([^/]+)$') { return $Matches[1] }
    } catch {
        $null = $_  # no redirect to read
    }
    return $null
}

$Tag = $Version
if ($Version -eq 'latest') {
    $Tag = Resolve-Latest
    $AssetDir = if ($Tag) { "$BaseUrl/download/$Tag" } else { "$BaseUrl/latest/download" }
} else {
    $AssetDir = "$BaseUrl/download/$Version"
}
function Get-AssetUrl([string]$Asset) { "$AssetDir/$Asset" }

$ReleaseName = if ($Version -eq 'nightly') { 'the nightly snapshot' } elseif ($Tag) { $Tag } else { 'the latest release' }

# --- download and verify ----------------------------------------------------

$Work = Join-Path ([System.IO.Path]::GetTempPath()) ("opensysml-install-" + [System.IO.Path]::GetRandomFileName())

function Get-Asset([string]$Asset, [switch]$Optional) {
    # Downloads into the work directory; $true when it landed, $false when the
    # release has no such asset and -Optional allowed that.
    $url = Get-AssetUrl $Asset
    $destination = Join-Path $Work $Asset
    try {
        Invoke-WebRequest -UseBasicParsing -Uri $url -OutFile $destination
        return $true
    } catch {
        if (Test-Path -LiteralPath $destination) { Remove-Item -LiteralPath $destination -Force }
        if ($Optional) { return $false }
        Fail "could not download $url ($($_.Exception.Message)); is $ReleaseName a published release with a $Platform build? See $BaseUrl"
    }
}

function Test-Checksum([string]$Asset, [string]$Manifest) {
    $expected = $null
    foreach ($line in Get-Content -LiteralPath (Join-Path $Work $Manifest)) {
        $fields = $line.Trim() -split '\s+', 2
        if ($fields.Count -eq 2 -and $fields[1].TrimStart('*') -eq $Asset) { $expected = $fields[0].ToLowerInvariant(); break }
    }
    if (-not $expected) { Fail "$Manifest of $ReleaseName does not list $Asset; releases before v0.0.4 have no bundle, and sysml-grpc is published from v0.9.0" }
    $actual = (Get-FileHash -LiteralPath (Join-Path $Work $Asset) -Algorithm SHA256).Hash.ToLowerInvariant()
    if ($actual -ne $expected) { Fail "$Asset does not match $Manifest (expected $expected, got $actual); the download is corrupt or tampered with" }
    Write-Host "  $Asset verified"
}

# The signed Windows build is published beside the unsigned one with its own
# manifest; a release without it (and every nightly) has only the unsigned build.
$Manifest = 'SHA256SUMS.txt'
$Bundle = if ($Os -eq 'windows') { "opensysml-$Platform.zip" } else { "opensysml-$Platform.tar.gz" }
$GrpcAsset = "sysml-grpc-$Platform$Exe"

Write-Host "Installing OpenSysML ($ReleaseName) for $Platform"
Write-Host "  tools:    $($Tools -join ' ')"
Write-Host "  binaries: $InstallDir"
Write-Host "  from:     $(Get-AssetUrl $Bundle)"
if ($WantGrpc) { Write-Host "            $(Get-AssetUrl $GrpcAsset)" }
if ($DryRun) {
    Write-Host 'Dry run: nothing downloaded or installed.'
    return
}

New-Item -ItemType Directory -Path $Work -Force | Out-Null
try {
    Write-Host 'Downloading...'
    if ($Os -eq 'windows' -and $Version -ne 'nightly' -and (Get-Asset 'SHA256SUMS-windows-signed.txt' -Optional)) {
        $Manifest = 'SHA256SUMS-windows-signed.txt'
        $Bundle = "opensysml-$Platform-signed.zip"
        $GrpcAsset = "sysml-grpc-$Platform-signed$Exe"
        Write-Host "  the release carries the signed Windows build; installing $Bundle"
    } else {
        Get-Asset $Manifest | Out-Null
    }

    Get-Asset $Bundle | Out-Null
    Test-Checksum $Bundle $Manifest
    if ($WantGrpc) {
        Get-Asset $GrpcAsset | Out-Null
        Test-Checksum $GrpcAsset $Manifest
    }

    # --- install ------------------------------------------------------------

    $Extracted = Join-Path $Work 'bundle'
    New-Item -ItemType Directory -Path $Extracted -Force | Out-Null
    if ($Bundle -like '*.zip') {
        Expand-Archive -LiteralPath (Join-Path $Work $Bundle) -DestinationPath $Extracted -Force
    } else {
        if (-not (Get-Command tar -ErrorAction SilentlyContinue)) { Fail "tar is needed to extract $Bundle" }
        & tar -xzf (Join-Path $Work $Bundle) -C $Extracted
        if ($LASTEXITCODE -ne 0) { Fail "tar could not extract $Bundle" }
    }

    try {
        New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
    } catch {
        Fail "cannot create $InstallDir ($($_.Exception.Message)); pass -InstallDir for a writable location"
    }

    $Installed = @()
    foreach ($tool in $Tools) {
        $source = if ($tool -eq 'sysml-grpc') { Join-Path $Work $GrpcAsset } else { Join-Path $Extracted "$tool$Exe" }
        if (-not (Test-Path -LiteralPath $source -PathType Leaf)) { Fail "$Bundle has no $tool$Exe" }
        $destination = Join-Path $InstallDir "$tool$Exe"
        # Copy beside the destination first, so an interrupted copy never leaves a
        # half-written binary under the final name.
        $staged = "$destination.tmp.$PID"
        Copy-Item -LiteralPath $source -Destination $staged -Force
        if (-not $HostIsWindows) {
            & chmod 0755 $staged
            if ($LASTEXITCODE -ne 0) { Fail "chmod could not mark $staged executable" }
        }
        Move-Item -LiteralPath $staged -Destination $destination -Force
        $Installed += $destination
    }
} finally {
    Remove-Item -LiteralPath $Work -Recurse -Force -ErrorAction SilentlyContinue
}

# --- check ------------------------------------------------------------------

if ($IsHostPlatform) {
    foreach ($tool in $Tools) {
        $binary = Join-Path $InstallDir "$tool$Exe"
        # Windows PowerShell turns a native command's stderr into a terminating
        # error under 'Stop', so the preference is relaxed for the run itself.
        $previous = $ErrorActionPreference
        $ErrorActionPreference = 'Continue'
        try {
            $reported = (& $binary --version 2>&1 | ForEach-Object { "$_" } | Select-Object -First 1) -join ''
            $exit = $LASTEXITCODE
        } finally {
            $ErrorActionPreference = $previous
        }
        if ($exit -ne 0) { Fail "$binary does not run: $reported" }
        if ($Tag -and $Version -ne 'nightly' -and " $reported " -notlike "* $Tag *") {
            Fail "$binary reports '$reported', not $Tag"
        }
        Write-Host "  $reported"
    }
} else {
    Write-Host "  staged for $Platform, not run on this $HostOs-$HostArch machine"
}

Write-Host "Installed: $($Installed -join ' ')"

# --- PATH -------------------------------------------------------------------

$Separator = [System.IO.Path]::PathSeparator
$OnPath = @(($env:PATH -split [regex]::Escape($Separator)) | ForEach-Object { $_.TrimEnd('\', '/') }) -contains $InstallDir.TrimEnd('\', '/')
if ($HostIsWindows -and -not $NoPath) {
    $userPath = [Environment]::GetEnvironmentVariable('Path', 'User')
    $userEntries = @(if ($userPath) { $userPath -split ';' } else { @() }) | ForEach-Object { $_.TrimEnd('\') }
    if ($userEntries -notcontains $InstallDir.TrimEnd('\')) {
        $newPath = if ($userPath) { "$userPath;$InstallDir" } else { $InstallDir }
        [Environment]::SetEnvironmentVariable('Path', $newPath, 'User')
        Write-Host "Added $InstallDir to the user PATH; new terminals will see it."
    }
    if (-not $OnPath) { $env:PATH = "$InstallDir$Separator$env:PATH" }
} elseif (-not $OnPath) {
    Write-Warning "$InstallDir is not on PATH; add it to your profile."
}
foreach ($tool in $Tools) {
    $found = Get-Command "$tool$Exe" -CommandType Application -ErrorAction SilentlyContinue | Select-Object -First 1
    if ($found -and $found.Source -ne (Join-Path $InstallDir "$tool$Exe")) {
        Write-Warning "'$tool' on PATH is $($found.Source), which comes before $(Join-Path $InstallDir "$tool$Exe")"
    }
}
