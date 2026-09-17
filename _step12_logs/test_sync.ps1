$ErrorActionPreference = 'SilentlyContinue'
$sv = New-Object Microsoft.PowerShell.Commands.WebRequestSession

# Step 1: Get CSRF token
$r = Invoke-WebRequest -Uri 'http://localhost:8080/login' -WebSession $sv -UseBasicParsing
$csrf = ($sv.Cookies.GetCookies('http://localhost:8080') | Where-Object { $_.Name -eq 'pwams_csrf' }).Value
Write-Host "CSRF token: $csrf"

# Step 2: Login
$headers = @{'X-CSRF-Token' = $csrf}
$body = 'login=komikukan&password=TestPassword123!'
$loginResp = Invoke-WebRequest -Uri 'http://localhost:8080/login' -Method POST -Headers $headers -Body $body -WebSession $sv -UseBasicParsing
Write-Host "Login status: $($loginResp.StatusCode)"

# Check for session cookie
$cookies = $sv.Cookies.GetCookies('http://localhost:8080')
Write-Host "Cookies:"
$cookies | Select-Object Name,Value | Format-Table -AutoSize

# Step 3: Get new CSRF token after login
$csrf = ($sv.Cookies.GetCookies('http://localhost:8080') | Where-Object { $_.Name -eq 'pwams_csrf' }).Value
Write-Host "CSRF after login: $csrf"

# Step 4: Create a person via sync push
$opId = [guid]::NewGuid().ToString()
$recordId = [guid]::NewGuid().ToString()
$idempotencyKey = [guid]::NewGuid().ToString()
$now = [DateTimeOffset]::UtcNow.ToString('o')
$uniqueSuffix = $opId.Substring(0,8)
$uniqueEmail = "testsync_$uniqueSuffix@example.com"
$uniqueNid = "NID_$uniqueSuffix"

$payload = @{
    full_name = 'TestSync User'
    nic_passport = $uniqueNid
    email = $uniqueEmail
    phone = '1234567890'
    address = '123 Test St'
    date_of_birth = '1990-01-01T00:00:00Z'
    gender = 'Male'
    status = 'Active'
}

$syncBody = @{
    operations = @(
        @{
            id = $opId
            entity_type = 'person'
            operation = 'CREATE'
            record_id = $recordId
            client_version = 1
            payload = $payload
            created_at = $now
        }
    )
} | ConvertTo-Json -Depth 10

$syncHeaders = @{
    'X-CSRF-Token' = $csrf
    'Idempotency-Key' = $idempotencyKey
    'Content-Type' = 'application/json'
}

Write-Host "`nSending sync push..."
Write-Host "Operation ID: $opId"
Write-Host "Record ID: $recordId"
Write-Host "Idempotency Key: $idempotencyKey"

$syncResp = Invoke-RestMethod -Uri 'http://localhost:8080/api/v1/sync/push' -Method POST -Headers $syncHeaders -Body $syncBody -WebSession $sv
Write-Host "`nSync response:"
$syncResp | ConvertTo-Json -Depth 10

# Output for verification
Write-Host "`n=== VERIFICATION DATA ==="
Write-Host "OPERATION_ID=$opId"
Write-Host "RECORD_ID=$recordId"
Write-Host "IDEMPOTENCY_KEY=$idempotencyKey"
Write-Host "EMAIL=$uniqueEmail"
Write-Host "NID=$uniqueNid"
