
# build.ps1

# Entry point
param (
    [string]$Task = ""
)

# Variables
$TARGET_API = "cmd/uspace/"
$API_OUT = "uspace"
$API_MAIN = "main.go"

$TARGET_J_WS = "cmd/uspace/jobs_feedback_ws/"
$J_WS_OUT = "j_ws"
$J_WS_MAIN = "main.go"

$TARGET_F_APP = "cmd/frontendapp/"
$F_APP_OUT = "frontendapp"
$F_APP_MAIN = "main.go"

$TARGET_WS = "cmd/frontendapp/ws/"
$WS_OUT = "ws_server"
$WS_MAIN = "ws_main.go"

$TARGET_AUTH = "cmd/minioth/"
$AUTH_OUT = "minioth"
$AUTH_MAIN = "main.go"

$TARGET_SHELL = "cmd/shell/"
$SHELL_OUT = "gshell"
$SHELL_MAIN = "main.go"

function Mod {
    go mod tidy
}

function Build {
    go build -o "$($TARGET_API)$($API_OUT).exe" "$($TARGET_API)$($API_MAIN)"
    go build -o "$($TARGET_F_APP)$($F_APP_OUT).exe" "$($TARGET_F_APP)$($F_APP_MAIN)"
    go build -o "$($TARGET_AUTH)$($AUTH_OUT).exe" "$($TARGET_AUTH)$($AUTH_MAIN)"
    go build -o "$($TARGET_WS)$($WS_OUT).exe" "$($TARGET_WS)$($WS_MAIN)"
    go build -o "$($TARGET_J_WS)$($J_WS_OUT).exe" "$($TARGET_J_WS)$($J_WS_MAIN)"
}

function Run {
    Start-Process -NoNewWindow -FilePath ".\$($TARGET_AUTH)$($AUTH_OUT).exe"
    Start-Sleep -Seconds 2
    Start-Process -NoNewWindow -FilePath ".\$($TARGET_F_APP)$($F_APP_OUT).exe"
    Start-Sleep -Seconds 2
    Start-Process -NoNewWindow -FilePath ".\$($TARGET_API)$($API_OUT).exe"
    Start-Sleep -Seconds 2
    Start-Process -NoNewWindow -FilePath ".\$($TARGET_WS)$($WS_OUT).exe"
    Start-Sleep -Seconds 2
    Start-Process -NoNewWindow -FilePath ".\$($TARGET_J_WS)$($J_WS_OUT).exe"
}

function Stop {
    Get-Process -Name $AUTH_OUT, $WS_OUT, $API_OUT, $AUTH_OUT, $J_WS_OUT, $F_APP_OUT -ErrorAction SilentlyContinue | Stop-Process
}

function uspace {
    go build -o "$($TARGET_API)$($API_OUT).exe" "$($TARGET_API)$($API_MAIN)"
    & "$($TARGET_API)$($API_OUT).exe"
}

function J_ws {
    go build -o "$($TARGET_J_WS)$($J_WS_OUT).exe" "$($TARGET_J_WS)$($J_WS_MAIN)"
    & "$($TARGET_J_WS)$($J_WS_OUT).exe"
}

function Front {
    go build -o "$($TARGET_F_APP)$($F_APP_OUT).exe" "$($TARGET_F_APP)$($F_APP_MAIN)"
    & "$($TARGET_F_APP)$($F_APP_OUT).exe"
}

function Front_ws {
    go build -o "$($TARGET_WS)$($WS_OUT).exe" "$($TARGET_WS)$($WS_MAIN)"
    & "$($TARGET_WS)$($WS_OUT).exe"
}

function Minioth {
    go build -o "$($TARGET_AUTH)$($AUTH_OUT).exe" "$($TARGET_AUTH)$($AUTH_MAIN)"
    & "$($TARGET_AUTH)$($AUTH_OUT).exe"
}

function Shell {
    go build -o "$($TARGET_SHELL)$($SHELL_OUT).exe" "$($TARGET_SHELL)$($SHELL_MAIN)"
    & "$($TARGET_SHELL)$($SHELL_OUT).exe"
}

function Clean {
    Remove-Item -Force "$($TARGET_API)$($API_OUT).exe", "$($TARGET_SHELL)$($SHELL_OUT).exe", "$($TARGET_AUTH)$($AUTH_OUT).exe", "$($TARGET_F_APP)$($F_APP_OUT).exe", "$($TARGET_WS)$($WS_OUT).exe", "$($TARGET_J_WS)$($J_WS_OUT).exe" -ErrorAction SilentlyContinue
}


switch ($Task) {
    "mod" { Mod }
    "build" { Build }
    "run" { Run }
    "stop" { Stop }
    "uspace" { uspace }
    "j_ws" { J_ws }
    "front" { Front }
    "front-ws" { Front_ws }
    "minioth" { Minioth }
    "shell" { Shell }
    "clean" { Clean }
    default { Write-Output "Available tasks: mod, build, run, stop, uspace, j_ws, front, front-ws, minioth, shell, clean" }
}
