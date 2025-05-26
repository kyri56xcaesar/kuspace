
# build.ps1

# Entry point
param (
    [string]$Task = ""
)

# Variables
$TARGET_API = "cmd/uspace/"
$API_OUT = "uspace"

$TARGET_WSS = "cmd/wss/"
$WSS_OUT = "ws_registry"

$TARGET_F_APP = "cmd/frontapp/"
$F_APP_OUT = "frontapp"

$TARGET_AUTH = "cmd/minioth/"
$AUTH_OUT = "minioth"



function Build {
    go build -o "$($TARGET_API)$($API_OUT).exe" "$($TARGET_API)main.go"
    go build -o "$($TARGET_F_APP)$($F_APP_OUT).exe" "$($TARGET_F_APP)main.go"
    go build -o "$($TARGET_AUTH)$($AUTH_OUT).exe" "$($TARGET_AUTH)main.go"
    go build -o "$($TARGET_WSS)$($WSS_OUT).exe" "$($TARGET_WSS)main.go"
}

function Run {
    Start-Process -NoNewWindow -FilePath ".\$($TARGET_AUTH)$($AUTH_OUT).exe"
    Start-Sleep -Seconds 2
    Start-Process -NoNewWindow -FilePath ".\$($TARGET_F_APP)$($F_APP_OUT).exe"
    Start-Sleep -Seconds 2
    Start-Process -NoNewWindow -FilePath ".\$($TARGET_API)$($API_OUT).exe"
    Start-Sleep -Seconds 2
    Start-Process -NoNewWindow -FilePath ".\$($TARGET_WSS)$($WSS_OUT).exe"
}

function Stop {
    Get-Process -Name $AUTH_OUT, $API_OUT, $AUTH_OUT, $WSS_OUT, $F_APP_OUT -ErrorAction SilentlyContinue | Stop-Process
}

function uspace {
    go build -o "$($TARGET_API)$($API_OUT).exe" "$($TARGET_API)main.go"
    & "$($TARGET_API)$($API_OUT).exe"
}

function wss {
    go build -o "$($TARGET_WSS)$($WSS_OUT).exe" "$($TARGET_WSS)main.go"
    & "$($TARGET_WSS)$($WSS_OUT).exe"
}

function Front {
    go build -o "$($TARGET_F_APP)$($F_APP_OUT).exe" "$($TARGET_F_APP)main.go"
    & "$($TARGET_F_APP)$($F_APP_OUT).exe"
}

function Minioth {
    go build -o "$($TARGET_AUTH)$($AUTH_OUT).exe" "$($TARGET_AUTH)main.go"
    & "$($TARGET_AUTH)$($AUTH_OUT).exe"
}

function Clean {
    Remove-Item -Force "$($TARGET_API)$($API_OUT).exe", "$($TARGET_AUTH)$($AUTH_OUT).exe", "$($TARGET_F_APP)$($F_APP_OUT).exe", "$($TARGET_WSS)$($WSS_OUT).exe" -ErrorAction SilentlyContinue
}


switch ($Task) {
    "build" { Build }
    "run" { Run }
    "stop" { Stop }
    "uspace" { uspace }
    "wss" { wss }
    "front" { Front }
    "minioth" { Minioth }
    "clean" { Clean }
    default { Write-Output "Available tasks: build, run, stop, uspace, wss, front, minioth, clean" }
}
