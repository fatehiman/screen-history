# Build Screen History for Windows and Linux.
# Needs: Go, and go-winres (go install github.com/tc-hib/go-winres@latest)
# Output goes to .\dist
$ErrorActionPreference = "Stop"
Set-Location $PSScriptRoot

# Windows resources (icons, manifest, version). Creates rsrc_windows_amd64.syso
go-winres make --in winres/winres.json --arch amd64
if ($LASTEXITCODE) { exit 1 }

go test ./...
if ($LASTEXITCODE) { exit 1 }

$env:CGO_ENABLED = "0"
$env:GOOS = "windows"; $env:GOARCH = "amd64"
go build -trimpath -ldflags "-H windowsgui -s -w" -o dist/screen-history.exe .
if ($LASTEXITCODE) { exit 1 }

$env:GOOS = "linux"; $env:GOARCH = "amd64"
go build -trimpath -ldflags "-s -w" -o dist/screen-history-linux-amd64 .
if ($LASTEXITCODE) { exit 1 }

Remove-Item Env:GOOS, Env:GOARCH, Env:CGO_ENABLED
Get-ChildItem dist
