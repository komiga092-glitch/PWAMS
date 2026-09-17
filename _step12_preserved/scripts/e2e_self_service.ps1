# PWAMS Self-Service E2E Verification Script
# Tests the full Beneficiary workflow against a running server.
$ErrorActionPreference = "Stop"
$base = "http://localhost:8081"
$script:failures = 0

function Pass($name) { Write-Host "PASS: $name" -ForegroundColor Green }
function Fail($name, $detail) { $script:failures++; Write-Host "FAIL: $name -- $detail" -ForegroundColor Red }

function Get-CsrfFrom($session) {
    $c = $session.Cookies.GetCookies($base) | Where-Object { $_.Name -eq "pwams_csrf" }
    if ($c) { return $c.Value }
    return ""
}

function New-Session {
    $s = New-Object Microsoft.PowerShell.Commands.WebRequestSession
    Invoke-WebRequest -Uri "$base/login" -WebSession $s -UseBasicParsing -TimeoutSec 10 | Out-Null
    return $s
}

function Invoke-Api($sess, $method, $uri, $bodyObj, $json = $true) {
    $csrf = Get-CsrfFrom $sess
    $params = @{ Uri = "$base$uri"; Method = $method; WebSession = $sess; UseBasicParsing = $true; MaximumRedirection = 0; TimeoutSec = 15; ErrorAction = "Stop" }
    # JSON APIs must echo the double-submit token in the X-CSRF-Token header
    # (the server only parses _csrf from form-encoded bodies).
    $headers = @{}
    if ($csrf) { $headers["X-CSRF-Token"] = $csrf }
    if ($headers.Count -gt 0) { $params.Headers = $headers }
    if ($bodyObj) {
        $params.ContentType = "application/json"
        $params.Body = ($bodyObj | ConvertTo-Json -Depth 6)
    }
    try {
        return Invoke-WebRequest @params
    } catch {
        return $_.Exception.Response
    }
}

function StatusOf($resp) {
    if ($resp.StatusCode) { return [int]$resp.StatusCode }
    return 0
}

# =========================
# 1. Admin login
# =========================
$adminPass = (Get-Content .env | Select-String -Pattern "SUPER_ADMIN_PASSWORD" | ForEach-Object { $_.ToString().Split('=')[1] })
$admin = New-Session
$loginResp = Invoke-Api $admin "POST" "/login" @{ login = "superadmin"; password = $adminPass }
if (StatusOf $loginResp -eq 303) { Pass "Admin login" } else { Fail "Admin login" "got $(StatusOf $loginResp)"; exit 1 }

# =========================
# 2. Create a Staff account (only Staff can register Beneficiary accounts —
#    canCreateRole business rule), then Person A/B + User A/B
# =========================
$stamp = Get-Date -Format "yyyyMMddHHmmss"
$emailA = "bene$stamp@test.local"
$emailB = "bene2$stamp@test.local"

$respStaff = Invoke-Api $admin "POST" "/users" @{ username = "e2estaff$stamp"; email = "staff$stamp@test.local"; role = "Staff" }
$staffUser = ($respStaff.Content | ConvertFrom-Json).user
if (-not $staffUser) { Fail "Create staff user" $respStaff.Content; exit 1 }
Invoke-Api $admin "PATCH" "/users/$($staffUser.id)/status" @{ status = "Active" } | Out-Null
Invoke-Api $admin "PATCH" "/users/$($staffUser.id)/password" @{ new_password = "E2eStaffPass1!" } | Out-Null
Pass "Create + activate staff user"

$staff = New-Session
$resp = Invoke-Api $staff "POST" "/login" @{ login = "staff$stamp@test.local"; password = "E2eStaffPass1!" }
if (StatusOf $resp -eq 303) { Pass "Staff login" } else { Fail "Staff login" "got $(StatusOf $resp)"; exit 1 }

$respA = Invoke-Api $staff "POST" "/persons" @{ full_name = "E2E Beneficiary A $stamp"; nic_passport = "E2E-A-$stamp"; email = $emailA; monthly_income = "20000" }
$personA = ($respA.Content | ConvertFrom-Json).person
if (-not $personA) { Fail "Create person A" "$($respA.StatusCode)"; exit 1 }
Pass "Create person A (id $($personA.id))"

$respB = Invoke-Api $admin "POST" "/persons" @{ full_name = "E2E Beneficiary B $stamp"; nic_passport = "E2E-B-$stamp"; email = $emailB; monthly_income = "18000" }
$personB = ($respB.Content | ConvertFrom-Json).person
if (-not $personB) { Fail "Create person B" $respB.Content; exit 1 }
Pass "Create person B (id $($personB.id))"

$respU = Invoke-Api $staff "POST" "/users" @{ username = "e2ebene$stamp"; email = $emailA; role = "Beneficiary" }
$userA = ($respU.Content | ConvertFrom-Json).user
if (-not $userA) { Fail "Create user A" "$($respU.StatusCode)"; exit 1 }
Invoke-Api $admin "PATCH" "/users/$($userA.id)/status" @{ status = "Active" } | Out-Null
Invoke-Api $admin "PATCH" "/users/$($userA.id)/password" @{ new_password = "E2eTestPass1!" } | Out-Null
Pass "Create + activate user A (id $($userA.id))"

$respU2 = Invoke-Api $staff "POST" "/users" @{ username = "e2ebene2$stamp"; email = $emailB; role = "Beneficiary" }
$userB = ($respU2.Content | ConvertFrom-Json).user
if (-not $userB) { Fail "Create user B" "$($respU2.StatusCode)"; exit 1 }
Invoke-Api $admin "PATCH" "/users/$($userB.id)/status" @{ status = "Active" } | Out-Null
Invoke-Api $admin "PATCH" "/users/$($userB.id)/password" @{ new_password = "E2eTestPass2!" } | Out-Null
Pass "Create + activate user B (id $($userB.id))"

# =========================
# 3. Beneficiary A login + dashboard
# =========================
$beneA = New-Session
$resp = Invoke-Api $beneA "POST" "/login" @{ login = $emailA; password = "E2eTestPass1!" }
if (StatusOf $resp -eq 303) { Pass "Beneficiary A login" } else { Fail "Beneficiary A login" "got $(StatusOf $resp)"; exit 1 }

$resp = Invoke-Api $beneA "GET" "/my/dashboard" $null
if (StatusOf $resp -eq 200 -and $resp.Content -match "My Dashboard") { Pass "Beneficiary A dashboard renders" }
else { Fail "Beneficiary A dashboard renders" "got $(StatusOf $resp)" }

# =========================
# 4. Aid request flow
# =========================
$resp = Invoke-Api $beneA "POST" "/my/aid/request" @{ aid_type = "Medical"; title = "E2E Aid Request A"; description = "E2E medical assistance"; priority = "High"; requested_amount = "5000"; currency = "LKR" }
if (StatusOf $resp -eq 303) { Pass "Beneficiary A submits own aid request" } else { Fail "Beneficiary A submits own aid request" "got $(StatusOf $resp)"; exit 1 }

$resp = Invoke-Api $beneA "GET" "/my/aid" $null
if (StatusOf $resp -eq 200 -and $resp.Content -match "E2E Aid Request A" -and $resp.Content -match "Pending") { Pass "My Aid lists own request as Pending" }
else { Fail "My Aid lists own request as Pending" "got $(StatusOf $resp)" }

# =========================
# 5. Loan application flow
# =========================
$resp = Invoke-Api $beneA "POST" "/my/loans/apply" @{ loan_amount = "10000"; interest_rate = "5"; duration_months = 10; purpose = "E2E loan purpose" }
if (StatusOf $resp -eq 303) { Pass "Beneficiary A applies for loan" } else { Fail "Beneficiary A applies for loan" "got $(StatusOf $resp)"; exit 1 }

$resp = Invoke-Api $beneA "GET" "/my/loans" $null
if (StatusOf $resp -eq 200 -and $resp.Content -match "Pending") { Pass "My Loans shows pending application" }
else { Fail "My Loans shows pending application" "got $(StatusOf $resp)" }

# =========================
# 6. SECURITY: Beneficiary A cannot access admin pages or review
# =========================
$resp = Invoke-Api $beneA "GET" "/users/page" $null
if (StatusOf $resp -eq 403) { Pass "Beneficiary blocked from /users/page" } else { Fail "Beneficiary blocked from /users/page" "got $(StatusOf $resp)" }

$resp = Invoke-Api $beneA "GET" "/aid-requests" $null
if (StatusOf $resp -eq 403) { Pass "Beneficiary blocked from /aid-requests list" } else { Fail "Beneficiary blocked from /aid-requests list" "got $(StatusOf $resp)" }

$resp = Invoke-Api $beneA "GET" "/loans" $null
if (StatusOf $resp -eq 403) { Pass "Beneficiary blocked from /loans list" } else { Fail "Beneficiary blocked from /loans list" "got $(StatusOf $resp)" }

$resp = Invoke-Api $beneA "GET" "/audit-logs" $null
if (StatusOf $resp -eq 403) { Pass "Beneficiary blocked from /audit-logs" } else { Fail "Beneficiary blocked from /audit-logs" "got $(StatusOf $resp)" }

# Find A's aid request id and loan id as admin — scoped to this run's
# person so stale rows from earlier runs can never be selected.
$resp = Invoke-Api $admin "GET" "/aid-requests?person_id=$($personA.id)" $null
$aidList = ($resp.Content | ConvertFrom-Json).data
$aidId = ($aidList | Where-Object { $_.title -eq "E2E Aid Request A" } | Select-Object -First 1).id
if ($aidId) { Pass "Admin can see A's aid request ($aidId)" } else { Fail "Admin can see A's aid request" "not found"; exit 1 }

$resp = Invoke-Api $beneA "PATCH" "/aid-requests/$aidId/review" @{ status = "Approved"; approved_amount = "5000" }
if (StatusOf $resp -eq 403) { Pass "Beneficiary cannot approve own request" } else { Fail "Beneficiary cannot approve own request" "got $(StatusOf $resp)" }

$resp = Invoke-Api $admin "GET" "/loans?person_id=$($personA.id)&status=Pending" $null
$loanList = ($resp.Content | ConvertFrom-Json).data.loans
$loanId = ($loanList | Where-Object { $_.purpose -eq "E2E loan purpose" } | Select-Object -First 1).id
if ($loanId) { Pass "Admin can see A's loan ($loanId)" } else { Fail "Admin can see A's loan" "not found"; exit 1 }

$resp = Invoke-Api $beneA "PATCH" "/loans/$loanId/review" @{ status = "Approved" }
if (StatusOf $resp -eq 403) { Pass "Beneficiary cannot approve own loan" } else { Fail "Beneficiary cannot approve own loan" "got $(StatusOf $resp)" }

# =========================
# 7. Admin reviews aid request -> Approved
# =========================
$resp = Invoke-Api $admin "PATCH" "/aid-requests/$aidId/review" @{ status = "Approved"; approved_amount = "5000"; review_notes = "E2E approve" }
if (StatusOf $resp -eq 200) { Pass "Admin approves aid request" } else { Fail "Admin approves aid request" "got $(StatusOf $resp): $($resp.Content)" }

# =========================
# 8. Admin reviews loan -> Approved -> Active
# =========================
$resp = Invoke-Api $admin "PATCH" "/loans/$loanId/review" @{ status = "Approved" }
if (StatusOf $resp -eq 200) { Pass "Admin approves loan" } else { Fail "Admin approves loan" "got $(StatusOf $resp): $($resp.Content)" }

$resp = Invoke-Api $admin "PATCH" "/loans/$loanId/review" @{ status = "Active" }
if (StatusOf $resp -eq 200) { Pass "Admin activates loan" } else { Fail "Admin activates loan" "got $(StatusOf $resp): $($resp.Content)" }

# =========================
# 9. Admin creates + pays a repayment installment
# =========================
$dueDate = (Get-Date).AddDays(30).ToString("yyyy-MM-dd")
$resp = Invoke-Api $admin "POST" "/loan-repayments" @{ loan_id = "$loanId"; installment_number = 1; due_date = $dueDate; amount = "1050" }
$repayment = ($resp.Content | ConvertFrom-Json).repayment
if ($repayment) { Pass "Admin creates repayment installment" } else { Fail "Admin creates repayment installment" "$($resp.Content)"; exit 1 }

$resp = Invoke-Api $admin "PATCH" "/loan-repayments/$($repayment.id)/pay" @{ paid_amount = "1050"; payment_reference = "E2E-PAY-1" }
if (StatusOf $resp -eq 200) { Pass "Admin records repayment payment" } else { Fail "Admin records repayment payment" "got $(StatusOf $resp): $($resp.Content)" }

# =========================
# 10. Beneficiary A sees statuses + notification
# =========================
$resp = Invoke-Api $beneA "GET" "/my/aid" $null
if ($resp.Content -match "Approved") { Pass "Beneficiary sees Approved aid status" } else { Fail "Beneficiary sees Approved aid status" "not found in /my/aid" }

$resp = Invoke-Api $beneA "GET" "/my/loans" $null
if ($resp.Content -match "Active") { Pass "Beneficiary sees Active loan" } else { Fail "Beneficiary sees Active loan" "not found in /my/loans" }

$resp = Invoke-Api $beneA "GET" "/my/repayments" $null
if ($resp.Content -match "Paid") { Pass "Beneficiary sees paid repayment" } else { Fail "Beneficiary sees paid repayment" "not found in /my/repayments" }

$resp = Invoke-Api $beneA "GET" "/my/dashboard" $null
$content = $resp.Content
if ($content -match "Approved Aid" -and $content -match "Outstanding Balance") { Pass "Dashboard renders aid + financials" } else { Fail "Dashboard renders aid + financials" "missing sections" }

# Expected: loan 10000 @5% => total 10500, repaid 1050 => outstanding 9450, percent 10
if ($content -match "9450") { Pass "Outstanding balance = 9450 (Loan 10000 + 5% - 1050 repaid)" } else { Fail "Outstanding balance = 9450" "value not found" }
if ($content -match "10\.0%") { Pass "Repayment percentage = 10%" } else { Fail "Repayment percentage = 10%" "value not found" }

$resp = Invoke-Api $beneA "GET" "/notifications" $null
if ($resp.Content -match "Aid Request Approved") { Pass "Beneficiary received aid approval notification" } else { Fail "Beneficiary received aid approval notification" "not found" }
if ($resp.Content -match "Loan Application Approved") { Pass "Beneficiary received loan approval notification" } else { Fail "Beneficiary received loan approval notification" "not found" }

# =========================
# 11. Data isolation: Beneficiary B must not see A's data
# =========================
$beneB = New-Session
$resp = Invoke-Api $beneB "POST" "/login" @{ login = $emailB; password = "E2eTestPass2!" }
if (StatusOf $resp -ne 303) { Fail "Beneficiary B login" "got $(StatusOf $resp)"; exit 1 }
Pass "Beneficiary B login"

$resp = Invoke-Api $beneB "GET" "/my/aid" $null
if ($resp.Content -notmatch "E2E Aid Request A") { Pass "B cannot see A's aid requests" } else { Fail "B cannot see A's aid requests" "leak detected" }

$resp = Invoke-Api $beneB "GET" "/my/loans" $null
if ($resp.Content -notmatch "E2E loan purpose") { Pass "B cannot see A's loans" } else { Fail "B cannot see A's loans" "leak detected" }

$resp = Invoke-Api $beneB "GET" "/my/repayments" $null
if ($resp.Content -notmatch "E2E-PAY-1") { Pass "B cannot see A's repayments" } else { Fail "B cannot see A's repayments" "leak detected" }

# =========================
# Summary
# =========================
Write-Host ""
if ($script:failures -eq 0) { Write-Host "ALL E2E CHECKS PASSED" -ForegroundColor Green }
else { Write-Host "$($script:failures) E2E CHECK(S) FAILED" -ForegroundColor Red; exit 1 }