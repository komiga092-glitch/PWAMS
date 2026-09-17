$ErrorActionPreference = 'Stop'
$baseUrl = 'http://localhost:8080'

# Create a fresh session
$sv = New-Object Microsoft.PowerShell.Commands.WebRequestSession

# Step 1: GET /login to obtain CSRF cookie
Write-Host "Step 1: GET /login ..."
$r = Invoke-WebRequest -Uri "$baseUrl/login" -WebSession $sv -UseBasicParsing
$csrf = ($sv.Cookies.GetCookies($baseUrl) | Where-Object { $_.Name -eq 'pwams_csrf' }).Value
Write-Host "  CSRF token: $csrf"

# Step 2: POST /login with form data
Write-Host "Step 2: POST /login ..."
$headers = @{'X-CSRF-Token' = $csrf}
$body = 'login=komikukan&password=TestPassword123!'
$loginResp = Invoke-WebRequest -Uri "$baseUrl/login" -Method POST -Headers $headers -Body $body -WebSession $sv -UseBasicParsing
Write-Host "  Login status: $($loginResp.StatusCode)"

# Step 3: Create a person record
Write-Host "Step 3: POST /api/v1/sync/push (CREATE) ..."
$opId1 = [guid]::NewGuid().ToString()
$recordId = [guid]::NewGuid().ToString()
$idempotencyKey1 = [guid]::NewGuid().ToString()
$now = [DateTimeOffset]::UtcNow.ToString('o')
$uniqueTag = $opId1.Substring(0,8)
$payload = @{
    full_name = 'ConflictTest User'
    nic_passport = 'NID_CFLT_' + $uniqueTag
    email = 'conflict_' + $uniqueTag + '@example.com'
    phone = '1234567890'
    address = '123 Test St'
    date_of_birth = '1990-01-01T00:00:00Z'
    gender = 'Male'
    occupation = 'Tester'
    monthly_income = 0
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
$headers = @{
    'X-CSRF-Token' = $csrf
    'Idempotency-Key' = $idempotencyKey1
    'Content-Type' = 'application/json'
}
$createResp = Invoke-RestMethod -Uri "$baseUrl/api/v1/sync/push" -Method POST -Headers $headers -Body $body -WebSession $sv
Write-Host "  Create response:"
$createResp | ConvertTo-Json -Depth 5

# Step 4: Update the record (version 1 -> 2)
Write-Host "Step 4: POST /api/v1/sync/push (UPDATE - Client B) ..."
$opId2 = [guid]::NewGuid().ToString()
$idempotencyKey2 = [guid]::NewGuid().ToString()
$payload2 = @{
    full_name = 'ConflictTest User Updated'
    nic_passport = 'NID_CFLT_' + $uniqueTag
    email = 'conflict_' + $uniqueTag + '@example.com'
    phone = '9876543210'
    address = '456 Updated St'
    date_of_birth = '1990-01-01T00:00:00Z'
    gender = 'Male'
    occupation = 'Senior Tester'
    monthly_income = 50000
    status = 'Active'
}
$body = @{
    operations = @(@{
        id = $opId2
        entity_type = 'person'
        operation = 'UPDATE'
        record_id = $recordId
        client_version = 1
        payload = $payload2
        created_at = $now
    })
} | ConvertTo-Json -Depth 10
$headers = @{
    'X-CSRF-Token' = $csrf
    'Idempotency-Key' = $idempotencyKey2
    'Content-Type' = 'application/json'
}
$updateResp = Invoke-RestMethod -Uri "$baseUrl/api/v1/sync/push" -Method POST -Headers $headers -Body $body -WebSession $sv
Write-Host "  Update response (Client B - should succeed):"
$updateResp | ConvertTo-Json -Depth 5

# Step 5: Attempt to update with stale version (version 1, but server has version 2)
Write-Host "Step 5: POST /api/v1/sync/push (UPDATE - Client A with stale version) ..."
$opId3 = [guid]::NewGuid().ToString()
$idempotencyKey3 = [guid]::NewGuid().ToString()
$payload3 = @{
    full_name = 'ConflictTest User Stale'
    nic_passport = 'NID_CFLT_' + $uniqueTag
    email = 'conflict_' + $uniqueTag + '@example.com'
    phone = '1111111111'
    address = '789 Stale St'
    date_of_birth = '1990-01-01T00:00:00Z'
    gender = 'Male'
    occupation = 'Stale Tester'
    monthly_income = 10000
    status = 'Active'
}
$body = @{
    operations = @(@{
        id = $opId3
        entity_type = 'person'
        operation = 'UPDATE'
        record_id = $recordId
        client_version = 1  # STALE VERSION - server has version 2
        payload = $payload3
        created_at = $now
    })
} | ConvertTo-Json -Depth 10
$headers = @{
    'X-CSRF-Token' = $csrf
    'Idempotency-Key' = $idempotencyKey3
    'Content-Type' = 'application/json'
}
try {
    $staleResp = Invoke-RestMethod -Uri "$baseUrl/api/v1/sync/push" -Method POST -Headers $headers -Body $body -WebSession $sv
    Write-Host "  Stale update response (should be 409 conflict):"
    $staleResp | ConvertTo-Json -Depth 5
} catch {
    Write-Host "  Stale update response (HTTP error):"
    Write-Host "  Status: $($_.Exception.Response.StatusCode.value__)"
    $streamReader = [System.IO.StreamReader]::new($_.Exception.Response.GetResponseStream())
    $errBody = $streamReader.ReadToEnd()
    Write-Host "  Body: $errBody"
}

Write-Host ""
Write-Host "=== SUMMARY ==="
Write-Host "Record ID: $recordId"
Write-Host "Create success: $($createResp.results[0].success)"
Write-Host "Update (Client B) success: $($updateResp.results[0].success)"
Write-Host "Update (Client B) server_version: $($updateResp.results[0].server_version)"
