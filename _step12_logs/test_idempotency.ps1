$ErrorActionPreference = 'SilentlyContinue'
$sv = New-Object Microsoft.PowerShell.Commands.WebRequestSession

# Step 1: Get CSRF token
$r = Invoke-WebRequest -Uri 'http://localhost:8080/login' -WebSession $sv -UseBasicParsing
$csrf = ($sv.Cookies.GetCookies('http://localhost:8080') | Where-Object { $_.Name -eq 'pwams_csrf' }).Value

# Step 2: Login
$headers = @{'X-CSRF-Token' = $csrf}
$body = 'login=komikukan&password=TestPassword123!'
$loginResp = Invoke-WebRequest -Uri 'http://localhost:8080/login' -Method POST -Headers $headers -Body $body -WebSession $sv -UseBasicParsing

# Step 3: Get new CSRF token after login
$csrf = ($sv.Cookies.GetCookies('http://localhost:8080') | Where-Object { $_.Name -eq 'pwams_csrf' }).Value

# Step 4: Send the SAME request with the SAME idempotency key
$opId = '2ea381b7-ba64-4d5f-b693-c2dfc33b12ae'
$recordId = '23ac821b-5a9a-4c27-884d-83fdce752fd4'
$idempotencyKey = 'f8029ab8-7a3d-4d6a-bdb7-641125459520'
$now = [DateTimeOffset]::UtcNow.ToString('o')

$payload = @{
    full_name = 'TestSync User'
    nic_passport = 'NID_2ea381b7'
    email = 'testsync_2ea381b7@example.com'
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

Write-Host "Sending duplicate sync push with same idempotency key..."
Write-Host "Idempotency Key: $idempotencyKey"

$syncResp = Invoke-RestMethod -Uri 'http://localhost:8080/api/v1/sync/push' -Method POST -Headers $syncHeaders -Body $syncBody -WebSession $sv
Write-Host "`nSync response:"
$syncResp | ConvertTo-Json -Depth 10
