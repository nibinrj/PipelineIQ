# Single entry point for PipelineIQ. PowerShell 7.
# Usage: .\tasks.ps1 up | down | logs | test | lint | jenkins | seed | help
param(
    [Parameter(Position = 0)]
    [ValidateSet('up', 'down', 'logs', 'test', 'lint', 'jenkins', 'seed', 'help')]
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
    'jenkins' {
        Require-EnvFile
        $deadline = (Get-Date).AddMinutes(3)
        do {
            try {
                $resp = Invoke-WebRequest -Uri 'http://localhost:8081/login' -UseBasicParsing
                if ($resp.StatusCode -eq 200) {
                    Write-Host 'Jenkins is up at http://localhost:8081'
                    return
                }
            } catch {
                Write-Host 'waiting for Jenkins...'
            }
            Start-Sleep -Seconds 3
        } while ((Get-Date) -lt $deadline)
        Write-Error 'Jenkins did not answer http://localhost:8081/login within 3 minutes.'
    }
    'seed' {
        Require-EnvFile
        $envMap = @{}
        Get-Content -LiteralPath '.env' | ForEach-Object {
            if ($_ -match '^\s*#' -or $_ -notmatch '=') { return }
            $key, $value = $_.Split('=', 2)
            $envMap[$key.Trim()] = $value.Trim()
        }
        $user = if ($envMap['JENKINS_ADMIN_ID']) { $envMap['JENKINS_ADMIN_ID'] } else { 'admin' }
        $pass = $envMap['JENKINS_ADMIN_PASSWORD']
        if (-not $pass) { Write-Error 'JENKINS_ADMIN_PASSWORD is missing from .env' }
        $basic = [Convert]::ToBase64String([Text.Encoding]::ASCII.GetBytes("${user}:${pass}"))
        $auth = @{ Authorization = "Basic $basic" }
        $deadline = (Get-Date).AddMinutes(3)
        do {
            try {
                Invoke-RestMethod -Headers $auth -Uri 'http://localhost:8081/job/seed/api/json' | Out-Null
                break
            } catch {
                Write-Host 'waiting for the seed job...'
                Start-Sleep -Seconds 3
            }
        } while ((Get-Date) -lt $deadline)
        $crumb = Invoke-RestMethod -Headers $auth -Uri 'http://localhost:8081/crumbIssuer/api/json'
        $auth[$crumb.crumbRequestField] = $crumb.crumb
        Invoke-RestMethod -Method Post -Headers $auth -Uri 'http://localhost:8081/job/seed/build'
        Write-Host 'Triggered the seed job.'
    }
    'help' {
        @'
PipelineIQ tasks (PowerShell 7):
  .\tasks.ps1 up       build and start the compose stack
  .\tasks.ps1 down     stop the compose stack
  .\tasks.ps1 logs     follow compose logs
  .\tasks.ps1 test     go test -race ./...
  .\tasks.ps1 lint     gofmt -l, go vet, golangci-lint run
  .\tasks.ps1 jenkins  wait until http://localhost:8081/login returns 200
  .\tasks.ps1 seed     trigger the Job DSL seed job (needs a crumb)
'@
    }
}
