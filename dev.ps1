# Run Go server (:8080) and Vite dev server (:5173) in parallel on Windows.
# POSIX-shell users (Linux/macOS/Git Bash) can use `make dev` instead.

$ErrorActionPreference = 'Stop'

$root = $PSScriptRoot
$webDir = Join-Path $root 'web'

if (-not (Test-Path (Join-Path $webDir 'node_modules'))) {
    Write-Host 'web/node_modules missing; running npm install...' -ForegroundColor Yellow
    Push-Location $webDir
    try { npm install --no-audit --no-fund } finally { Pop-Location }
}

Write-Host ''
Write-Host 'Starting Go server (:8080) and Vite dev server (:5173).' -ForegroundColor Cyan
Write-Host 'Press Ctrl+C to stop both.' -ForegroundColor Cyan
Write-Host ''

$go = Start-Job -Name 'yafu-go' -ScriptBlock {
    param($dir)
    Set-Location $dir
    & go run ./cmd/yafu --log-level debug 2>&1
} -ArgumentList $root

$web = Start-Job -Name 'yafu-web' -ScriptBlock {
    param($dir)
    Set-Location $dir
    & npm run dev 2>&1
} -ArgumentList $webDir

try {
    while ($true) {
        $go | Receive-Job | ForEach-Object {
            Write-Host "[go]  $_" -ForegroundColor Cyan
        }
        $web | Receive-Job | ForEach-Object {
            Write-Host "[web] $_" -ForegroundColor Magenta
        }
        if ($go.State -ne 'Running' -and $web.State -ne 'Running') { break }
        Start-Sleep -Milliseconds 200
    }
} finally {
    Write-Host ''
    Write-Host 'Stopping jobs...' -ForegroundColor Yellow
    Stop-Job $go, $web -ErrorAction SilentlyContinue
    Receive-Job $go, $web -ErrorAction SilentlyContinue | Out-Null
    Remove-Job $go, $web -Force -ErrorAction SilentlyContinue
}
