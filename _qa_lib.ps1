# Helper for QA session management during manual/automated verification.
# Sessions are DB-backed, so a server restart (which clears the in-memory
# rate-limit buckets) does NOT invalidate existing sessions.
$ErrorActionPreference = 'SilentlyContinue'
$script:BASE = 'http://127.0.0.1:8081'

function Restart-PwamsServer {
    $p = (Get-NetTCPConnection -State Listen -LocalPort 8081 -ErrorAction SilentlyContinue).OwningProcess
    if ($p) { Stop-Process -Id $p -Force }
    Start-Sleep -Seconds 2
    $env:APP_PORT = '8081'
    $env:APP_ENV = 'development'
    Start-Process -FilePath 'e:\PWAMS\bin\qa_server.exe' -WorkingDirectory 'e:\PWAMS' `
        -RedirectStandardOutput 'e:\PWAMS\_qa_srv_out.log' `
        -RedirectStandardError 'e:\PWAMS\_qa_srv_err.log' -WindowStyle Hidden
    foreach ($i in 1..60) {
        Start-Sleep -Seconds 1
        $h = & curl.exe -s $script:BASE/health
        if ($h -match '"status":"healthy"') { return $true }
    }
    return $false
}

function Get-Session([string]$User) {
    $j = "e:\PWAMS\_qa_session_$User.txt"
    if (Test-Path $j) {
        $line = Get-Content $j | Where-Object { $_ -match 'pwams_session' }
        if ($line) { return $j }
        Remove-Item $j -Force
    }
    New-Item -ItemType File -Path $j -Force | Out-Null
    & curl.exe -s -o NUL -c $j -b $j "$script:BASE/login"
    $csrf = ((Get-Content $j | Select-String 'pwams_csrf') -split "`t")[-1]
    $st = & curl.exe -s -o NUL -w '%{http_code}' -c $j -b $j -X POST "$script:BASE/login" `
        -H "X-CSRF-Token: $csrf" --data-urlencode "login=$User" `
        --data-urlencode 'password=QaPassw0rd!2026'
    if ($st -eq '429') {
        Restart-PwamsServer | Out-Null
        & curl.exe -s -o NUL -c $j -b $j "$script:BASE/login"
        $csrf = ((Get-Content $j | Select-String 'pwams_csrf') -split "`t")[-1]
        $st = & curl.exe -s -o NUL -w '%{http_code}' -c $j -b $j -X POST "$script:BASE/login" `
            -H "X-CSRF-Token: $csrf" --data-urlencode "login=$User" `
            --data-urlencode 'password=QaPassw0rd!2026'
    }
    if ($st -eq '303') { return $j }
    return $null
}

# Status probe. $kind = 'api' (JSON) or 'page' (HTML navigation).
function Get-Status([string]$Jar, [string]$Method, [string]$Path, [string]$Kind = 'api', $Body = $null) {
    $args_ = @('-s', '-o', 'NUL', '-w', '%{http_code}', '-b', $Jar, '-c', $Jar, '-X', $Method)
    if ($Kind -eq 'page') { $args_ += @('-H', 'Accept: text/html') } else { $args_ += @('-H', 'Accept: application/json') }
    if ($null -ne $Body) {
        $tmp = New-TemporaryFile
        [System.IO.File]::WriteAllText($tmp, $Body)
        $args_ += @('-H', 'Content-Type: application/json', '--data-binary', "@$tmp")
    }
    $args_ += ($script:BASE + $Path)
    $code = & curl.exe @args_
    if ($null -ne $Body) { Remove-Item $tmp -Force }
    return $code
}
