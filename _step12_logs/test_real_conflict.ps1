$ErrorActionPreference = 'SilentlyContinue'
$baseUrl = 'http://localhost:8080'

# Create a fresh session
$sv = New-Object Microsoft.PowerShell.Commands.WebRequestSession

# Step 1: GET login page to obtain CSRF cookie
$r = Invoke-WebRequest -Uri "$baseUrl/login" -WebSession $sv -UseBasicParsing
$csrf = ($sv.Cookies.GetCookies($baseUrl) | Where-Object { $_.Name -eq 'pwams_csrf' }).Value
Write-Output "Step 1 - CSRF cookie obtained: $csrf"

# Step 2: POST login (form-encoded)
$body = 'login=komikukan&password=TestPassword123!'
$headers = @{'X-CSRF-Token' = $csrf}
$loginResp = Invoke-WebRequest -Uri "$baseUrl/login" -Method POST -Headers $headers -Body $body -WebSession $sv -UseBasicParsing
Write-Output "Step 2 - Login response status: $($loginResp.StatusCode)"

# Step 3: Create a new person record (Client A - initial create)
$opId1 = [guid]::NewGuid().ToString()
$recordId = [guid]::NewGuid().ToString()
$idempotencyKey1 = [guid]::NewGuid().ToString()
$now = [DateTimeOffset]::UtcNow.ToString('o')

# Use the correct field names matching the Person struct JSON tags
$payload = @{
    full_name = 'ConflictTest Original'
    nic_passport = 'NID_' + $opId1.Substring(0,8)
    email = 'conflict_' + $opId1.Substring(0,8) + '@example.com'
    phone = '1111111111'
    address = 'Original Address'
    date_of_birth = '1990-01-01T00:00:00Z'
    gender = 'Male'
    status = 'Active'
}

$body = @{
    operations = @(@{
        id = $opId1
        entity_type = 'person'
        operation = 'CREATE'
        record_id = $recordId
        client_version = 1
        payload = $payload
        created_at = $now
    })
} | ConvertTo-Json -Depth 10

$headers = @{'X-CSRF-Token' = $csrf; 'Idempotency-Key' = $idempotencyKey1; 'Content-Type' = 'application/json'}
$createResp = Invoke-RestMethod -Uri "$baseUrl/api/v1/sync/push" -Method POST -Headers $headers -Body $body -WebSession $sv
Write-Output "Step 3 - CREATE response: $($createResp | ConvertTo-Json -Depth 5)"

# Step 4: Client A reads version 1 (simulating offline read)
Write-Output "Step 4 - Client A reads record at version 1 (record_id: $recordId)"

# Step 5: Client B updates the record (version becomes 2)
$opId2 = [guid]::NewGuid().ToString()
$idempotencyKey2 = [guid]::NewGuid().ToString()
$now2 = [DateTimeOffset]::UtcNow.ToString('o')

$payload2 = @{
    full_name = 'ConflictTest UpdatedByB'
    nic_passport = 'NID_' + $opId1.Substring(0,8)
    email = 'conflict_b_' + $opId2.Substring(0,8) + '@example.com'
    phone = '2222222222'
    address = 'Updated by B'
    date_of_birth = '1990-01-01T00:00:00Z'
    gender = 'Male'
    status = 'Active'
}

$body2 = @{
    operations = @(@{
        id = $opId2
        entity_type = 'person'
        operation = 'UPDATE'
        record_id = $recordId
        client_version = 1
        payload = $payload2
        created_at = $now2
    })
} | ConvertTo-Json -Depth 10

$headers2 = @{'X-CSRF-Token' = $csrf; 'Idempotency-Key' = $idempotencyKey2; 'Content-Type' = 'application/json'}
$updateRespB = Invoke-RestMethod -Uri "$baseUrl/api/v1/sync/push" -Method POST -Headers $headers2 -Body $body2 -WebSession $sv
Write-Output "Step 5 - Client B UPDATE response: $($updateRespB | ConvertTo-Json -Depth 5)"

# Step 6: Client A attempts update using stale version 1 (should get 409)
$opId3 = [guid]::NewGuid().ToString()
$idempotencyKey3 = [guid]::NewGuid().ToString()
$now3 = [DateTimeOffset]::UtcNow.ToString('o')

$payload3 = @{
    full_name = 'ConflictTest UpdatedByA_Stale'
    nic_passport = 'NID_' + $opId1.Substring(0,8)
    email = 'conflict_a_' + $opId3.Substring(0,8) + '@example.com'
    phone = '3333333333'
    address = 'Updated by A (stale)'
    date_of_birth = '1990-01-01T00:00:00Z'
    gender = 'Male'
    status = 'Active'
}

$body3 = @{
    operations = @(@{
        id = $opId3
        entity_type = 'person'
        operation = 'UPDATE'
        record_id = $recordId
        client_version = 1
        payload = $payload3
        created_at = $now3
    })
} | ConvertTo-Json -Depth 10

$headers3 = @{'X-CSRF-Token' = $csrf; 'Idempotency-Key' = $idempotencyKey3; 'Content-Type' = 'application/json'}

try {
    $updateRespA = Invoke-RestMethod -Uri "$baseUrl/api/v1/sync/push" -Method POST -Headers $headers3 -Body $body3 -WebSession $sv
    Write-Output "Step 6 - Client A UPDATE response: $($updateRespA | ConvertTo-Json -Depth 5)"
} catch {
    $statusCode = $_.Exception.Response.StatusCode.value__
    $stream = $_.Exception.Response.GetResponseStream()
    $reader = New-Object System.IO.StreamReader($stream)
    $errorBody = $reader.ReadToEnd()
    Write-Output "Step 6 - Client A UPDATE failed with HTTP $statusCode : $errorBody"
}

Write-Output "=== TEST COMPLETE ==="
Write-Output "Record ID: $recordId"
Write-Output "Check PostgreSQL for final state"
