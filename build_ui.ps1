$ErrorActionPreference = "Stop"

Write-Host "Building admin_access.html..."

$cssPath = ".\ui_src\admin.css"
$htmlPath = ".\ui_src\admin.html"
$jsPath = ".\ui_src\admin.js"
$outPath = ".\internal\httpapi\admin_access.html"

if (-Not (Test-Path $cssPath)) { throw "Missing $cssPath" }
if (-Not (Test-Path $htmlPath)) { throw "Missing $htmlPath" }
if (-Not (Test-Path $jsPath)) { throw "Missing $jsPath" }

$cssContent = Get-Content -Path $cssPath -Raw -Encoding UTF8
$htmlContent = Get-Content -Path $htmlPath -Raw -Encoding UTF8
$jsContent = Get-Content -Path $jsPath -Raw -Encoding UTF8

$template = @"
<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>GrokPi Admin Dashboard</title>
  <link rel="preconnect" href="https://fonts.googleapis.com">
  <link rel="preconnect" href="https://fonts.gstatic.com" crossorigin>
  <link href="https://fonts.googleapis.com/css2?family=Inter:wght@400;500;600&family=JetBrains+Mono:wght@400;500&display=swap" rel="stylesheet">
  <style>
$cssContent
  </style>
</head>
<body>
$htmlContent
<script>
$jsContent
</script>
</body>
</html>
"@

Set-Content -Path $outPath -Value $template -Encoding UTF8

Write-Host "Successfully built: $outPath"
