param(
  [string]$OutputDirectory = ''
)

$projectRoot = (Resolve-Path (Join-Path $PSScriptRoot '..\..')).Path
if ([string]::IsNullOrWhiteSpace($OutputDirectory)) {
  $OutputDirectory = Join-Path $projectRoot 'output'
}
New-Item -ItemType Directory -Force -Path $OutputDirectory | Out-Null
$stamp = Get-Date -Format 'yyyyMMdd-HHmmss'
$packagePath = Join-Path $OutputDirectory "roblox-community-login-policy-$stamp.zip"
$tempRoot = Join-Path ([IO.Path]::GetTempPath()) ("robforum-package-" + [guid]::NewGuid().ToString('N'))
$packageRoot = Join-Path $tempRoot 'roblox-community'
New-Item -ItemType Directory -Force -Path $packageRoot | Out-Null

try {
  $files = & git -C $projectRoot ls-files -co --exclude-standard
  foreach ($relative in $files) {
    if ($relative.Replace('\', '/').StartsWith('frontend/.omo/')) { continue }
    $source = Join-Path $projectRoot $relative
    if (-not (Test-Path -LiteralPath $source -PathType Leaf)) { continue }
    $destination = Join-Path $packageRoot $relative
    New-Item -ItemType Directory -Force -Path (Split-Path -Parent $destination) | Out-Null
    Copy-Item -LiteralPath $source -Destination $destination
  }

  $distDestination = Join-Path $packageRoot 'frontend\dist'
  New-Item -ItemType Directory -Force -Path $distDestination | Out-Null
  Copy-Item -Path (Join-Path $projectRoot 'frontend\dist\*') -Destination $distDestination -Recurse -Force

  $binDirectory = Join-Path $packageRoot 'bin'
  New-Item -ItemType Directory -Force -Path $binDirectory | Out-Null
  $previousGOOS = $env:GOOS
  $previousGOARCH = $env:GOARCH
  $previousCGO = $env:CGO_ENABLED
  try {
    $env:GOOS = 'linux'
    $env:GOARCH = 'amd64'
    $env:CGO_ENABLED = '0'
    & go -C $projectRoot build -o (Join-Path $binDirectory 'roblox-community-linux-amd64') ./cmd/server
    if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
  } finally {
    $env:GOOS = $previousGOOS
    $env:GOARCH = $previousGOARCH
    $env:CGO_ENABLED = $previousCGO
  }

  Compress-Archive -LiteralPath $packageRoot -DestinationPath $packagePath -CompressionLevel Optimal
  $item = Get-Item -LiteralPath $packagePath
  $hash = Get-FileHash -LiteralPath $packagePath -Algorithm SHA256
  [pscustomobject]@{ Path = $item.FullName; Bytes = $item.Length; SHA256 = $hash.Hash }
} finally {
  $resolvedTemp = [IO.Path]::GetFullPath($tempRoot)
  $tempBase = [IO.Path]::GetFullPath([IO.Path]::GetTempPath())
  if ($resolvedTemp.StartsWith($tempBase, [StringComparison]::OrdinalIgnoreCase) -and (Test-Path -LiteralPath $resolvedTemp)) {
    Remove-Item -LiteralPath $resolvedTemp -Recurse -Force
  }
}
