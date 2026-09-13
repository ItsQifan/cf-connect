<#
.SYNOPSIS
    把本插件包里的 cf-connect 目录加入用户 PATH。

.DESCRIPTION
    插件包内的可执行文件在 bin\ 下；运行本脚本会把 bin 目录加入*用户* PATH，
    之后新开的终端里可以直接用 cf-connect 命令（钉钉桥接 skill 依赖它）。

    只写 HKCU，不需要管理员权限；用 -Uninstall 可以撤销。

.PARAMETER Uninstall
    把该目录从用户 PATH 中移除。

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

$root = $PSScriptRoot
if ([string]::IsNullOrWhiteSpace($root)) { $root = (Get-Location).Path }
$root = (Resolve-Path -LiteralPath $root).Path.TrimEnd('\')

# 二进制可能在包根，也可能在 bin\ 下
$target = $root
$exe = Join-Path $root 'cf-connect.exe'
if (-not (Test-Path -LiteralPath $exe)) {
    $candidate = Join-Path $root 'bin'
    if (Test-Path -LiteralPath (Join-Path $candidate 'cf-connect.exe')) {
        $target = $candidate
        $exe = Join-Path $candidate 'cf-connect.exe'
    } else {
        Write-Warning "cf-connect.exe not found under $root - unpack the full plugin package first."
    }
}

# 直接读写 HKCU\Environment，而不是走 [Environment]::SetEnvironmentVariable：
# .NET 的 setter 总是写 REG_SZ，会把 REG_EXPAND_SZ 的 PATH 静默降级，
# 导致 PATH 里 %USERPROFILE% 之类的条目不再展开。保留原值类型可以避开这一点。
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
    if (-not $envKey) { throw 'Cannot open HKCU\Environment for writing.' }
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
Write-Host "  1. Open a NEW terminal (the current one does not see the change)."
Write-Host "  2. cd $root"
Write-Host '  3. copy config.example.toml config.toml   then fill in your DingTalk client_id / client_secret'
Write-Host '  4. cf-connect daemon install ; cf-connect daemon start'
Write-Host '  5. cf-connect send -m "bridge ping"     # 钉钉里应该收到这条消息'
Write-Host ''
Write-Host 'See README.md in this directory for the full walkthrough.'
