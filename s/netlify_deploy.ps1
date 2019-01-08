#!/usr/bin/env pwsh
Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"
function exitIfFailed { if ($LASTEXITCODE -ne 0) { exit } }

$exe = ".\sumatra_website.exe"
$plat = $PSVersionTable["Platform"]
if ($plat = "Unix") {
    $exe = "./sumatra_website"
}
go build -o $exe
exitIfFailed
Start-Process -Wait -FilePath $exe
Remove-Item -Path $exe

# using https://github.com/netlify/cli
netlify deploy --prod --dir www --site "2963982f-7d39-439c-a7eb-0eb118efbd02"
# get-childitem . -include blog_app* | ForEach-Object ($_) {remove-item $_.fullname}
