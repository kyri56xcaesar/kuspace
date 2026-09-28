# build.ps1 - Windows twin of the Makefile (same task names)
#
#   .\build.ps1                 list tasks
#   .\build.ps1 setup           first-time setup (submodule, secrets, compose .env)
#   .\build.ps1 up              build + start the docker-compose stack
#   .\build.ps1 smoke           end-to-end test (needs bash: Git Bash or WSL)
#   .\build.ps1 check           fmt + vet + unit tests
#   .\build.ps1 build-uspace    one service (also: run-uspace, build-minioth, ...)

param(
    [Parameter(Position = 0)][string]$Task = "help"
)

$ErrorActionPreference = "Stop"

$Pkgs       = @("./...")  # data/ has its own go.mod, so ./... skips it
$Services   = @("uspace", "frontapp", "wss")
$Bin        = "bin"
$ComposeDir = "deployments/docker-compose"
$Secrets    = "configs/secrets.env"
$MiniothDir = "third_party/minioth"

function Invoke-Checked([string]$Exe, [string[]]$ArgList) {
    & $Exe @ArgList
    if ($LASTEXITCODE -ne 0) { throw "$Exe $($ArgList -join ' ') failed ($LASTEXITCODE)" }
}

function Compose([string[]]$ArgList) {
    Invoke-Checked "docker" (@("compose", "--project-directory", $ComposeDir, "-f", "$ComposeDir/docker-compose.yml") + $ArgList)
}

function New-Hex([int]$Bytes) {
    $b = New-Object byte[] $Bytes
    [System.Security.Cryptography.RandomNumberGenerator]::Fill($b)
    -join ($b | ForEach-Object { $_.ToString("x2") })
}

# --------------------------------------------------------------- setup
function Submodule { Invoke-Checked "git" @("submodule", "update", "--init", "--recursive") }

function Secrets {
    if (Test-Path $Secrets) {
        Write-Output "$Secrets exists, leaving it alone"
    } else {
        $svc = New-Hex 32
        $values = @{
            "JWT_SECRET_KEY"          = New-Hex 64
            "JWT_REFRESH_SECRET_KEY"  = New-Hex 64
            "SERVICE_SECRET_KEY"      = $svc
            "MINIOTH_SERVICE_SECRETS" = "uspace:$svc,frontapp:$svc,wss:$svc"
            "MINIOTH_SECRET_KEY"      = New-Hex 16
            "MINIO_SECRET_KEY"        = New-Hex 16
            "FSL_SECRET_KEY"          = New-Hex 16
        }
        $lines = Get-Content "configs/secrets.env.example" | ForEach-Object {
            if ($_ -match '^([A-Z_]+)=$' -and $values.ContainsKey($Matches[1])) { "$($Matches[1])=$($values[$Matches[1]])" } else { $_ }
        }
        Set-Content -Path $Secrets -Value $lines -Encoding ascii
        Write-Output "created $Secrets"
    }
    # compose reads the secrets through .env next to the compose file
    Copy-Item $Secrets "$ComposeDir/.env" -Force
}

function Setup { Submodule; Secrets }

# --------------------------------------------------------------- build
function Build-Service([string]$Name) { Invoke-Checked "go" @("build", "-o", "$Bin/$Name.exe", "./cmd/$Name") }

function Build-Minioth {
    Push-Location $MiniothDir
    try { Invoke-Checked "go" @("build", "-o", "../../$Bin/minioth.exe", "./cmd/minioth") } finally { Pop-Location }
}

function Build { foreach ($s in $Services) { Build-Service $s }; Build-Minioth }

# --------------------------------------------------------------- quality
function Fmt {
    $out = & gofmt -l cmd internal pkg scripts
    if ($out) { $out; throw "files need gofmt" }
}
function Vet  { Invoke-Checked "go" (@("vet") + $Pkgs) }
function Test { Invoke-Checked "go" (@("test") + $Pkgs) }
function Lint { Invoke-Checked "golangci-lint" (@("run", "-c", ".golangci-lint.yaml") + $Pkgs) }
function WebDeps {
    if (-not (Get-Command npm -ErrorAction SilentlyContinue)) { throw "the JS tools need node and npm" }
    if (-not (Test-Path "web/node_modules")) { Invoke-Checked "npm" @("ci", "--prefix", "web", "--no-audit", "--no-fund") }
}
function TestJs { WebDeps; Invoke-Checked "node" @("--test", "web/tests/") }
function LintJs {
    WebDeps
    Push-Location web
    try { Invoke-Checked "npx" @("--no-install", "eslint", "static/js", "tests") } finally { Pop-Location }
}
function Check { Fmt; Vet; Test }

# --------------------------------------------------------------- docker compose
function Images { foreach ($s in @("minioth", "wss", "uspace", "frontapp")) { Write-Output "building $s"; Compose @("build", $s) } }
function Up { Secrets; Images; Compose @("up", "-d"); Compose @("ps") }
function Down { Compose @("down") }
function Smoke {
    if (-not (Get-Command bash -ErrorAction SilentlyContinue)) { throw "smoke needs bash (Git Bash or WSL)" }
    Invoke-Checked "bash" @("scripts/smoke.sh")
}

# --------------------------------------------------------------- kubernetes
function Kuspacectl([string[]]$ArgList) { Invoke-Checked "go" (@("run", "scripts/kuspacectl.go") + $ArgList) }

# --------------------------------------------------------------- cleanup
function Remove-Built { Remove-Item -Recurse -Force $Bin, "docs/code" -ErrorAction SilentlyContinue }

$tasks = [ordered]@{
    "help"          = { $tasks.Keys | ForEach-Object { "  $_" } }
    "setup"         = { Setup }
    "submodule"     = { Submodule }
    "secrets"       = { Secrets }
    "build"         = { Build }
    "build-minioth" = { Build-Minioth }
    "fmt"           = { Fmt }
    "vet"           = { Vet }
    "test"          = { Test }
    "test-js"       = { TestJs }
    "lint-js"       = { LintJs }
    "lint"          = { Lint }
    "check"         = { Check }
    "images"        = { Images }
    "up"            = { Up }
    "down"          = { Down }
    "logs"          = { Compose @("logs", "-f") }
    "ps"            = { Compose @("ps") }
    "smoke"         = { Smoke }
    "k8s-build"     = { Kuspacectl @("-build") }
    "k8s-push"      = { Kuspacectl @("-build", "-push") }
    "k8s-deploy"    = { Kuspacectl @("-deploy") }
    "k8s-destroy"   = { Kuspacectl @("-destroy") }
    "clean"         = { Remove-Built }
}

switch -Regex ($Task) {
    '^build-(.+)$' { if ($Matches[1] -eq "minioth") { Build-Minioth } else { Build-Service $Matches[1] }; break }
    '^run-(.+)$'   { Build-Service $Matches[1]; & "$Bin/$($Matches[1]).exe"; break }
    default {
        if ($tasks.Contains($Task)) { & $tasks[$Task] }
        else { Write-Output "unknown task '$Task'"; & $tasks["help"]; exit 1 }
    }
}
