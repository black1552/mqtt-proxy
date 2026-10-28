# build.ps1 — 一键构建：编译 AntD/React 控制台并将其内嵌到 Go 二进制中，
# 产出单个自包含的 mqtt-proxy(.exe)。
# 用 Continue（而非 Stop）：原生工具链会向 stderr 写入无害的警告，
# 否则 PowerShell 会将其当作终止性错误。下方通过 $LASTEXITCODE 显式捕获失败。
$ErrorActionPreference = "Continue"
$root = $PSScriptRoot

Write-Host "==> building frontend (admin/)" -ForegroundColor Cyan
Push-Location (Join-Path $root "admin")
if (-not (Test-Path "node_modules")) { npm install }
npm run build
if ($LASTEXITCODE -ne 0) { Pop-Location; throw "frontend build failed" }
Pop-Location

Write-Host "==> building backend binary" -ForegroundColor Cyan
Push-Location $root
$exe = "mqtt-proxy"
# 用 Go 工具链的 GOOS 判断目标平台，不依赖宿主环境变量（$IsWindows 仅 PS7+ 提供）。
if ((go env GOOS) -eq "windows") { $exe = "$exe.exe" }
$env:CGO_ENABLED = "0"
go build -trimpath -ldflags "-s -w" -o $exe .
if ($LASTEXITCODE -ne 0) { Pop-Location; throw "go build failed" }
Pop-Location

Write-Host "==> done: $root\$exe" -ForegroundColor Green
