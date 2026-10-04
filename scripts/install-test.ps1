#!/usr/bin/env pwsh
# Checks install.ps1 against the throwaway release scripts/install-test.sh serves;
# that script runs this one when pwsh is on PATH, passing the server's addresses
# and the tags it published, so the cases here are the PowerShell side of the same
# fixture: the signed-Windows preference, the checksum gate, the latest redirect
# and the failure messages. Progress goes to the host, as the installer's does,
# and the parameters are read inside the helpers.
[Diagnostics.CodeAnalysis.SuppressMessageAttribute('PSAvoidUsingWriteHost', '')]
[Diagnostics.CodeAnalysis.SuppressMessageAttribute('PSReviewUnusedParameter', '')]
param(
    [Parameter(Mandatory)][string]$Script,
    [Parameter(Mandatory)][string]$BaseUrl,
    [Parameter(Mandatory)][string]$MirrorUrl,
    [Parameter(Mandatory)][string]$HostPlatform,
    [Parameter(Mandatory)][string]$Good,
    [Parameter(Mandatory)][string]$Signed,
    [Parameter(Mandatory)][string]$Tampered
)
Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

$work = Join-Path ([IO.Path]::GetTempPath()) "install-test-ps-$PID"
New-Item -ItemType Directory -Path $work -Force | Out-Null
$script:failures = 0
$script:n = 0
$script:out = ''
$script:status = 0
$script:dir = ''

# Invoke-Installer <args>: install.ps1 into a fresh directory, capturing its output.
function Invoke-Installer {
    $script:n++
    $script:dir = Join-Path $work "install-$script:n"
    $previous = $ErrorActionPreference
    $ErrorActionPreference = 'Continue'
    try {
        $base = if ($args -contains '-BaseUrl') { @() } else { @('-BaseUrl', $BaseUrl) }
        $script:out = (& pwsh -NoProfile -File $Script @base -InstallDir $script:dir @args 2>&1 | ForEach-Object { "$_" }) -join "`n"
        $script:status = $LASTEXITCODE
    } finally {
        $ErrorActionPreference = $previous
    }
}

function Write-Failure([string]$Message) {
    Write-Host "FAIL $Message"
    Write-Host $script:out
    $script:failures++
}

# Test-Ok <name> <args>: the install must succeed.
function Test-Ok([string]$Name) {
    Invoke-Installer @args
    if ($script:status -ne 0) { Write-Failure "${Name}: exit $script:status"; return $false }
    Write-Host "ok   $Name"
    return $true
}

# Test-Failure <name> <message> <args>: the install must fail, saying <message>.
function Test-Failure([string]$Name, [string]$Message) {
    Invoke-Installer @args
    if ($script:status -eq 0 -or -not $script:out.Contains($Message)) {
        Write-Failure "${Name}: exit $script:status, expected a failure mentioning '$Message'"
        return $false
    }
    Write-Host "ok   $Name"
    return $true
}

# Assert-Said <text>: the last run's output must mention it.
function Assert-Said([string]$Text) {
    if (-not $script:out.Contains($Text)) { Write-Failure "expected output to mention '$Text'" }
}

# Assert-Installed <names>: files the last run must have left in its directory.
function Assert-Installed([string[]]$Names) {
    foreach ($name in $Names) {
        if (-not (Test-Path -LiteralPath (Join-Path $script:dir $name) -PathType Leaf)) { Write-Failure "$script:dir/$name missing" }
    }
}

# Assert-Absent <names>: files the last run must not have left in its directory.
function Assert-Absent([string[]]$Names) {
    foreach ($name in $Names) {
        if (Test-Path -LiteralPath (Join-Path $script:dir $name)) { Write-Failure "$script:dir/$name should not exist" }
    }
}

if (Test-Ok 'release by tag' -Version $Good) {
    Assert-Installed sysml, sysml-lsp
    Assert-Absent sysml-grpc
    Assert-Said "Installing OpenSysML ($Good) for $HostPlatform"
    Assert-Said "opensysml-$HostPlatform.tar.gz verified"
    Assert-Said "sysml $Good"
    Assert-Said "sysml-lsp $Good"
    Assert-Said "Installed: $script:dir/sysml $script:dir/sysml-lsp"
}

if (Test-Ok 'tag without the v' -Version $Good.Substring(1)) {
    Assert-Said "Installing OpenSysML ($Good)"
}

if (Test-Ok 'all tools' -Version $Good -Tools all) {
    Assert-Installed sysml, sysml-lsp, sysml-grpc
    Assert-Said "sysml-grpc version $Good"
}

if (Test-Ok 'one tool' -Version $Good -Tools sysml) {
    Assert-Installed sysml
    Assert-Absent sysml-lsp
}

if (Test-Ok 'latest through the redirect') {
    Assert-Said "Installing OpenSysML ($Good)"
    Assert-Said "download/$Good/opensysml-$HostPlatform.tar.gz"
    Assert-Installed sysml, sysml-lsp
}

if (Test-Ok 'latest from a mirror without the redirect' -BaseUrl $MirrorUrl) {
    Assert-Said 'Installing OpenSysML (the latest release)'
    Assert-Said "$MirrorUrl/latest/download/opensysml-$HostPlatform.tar.gz"
    Assert-Installed sysml, sysml-lsp
}

if (Test-Ok 'nightly' -Version nightly) {
    Assert-Said 'Installing OpenSysML (the nightly snapshot)'
    Assert-Said "sysml $Good-5-gabcdef0"
}

if (Test-Ok 'staged for windows, unsigned release' -Version $Good -Os windows) {
    Assert-Installed sysml.exe, sysml-lsp.exe
    Assert-Absent sysml
    Assert-Said 'opensysml-windows-amd64.zip verified'
    Assert-Said "staged for windows-amd64, not run on this $HostPlatform machine"
    if ((Get-Content -LiteralPath (Join-Path $script:dir sysml.exe) -Raw) -notlike "sysml $($Good.Substring(1)) for windows*") {
        Write-Failure 'the unsigned windows build was not the one installed'
    }
}

if (Test-Ok 'staged for windows, signed release' -Version $Signed -Os windows) {
    Assert-Installed sysml.exe, sysml-lsp.exe
    Assert-Said 'the release carries the signed Windows build; installing opensysml-windows-amd64-signed.zip'
    Assert-Said 'opensysml-windows-amd64-signed.zip verified'
    if ((Get-Content -LiteralPath (Join-Path $script:dir sysml.exe) -Raw) -notlike '*, signed*') {
        Write-Failure 'the signed windows build was not the one installed'
    }
}

if (Test-Ok 'dry run' -Version $Good -Tools all -DryRun) {
    Assert-Said 'Dry run: nothing downloaded or installed.'
    Assert-Said "download/$Good/sysml-grpc-$HostPlatform"
    if (Test-Path -LiteralPath $script:dir) { Write-Failure "dry run created $script:dir" }
}

if (Test-Failure 'tampered bundle' "opensysml-$HostPlatform.tar.gz does not match SHA256SUMS.txt" -Version $Tampered) {
    if (Test-Path -LiteralPath $script:dir) { Write-Failure "a tampered release left files under $script:dir" }
}
Test-Failure 'unpublished release' "could not download $BaseUrl/download/v0.0.0/SHA256SUMS.txt" -Version v0.0.0 | Out-Null
Test-Failure 'unpublished windows arm64' 'no windows/arm64 build is published' -Os windows -Arch arm64 | Out-Null
Test-Failure 'bad version' '-Version must be a release tag' -Version main | Out-Null
Test-Failure 'bad tool' "unknown tool 'sysml-repl'" -Tools sysml-repl | Out-Null

Remove-Item -LiteralPath $work -Recurse -Force -ErrorAction SilentlyContinue
if ($script:failures -ne 0) {
    Write-Host "install-test.ps1: $script:failures failure(s)"
    exit 1
}
Write-Host 'install-test.ps1: all cases passed'
