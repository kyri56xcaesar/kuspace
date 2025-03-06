$ErrorActionPreference = "Stop"

$TARGET_API = "cmd\userspace\"
$API_OUT = "userspace"
$API_MAIN = "main.go"

$TARGET_F_APP = "cmd\frontendapp\"
$F_APP_OUT = "frontendapp"
$F_APP_MAIN = "main.go"

$TARGET_WS = "cmd\frontendapp\ws\"
$WS_OUT = "ws_server"
$WS_MAIN = "ws_main.go"

$TARGET_AUTH = "cmd\minioth\"
$AUTH_OUT = "minioth"
$AUTH_MAIN = "main.go"

$TARGET_SHELL = "cmd\shell\"
$SHELL_OUT = "gshell"
$SHELL_MAIN = "main.go"

function Run-Command {
    param([string]$command)
    Write-Host "Running: $command"
    Invoke-Expression $command
}

# Tidy dependencies
function mod {
    Run-Command "go mod tidy"
}

# Build all targets
function build {
    Run-Command "go build -o ${TARGET_API}${API_OUT} ${TARGET_API}${API_MAIN}"
    Run-Command "go build -o ${TARGET_F_APP}${F_APP_OUT} ${TARGET_F_APP}${F_APP_MAIN}"
    Run-Command "go build -o ${TARGET_AUTH}${AUTH_OUT} ${TARGET_AUTH}${AUTH_MAIN}"
    Run-Command "go build -o ${TARGET_WS}${WS_OUT} ${TARGET_WS}${WS_MAIN}"
}

# Run all services
function run {
    Run-Command "go build -o ${TARGET_API}${API_OUT} ${TARGET_API}${API_MAIN}"
    Run-Command "go build -o ${TARGET_F_APP}${F_APP_OUT} ${TARGET_F_APP}${F_APP_MAIN}"
    Run-Command "go build -o ${TARGET_AUTH}${AUTH_OUT} ${TARGET_AUTH}${AUTH_MAIN}"
    Run-Command "go build -o ${TARGET_WS}${WS_OUT} ${TARGET_WS}${WS_MAIN}"
}

# Stop all services
function stop {
    Stop-Process -Name ${AUTH_OUT} -Force -ErrorAction SilentlyContinue
    Stop-Process -Name ${WS_OUT} -Force -ErrorAction SilentlyContinue
    Stop-Process -Name ${API_OUT} -Force -ErrorAction SilentlyContinue
    Stop-Process -Name ${AUTH_OUT} -Force -ErrorAction SilentlyContinue
}

# Clean all targets
function clean {
    Remove-Item -Force -Path "${TARGET_API}${API_OUT}.exe", "${TARGET_SHELL}${SHELL_OUT}.exe", "${TARGET_AUTH}${AUTH_OUT}.exe", "${TARGET_F_APP}${F_APP_OUT}.exe", "${TARGET_WS}${WS_OUT}.exe"
}

# Call a specific target
function userspace {
    Run-Command "go build -o ${TARGET_API}${API_OUT} ${TARGET_API}${API_MAIN}"
    Run-Command ".\\${TARGET_API}${API_OUT}.exe"
}

function front {
    Run-Command "go build -o ${TARGET_F_APP}${F_APP_OUT} ${TARGET_F_APP}${F_APP_MAIN}"
    Run-Command ".\\${TARGET_F_APP}${F_APP_OUT}.exe"
}

function front_ws {
    Run-Command "go build -o ${TARGET_WS}${WS_OUT} ${TARGET_WS}${WS_MAIN}"
    Run-Command ".\\${TARGET_WS}${WS_OUT}.exe"
}

function minioth {
    Run-Command "go build -o ${TARGET_AUTH}${AUTH_OUT} ${TARGET_AUTH}${AUTH_MAIN}"
    Run-Command ".\\${TARGET_AUTH}${AUTH_OUT}.exe"
}

function shell {
    Run-Command "go build -o ${TARGET_SHELL}${SHELL_OUT} ${TARGET_SHELL}${SHELL_MAIN}"
    Run-Command ".\\${TARGET_SHELL}${SHELL_OUT}.exe"
}

# Interactively choose action to run
Write-Host "Choose an action:"
Write-Host "1. Build"
Write-Host "2. Run"
Write-Host "3. Stop"
Write-Host "4. Clean"
Write-Host "5. Userspace"
Write-Host "6. Front"
Write-Host "7. Front WS"
Write-Host "8. Minioth"
Write-Host "9. Shell"

$choice = Read-Host "Enter the number of the action you want to perform"

switch ($choice) {
    "1" { build }
    "2" { run }
    "3" { stop }
    "4" { clean }
    "5" { userspace }
    "6" { front }
    "7" { front_ws }
    "8" { minioth }
    "9" { shell }
    default { Write-Host "Invalid choice. Please choose a valid option." }
}
