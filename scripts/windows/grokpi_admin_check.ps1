param(
    [string]$AppKey = ""
)

$ApiBase = if ($env:GROKPI_ADMIN_BASE_URL) { $env:GROKPI_ADMIN_BASE_URL } else { "http://127.0.0.1:8080" }

if ($AppKey -eq "") {
    $secureKey = Read-Host "Enter App Key (Admin Password)" -AsSecureString
    $AppKey = [System.Net.NetworkCredential]::new("", $secureKey).Password
}

if ([string]::IsNullOrWhiteSpace($AppKey)) {
    Write-Host "APP_KEY is required" -ForegroundColor Red
    exit 1
}

Write-Host "== POST /admin/login =="
try {
    $loginBody = @{ key = $AppKey } | ConvertTo-Json -Compress
    $loginRes = Invoke-RestMethod -Uri "$ApiBase/admin/login" -Method Post -ContentType "application/json" -Body $loginBody -SessionVariable adminSession
    $loginRes | ConvertTo-Json -Depth 5 | Write-Host
} catch {
    $resp = $_.ErrorDetails.Message
    if ([string]::IsNullOrWhiteSpace($resp)) {
        $resp = $_.Exception.Message
    }
    Write-Host "login failed: $resp" -ForegroundColor Red
    exit 1
}

Write-Host "`n== GET /admin/verify =="
try {
    $verifyRes = Invoke-RestMethod -Uri "$ApiBase/admin/verify" -Method Get -WebSession $adminSession -Headers @{ "Authorization" = "Bearer $AppKey" }
    $verifyRes | ConvertTo-Json -Depth 5 | Write-Host
    Write-Host "admin auth check: ok" -ForegroundColor Green
    exit 0
} catch {
    $resp = $_.ErrorDetails.Message
    if ([string]::IsNullOrWhiteSpace($resp)) {
        $resp = $_.Exception.Message
    }
    Write-Host $resp
    Write-Host "admin auth check: failed" -ForegroundColor Red
    exit 1
}
