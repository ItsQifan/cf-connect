# Build the CF-Connect release archive on Windows.
#
# The Makefile's `release-all` target runs a POSIX shell loop; this script is
# the Windows-native equivalent and produces the artifact the team actually
# distributes: cf-connect-<version>-windows-amd64.zip containing the binary plus
# the config template, QUICKSTART, the optional PATH helper and the README.
#
# Usage:
#   pwsh -File .\scripts\release-windows.ps1 [-Version v1.5.1-cf.1]
[CmdletBinding()]
param(
    [string]$Version = '',
    [string]$OutDir = 'dist'
)

$ErrorActionPreference = 'Stop'

$root = Split-Path -Parent $PSScriptRoot
if ([string]::IsNullOrWhiteSpace($root)) { $root = (Get-Location).Path }

# Version: explicit argument, else the Makefile's VERSION, else "dev".
if ([string]::IsNullOrWhiteSpace($Version)) {
    $makefile = Join-Path $root 'Makefile'
    if (Test-Path $makefile) {
        $line = Select-String -Path $makefile -Pattern '^VERSION\s*:?=\s*(.+)$' | Select-Object -First 1
        if ($line) { $Version = $line.Matches[0].Groups[1].Value.Trim() }
    }
}
if ([string]::IsNullOrWhiteSpace($Version)) { $Version = 'dev' }

$commit = 'none'
$buildTime = (Get-Date).ToUniversalTime().ToString('yyyy-MM-ddTHH:mm:ssZ')
try { $commit = (git -C $root rev-parse --short HEAD).Trim() } catch { }

$app = 'cf-connect'
$cmdPath = './cmd/cf-connect'
$goos = 'windows'
$goarch = 'amd64'
$name = "$app-$Version-$goos-$goarch"

$dist = Join-Path $root $OutDir
$stage = Join-Path $dist "stage-$name"
if (Test-Path $stage) { Remove-Item $stage -Recurse -Force }
New-Item -ItemType Directory -Path $stage -Force | Out-Null

Write-Host "Building $name"

# Web assets must be embedded, so web/dist has to exist first.
$distIndex = Join-Path $root 'web\dist\index.html'
if (-not (Test-Path $distIndex)) {
    throw "web/dist is missing. Run 'cd web && pnpm install && pnpm build' first (the admin UI is go:embed-ed into the binary)."
}

$ldflags = "-s -w -X main.version=$Version -X main.commit=$commit -X main.buildTime=$buildTime"
$exe = Join-Path $stage "$app.exe"

$env:GOOS = $goos
$env:GOARCH = $goarch
$env:CGO_ENABLED = '0'
& go build -ldflags $ldflags -o $exe $cmdPath
if ($LASTEXITCODE -ne 0) { throw "go build failed with exit code $LASTEXITCODE" }

foreach ($extra in @('config.example.toml', 'QUICKSTART.md', 'install.ps1', 'README.md')) {
    $src = Join-Path $root $extra
    if (Test-Path $src) {
        Copy-Item $src (Join-Path $stage $extra) -Force
    } else {
        Write-Warning "missing $extra - not packaged"
    }
}

$zipPath = Join-Path $dist "$name.zip"
if (Test-Path $zipPath) { Remove-Item $zipPath -Force }
Compress-Archive -Path (Join-Path $stage '*') -DestinationPath $zipPath -CompressionLevel Optimal
Remove-Item $stage -Recurse -Force

$size = [math]::Round((Get-Item $zipPath).Length / 1MB, 2)
Write-Host "Archive: $zipPath ($size MB)"

$hash = (Get-FileHash $zipPath -Algorithm SHA256).Hash.ToLower()
"$hash  $name.zip" | Set-Content (Join-Path $dist 'checksums.txt') -Encoding ascii
Write-Host "SHA256:  $hash"
