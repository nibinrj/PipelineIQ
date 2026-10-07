# Single entry point for PipelineIQ. PowerShell 7.
# Usage: .\tasks.ps1 up | down | logs | test | lint | help
param(
    [Parameter(Position = 0)]
    [ValidateSet('up', 'down', 'logs', 'test', 'lint', 'help')]
    [string]$Task = 'help'
)

$ErrorActionPreference = 'Stop'
Set-Location $PSScriptRoot

function Require-EnvFile {
    if (-not (Test-Path -LiteralPath '.env')) {
        Write-Error "Copy .env.example to .env and set the passwords before '$Task'."
    }
}

switch ($Task) {
    'up' {
        Require-EnvFile
        docker compose --env-file .env up -d --build
        docker compose --env-file .env ps
    }
    'down' {
        docker compose --env-file .env down
    }
    'logs' {
        docker compose --env-file .env logs -f --tail 200
    }
    'test' {
        go test -race ./...
    }
    'lint' {
        $dirty = & gofmt -l .
        if ($dirty) {
            Write-Error ("gofmt would change:`n" + ($dirty -join "`n"))
        }
        go vet ./...
        golangci-lint run
    }
    'help' {
        @'
PipelineIQ tasks (PowerShell 7):
  .\tasks.ps1 up      build and start postgres, grafana, pipelineiq-server
  .\tasks.ps1 down    stop the compose stack
  .\tasks.ps1 logs    follow compose logs
  .\tasks.ps1 test    go test -race ./...
  .\tasks.ps1 lint    gofmt -l, go vet, golangci-lint run
'@
    }
}
