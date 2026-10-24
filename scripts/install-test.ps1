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
    [Parameter(Mandatory)][string]$Tampered,
    [Parameter(Mandatory)][string]$Unlisted,
    [Parameter(Mandatory)][string]$NoKernel,
    [Parameter(Mandatory)][string]$NoGrpc,
    [Parameter(Mandatory)][string]$Repeated,
    [Parameter(Mandatory)][string]$Conflicting
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

# Assert-Unsaid <text>: the last run's output must not mention it.
function Assert-Unsaid([string]$Text) {
    if ($script:out.Contains($Text)) { Write-Failure "expected output not to mention '$Text'" }
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
    Assert-Absent sysml-grpc, sysml-jupyter-kernel, sysml-mcp
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
    Assert-Installed sysml, sysml-lsp, sysml-grpc, sysml-jupyter-kernel, sysml-mcp
    Assert-Said 'tools:    sysml sysml-lsp sysml-grpc sysml-jupyter-kernel sysml-mcp'
    Assert-Unsaid '-Tools all:'
    Assert-Said "sysml-grpc-$HostPlatform verified"
    Assert-Said "sysml-grpc version $Good"
    Assert-Said "sysml-jupyter-kernel-$HostPlatform verified"
    Assert-Said "sysml-jupyter-kernel version $Good"
    Assert-Said "sysml-mcp-$HostPlatform.tar.gz verified"
    Assert-Said "sysml-mcp $Good"
    Assert-Said "Installed: $script:dir/sysml $script:dir/sysml-lsp $script:dir/sysml-grpc $script:dir/sysml-jupyter-kernel $script:dir/sysml-mcp"
}

# `all` means what the chosen release published, read from its manifest.
if (Test-Ok 'all tools of a release without the kernel' -Version $NoKernel -Tools all) {
    Assert-Installed sysml, sysml-lsp, sysml-grpc
    Assert-Absent sysml-jupyter-kernel
    Assert-Said "-Tools all: $NoKernel publishes no sysml-jupyter-kernel for $HostPlatform"
    Assert-Said 'tools:    sysml sysml-lsp sysml-grpc sysml-mcp'
    Assert-Unsaid "sysml-jupyter-kernel-$HostPlatform"
    Assert-Said "sysml-grpc version $NoKernel"
    Assert-Said "Installed: $script:dir/sysml $script:dir/sysml-lsp $script:dir/sysml-grpc $script:dir/sysml-mcp"
}

if (Test-Ok 'all tools of a release without sysml-grpc' -Version $NoGrpc -Tools all) {
    Assert-Installed sysml, sysml-lsp
    Assert-Absent sysml-grpc, sysml-jupyter-kernel, sysml-mcp
    Assert-Said "-Tools all: $NoGrpc publishes no sysml-grpc or sysml-jupyter-kernel or sysml-mcp for $HostPlatform"
    Assert-Said 'tools:    sysml sysml-lsp'
    Assert-Said "Installed: $script:dir/sysml $script:dir/sysml-lsp"
}

if (Test-Ok 'all tools of a release whose manifest omits assets' -Version $Unlisted -Tools all) {
    Assert-Installed sysml, sysml-lsp, sysml-mcp
    Assert-Absent sysml-grpc, sysml-jupyter-kernel
    Assert-Said "-Tools all: $Unlisted publishes no sysml-grpc or sysml-jupyter-kernel for $HostPlatform"
}

if (Test-Ok 'all tools of latest' -Tools all) {
    Assert-Said "Installing OpenSysML ($Good)"
    Assert-Installed sysml, sysml-lsp, sysml-grpc, sysml-jupyter-kernel, sysml-mcp
}

if (Test-Ok 'all tools of the nightly' -Version nightly -Tools all) {
    Assert-Said 'Installing OpenSysML (the nightly snapshot)'
    Assert-Installed sysml, sysml-lsp, sysml-grpc, sysml-jupyter-kernel, sysml-mcp
}

if (Test-Ok 'all tools staged for windows, from a release without the kernel' -Version $NoKernel -Os windows -Tools all) {
    Assert-Installed sysml.exe, sysml-lsp.exe, sysml-grpc.exe, sysml-mcp.exe
    Assert-Absent sysml-jupyter-kernel.exe
    Assert-Said "-Tools all: $NoKernel publishes no sysml-jupyter-kernel for windows-amd64"
}

if (Test-Ok 'the kernel alone' -Version $Good -Tools sysml-jupyter-kernel) {
    Assert-Installed sysml-jupyter-kernel
    Assert-Absent sysml, sysml-lsp, sysml-grpc, sysml-mcp
    Assert-Said "download/$Good/sysml-jupyter-kernel-$HostPlatform"
    Assert-Unsaid "opensysml-$HostPlatform.tar.gz"
    Assert-Said "Installed: $script:dir/sysml-jupyter-kernel"
}

# A tool published on its own does not pull the bundle in.
if (Test-Ok 'sysml-grpc alone' -Version $Good -Tools sysml-grpc) {
    Assert-Installed sysml-grpc
    Assert-Absent sysml, sysml-lsp, sysml-jupyter-kernel
    Assert-Unsaid "opensysml-$HostPlatform.tar.gz"
    Assert-Said "  from:     $BaseUrl/download/$Good/sysml-grpc-$HostPlatform"
    Assert-Said "sysml-grpc-$HostPlatform verified"
    Assert-Said "Installed: $script:dir/sysml-grpc"
}

# sysml-mcp comes from its own archive.
if (Test-Ok 'the MCP server alone' -Version $Good -Tools sysml-mcp) {
    Assert-Installed sysml-mcp
    Assert-Absent sysml, sysml-lsp, sysml-grpc, sysml-jupyter-kernel
    Assert-Unsaid "opensysml-$HostPlatform.tar.gz"
    Assert-Said "download/$Good/sysml-mcp-$HostPlatform.tar.gz"
    Assert-Said "sysml-mcp-$HostPlatform.tar.gz verified"
    Assert-Said "sysml-mcp $Good"
    Assert-Said "Installed: $script:dir/sysml-mcp"
}

if (Test-Ok 'MCP server staged for windows' -Version $Good -Os windows -Tools sysml-mcp) {
    Assert-Installed sysml-mcp.exe
    Assert-Absent sysml.exe, sysml-lsp.exe, sysml-mcp
    Assert-Said "download/$Good/sysml-mcp-windows-amd64.zip"
    Assert-Said 'sysml-mcp-windows-amd64.zip verified'
    Assert-Said "Installed: $(Join-Path $script:dir sysml-mcp.exe)"
    if ((Get-Content -LiteralPath (Join-Path $script:dir sysml-mcp.exe) -Raw) -notlike "sysml-mcp $($Good.Substring(1)) for windows*") {
        Write-Failure 'the windows MCP server was not the one installed'
    }
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

if (Test-Ok 'kernel staged for windows' -Version $Good -Os windows -Tools sysml-jupyter-kernel) {
    Assert-Installed sysml-jupyter-kernel.exe
    Assert-Absent sysml.exe, sysml-lsp.exe, sysml-grpc.exe, sysml-mcp.exe, sysml-jupyter-kernel
    Assert-Said "download/$Good/sysml-jupyter-kernel-windows-amd64.exe"
    Assert-Unsaid 'opensysml-windows-amd64.zip'
    Assert-Said 'sysml-jupyter-kernel-windows-amd64.exe verified'
    Assert-Said "Installed: $(Join-Path $script:dir sysml-jupyter-kernel.exe)"
    if ((Get-Content -LiteralPath (Join-Path $script:dir sysml-jupyter-kernel.exe) -Raw) -notlike "sysml-jupyter-kernel $($Good.Substring(1)) for windows*") {
        Write-Failure 'the windows kernel was not the one installed'
    }
}

# The fixture's "signed" executables carry no Authenticode signature, so the
# signed build can only be observed being declined from a non-Windows host; on
# Windows the installer would pick it and reject the unsigned bytes.
$HostIsWindows = ($PSVersionTable.PSEdition -eq 'Desktop') -or [bool](Get-Variable IsWindows -ValueOnly -ErrorAction SilentlyContinue)
if (-not $HostIsWindows) {
    if (Test-Ok 'staged for windows, signed release, from a non-Windows host' -Version $Signed -Os windows) {
        Assert-Said 'the signed Windows build is not chosen from'
        Assert-Said 'where its Authenticode signatures cannot be checked'
        Assert-Said 'opensysml-windows-amd64.zip verified'
        if ((Get-Content -LiteralPath (Join-Path $script:dir sysml.exe) -Raw) -like '*, signed*') {
            Write-Failure 'the signed windows build was installed without its signatures being checked'
        }
    }

    if (Test-Ok 'dry run of the signed release, from a non-Windows host' -Version $Signed -Os windows -DryRun) {
        Assert-Said 'the signed Windows build is not chosen from'
        Assert-Said "download/$Signed/opensysml-windows-amd64.zip"
        Assert-Said 'Dry run: nothing installed.'
        if (Test-Path -LiteralPath $script:dir) { Write-Failure "dry run created $script:dir" }
    }

    if (Test-Ok 'kernel from the signed release, from a non-Windows host' -Version $Signed -Os windows -Tools sysml-jupyter-kernel) {
        Assert-Said 'the signed Windows build is not chosen from'
        Assert-Said "download/$Signed/sysml-jupyter-kernel-windows-amd64.exe"
        Assert-Said 'sysml-jupyter-kernel-windows-amd64.exe verified'
        Assert-Installed sysml-jupyter-kernel.exe
    }
} else {
    if (Test-Failure 'staged for windows, signed release, unsigned bytes' 'the Authenticode signature of sysml.exe is' -Version $Signed -Os windows) {
        Assert-Said 'the release carries the signed Windows build'
        Assert-Said 'installing opensysml-windows-amd64-signed.zip'
        Assert-Said 'opensysml-windows-amd64-signed.zip verified'
        Assert-Absent sysml.exe, sysml-lsp.exe
    }

    if (Test-Ok 'dry run of the signed release' -Version $Signed -Os windows -DryRun) {
        Assert-Said 'the release carries the signed Windows build'
        Assert-Said 'installing opensysml-windows-amd64-signed.zip'
        Assert-Said "download/$Signed/opensysml-windows-amd64-signed.zip"
        Assert-Said 'Dry run: nothing installed.'
        if (Test-Path -LiteralPath $script:dir) { Write-Failure "dry run created $script:dir" }
    }

    # The signed build carries no kernel, so the kernel comes from the unsigned
    # release and is checked against SHA256SUMS.txt; alone it needs no Authenticode.
    if (Test-Ok 'kernel from the signed release' -Version $Signed -Os windows -Tools sysml-jupyter-kernel) {
        Assert-Said 'the release carries the signed Windows build'
        Assert-Said 'none of the requested tools is among its assets'
        Assert-Unsaid 'installing opensysml-windows-amd64-signed.zip'
        Assert-Said 'sysml-jupyter-kernel is not among the signed assets'
        Assert-Said "download/$Signed/sysml-jupyter-kernel-windows-amd64.exe"
        Assert-Said 'sysml-jupyter-kernel-windows-amd64.exe verified'
        Assert-Installed sysml-jupyter-kernel.exe
    }
}

if (Test-Ok 'dry run' -Version $Good -Tools all -DryRun) {
    Assert-Said 'Dry run: nothing installed.'
    Assert-Said 'tools:    sysml sysml-lsp sysml-grpc sysml-jupyter-kernel sysml-mcp'
    Assert-Said "download/$Good/sysml-grpc-$HostPlatform"
    Assert-Said "download/$Good/sysml-jupyter-kernel-$HostPlatform"
    Assert-Said "download/$Good/sysml-mcp-$HostPlatform.tar.gz"
    if (Test-Path -LiteralPath $script:dir) { Write-Failure "dry run created $script:dir" }
}

if (Test-Ok 'dry run of all tools of a release without the kernel' -Version $NoKernel -Tools all -DryRun) {
    Assert-Said 'Dry run: nothing installed.'
    Assert-Said "-Tools all: $NoKernel publishes no sysml-jupyter-kernel for $HostPlatform"
    Assert-Said 'tools:    sysml sysml-lsp sysml-grpc sysml-mcp'
    Assert-Said "download/$NoKernel/sysml-grpc-$HostPlatform"
    Assert-Unsaid "sysml-jupyter-kernel-$HostPlatform"
    if (Test-Path -LiteralPath $script:dir) { Write-Failure "dry run created $script:dir" }
}

if (Test-Failure 'tampered bundle' "opensysml-$HostPlatform.tar.gz does not match SHA256SUMS.txt" -Version $Tampered) {
    if (Test-Path -LiteralPath $script:dir) { Write-Failure "a tampered release left files under $script:dir" }
}
# One digest listed twice is one digest; two digests for one asset is no manifest to trust.
if (Test-Ok "manifest repeating the bundle's line" -Version $Repeated -Tools all) {
    Assert-Installed sysml, sysml-lsp, sysml-grpc, sysml-jupyter-kernel, sysml-mcp
    Assert-Said "opensysml-$HostPlatform.tar.gz verified"
}
if (Test-Failure 'manifest listing the bundle with two digests' "lists opensysml-$HostPlatform.tar.gz with more than one digest" -Version $Conflicting) {
    Assert-Unsaid 'does not match'
    if (Test-Path -LiteralPath $script:dir) { Write-Failure "a conflicting manifest left files under $script:dir" }
}
Test-Failure 'asset the manifest omits' "does not list sysml-grpc-$HostPlatform" -Version $Unlisted -Tools sysml-grpc | Out-Null
if (Test-Failure 'kernel the manifest omits' "does not list sysml-jupyter-kernel-$HostPlatform" -Version $Unlisted -Tools sysml-jupyter-kernel) {
    Assert-Said 'sysml-jupyter-kernel from v0.10.0'
}
if (Test-Failure 'unpublished kernel' "could not download $BaseUrl/download/$NoKernel/sysml-jupyter-kernel-$HostPlatform" -Version $NoKernel -Tools sysml-jupyter-kernel) {
    if (Test-Path -LiteralPath $script:dir) { Write-Failure "a release without the kernel left files under $script:dir" }
}
if (Test-Failure 'unpublished sysml-grpc' "could not download $BaseUrl/download/$NoGrpc/sysml-grpc-$HostPlatform" -Version $NoGrpc -Tools 'sysml-grpc,sysml') {
    if (Test-Path -LiteralPath $script:dir) { Write-Failure "a release without sysml-grpc left files under $script:dir" }
}
Test-Failure 'unpublished release' "could not download $BaseUrl/download/v0.0.0/SHA256SUMS.txt" -Version v0.0.0 | Out-Null
Test-Failure 'unpublished windows arm64' 'no windows/arm64 build is published' -Os windows -Arch arm64 | Out-Null
Test-Failure 'bad version' '-Version must be a release tag' -Version main | Out-Null
if (Test-Failure 'bad tool' "unknown tool 'sysml-repl'" -Tools sysml-repl) {
    Assert-Said 'the released tools are sysml, sysml-lsp, sysml-grpc, sysml-jupyter-kernel and sysml-mcp'
}
Test-Failure 'all beside a tool' "'all' names every tool the release published and stands alone in -Tools" -Tools 'all,sysml' | Out-Null
Test-Failure 'all with a stray space' "unknown tool 'a ll'" -Tools 'a ll' | Out-Null

Remove-Item -LiteralPath $work -Recurse -Force -ErrorAction SilentlyContinue
if ($script:failures -ne 0) {
    Write-Host "install-test.ps1: $script:failures failure(s)"
    exit 1
}
Write-Host 'install-test.ps1: all cases passed'
