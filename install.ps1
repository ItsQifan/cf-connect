<#
.SYNOPSIS
    Add the CF-Connect directory to your user PATH.

.DESCRIPTION
    Optional convenience script shipped in the release archive. Running it adds
    the directory containing this script to the *user* PATH so that
    `cf-connect` can be invoked from any newly opened terminal.

    It writes only to HKCU (no administrator rights needed) and can be undone
    with -Uninstall.

.PARAMETER Uninstall
    Remove the directory from the user PATH again.

.EXAMPLE
    powershell -ExecutionPolicy Bypass -File .\install.ps1

.EXAMPLE
    powershell -ExecutionPolicy Bypass -File .\install.ps1 -Uninstall
#>
[CmdletBinding()]
param(
    [switch]$Uninstall
)

$ErrorActionPreference = 'Stop'

$target = $PSScriptRoot
if ([string]::IsNullOrWhiteSpace($target)) {
    $target = (Get-Location).Path
}
$target = (Resolve-Path -LiteralPath $target).Path.TrimEnd('\')

$exe = Join-Path $target 'cf-connect.exe'
if (-not (Test-Path -LiteralPath $exe)) {
    Write-Warning "cf-connect.exe not found in $target - run this script from the unpacked archive directory."
}

# Read and write HKCU\Environment directly instead of going through
# [Environment]::SetEnvironmentVariable. The .NET setter always writes REG_SZ,
# so it silently downgrades a REG_EXPAND_SZ PATH and any %USERPROFILE%-style
# entry in it stops expanding - i.e. it can break the user's PATH. Preserving
# the stored value kind avoids that entirely.
$envKey = [Microsoft.Win32.Registry]::CurrentUser.OpenSubKey('Environment', $true)
if (-not $envKey) {
    throw 'Cannot open HKCU\Environment for reading. User PATH was not changed.'
}

function Get-UserPath {
    if (-not $envKey) { return @() }
    $value = [string]$envKey.GetValue('PATH', '', 'DoNotExpandEnvironmentNames')
    if ([string]::IsNullOrWhiteSpace($value)) { return @() }
    return @($value -split ';' | Where-Object { $_ -ne '' })
}

function Set-UserPath([string[]]$entries) {
    if (-not $envKey) {
        throw 'Cannot open HKCU\Environment for writing.'
    }
    # Default to ExpandString when the value does not exist yet; that is the
    # kind Windows itself uses for the user PATH.
    $kind = [Microsoft.Win32.RegistryValueKind]::ExpandString
    if ($envKey.GetValueNames() -contains 'PATH') {
        $kind = $envKey.GetValueKind('PATH')
    }
    $envKey.SetValue('PATH', ($entries -join ';'), $kind)
}

$current = Get-UserPath

if ($Uninstall) {
    $updated = @($current | Where-Object { $_.TrimEnd('\') -ne $target })
    if ($updated.Count -eq $current.Count) {
        Write-Host "Not present in user PATH: $target"
        return
    }
    Set-UserPath $updated
    Write-Host "Removed from user PATH: $target"
    Write-Host 'Open a new terminal for the change to take effect.'
    return
}

if ($current | Where-Object { $_.TrimEnd('\') -eq $target }) {
    Write-Host "Already in user PATH: $target"
} else {
    Set-UserPath (@($target) + $current)
    Write-Host "Added to user PATH: $target"
}

Write-Host ''
Write-Host 'Next steps:'
Write-Host '  1. Open a NEW terminal (the current one does not see the change).'
Write-Host '  2. cd to this directory and run:  copy config.example.toml config.toml'
Write-Host '  3. Fill in your DingTalk client_id / client_secret, then run:  cf-connect'
Write-Host ''
Write-Host "See QUICKSTART.md in $target for the full walkthrough."
