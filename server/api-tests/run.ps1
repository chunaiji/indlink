param(
    [string]$e = "dev",
    [string]$m = "",
    [string]$f = "",
    [switch]$AllowFlaky
)
$ErrorActionPreference = "Stop"
$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
Set-Location $scriptDir

$envBase = Join-Path $scriptDir "environments\$e.postman_environment.json"
$envLocal = Join-Path $scriptDir "environments\$e.local.json"
$envMerged = Join-Path $scriptDir "environments\$e.merged.json"
if (-not (Test-Path $envBase)) { Write-Error "environment 文件不存在：$envBase"; exit 1 }

$baseObj = Get-Content $envBase -Raw | ConvertFrom-Json
if (Test-Path $envLocal) {
    $localObj = Get-Content $envLocal -Raw | ConvertFrom-Json
    $map = @{}
    foreach ($v in $baseObj.values) { $map[$v.key] = $v }
    foreach ($v in $localObj.values) { $map[$v.key] = $v }
    $baseObj.values = @($map.Values)
}
$baseObj | ConvertTo-Json -Depth 10 | Set-Content $envMerged -Encoding UTF8

if ($m) {
    $collections = @(Join-Path $scriptDir "collections\$m.postman_collection.json")
} else {
    $collections = Get-ChildItem (Join-Path $scriptDir "collections") -Filter "*.postman_collection.json" | ForEach-Object { $_.FullName }
}

$reportDir = Join-Path $scriptDir "reports"
New-Item -ItemType Directory -Force -Path $reportDir | Out-Null

$exitCode = 0
foreach ($c in $collections) {
    $name = [System.IO.Path]::GetFileNameWithoutExtension($c)
    $nargs = @(
        "run", $c, "-e", $envMerged,
        "--reporters", "cli,htmlextra,junit,json",
        "--reporter-htmlextra-export", (Join-Path $reportDir "$name.html"),
        "--reporter-junit-export", (Join-Path $reportDir "$name.junit.xml"),
        "--reporter-json-export", (Join-Path $reportDir "$name.json")
    )
    if ($f) { $nargs += @("--folder", $f) }
    Write-Host ">>> newman $($nargs -join ' ')"
    & newman @nargs
    if ($LASTEXITCODE -ne 0) { $exitCode = $LASTEXITCODE }
}

$extractor = Join-Path $scriptDir "scripts\extract-responses.js"
if (Test-Path $extractor) { Write-Host ">>> node $extractor"; & node $extractor }

Remove-Item $envMerged -ErrorAction SilentlyContinue
exit $exitCode
