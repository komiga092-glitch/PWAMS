. 'e:\PWAMS\_qa_lib.ps1'

$users = @('qa_super_admin', 'qa_admin', 'qa_manager', 'qa_staff',
           'qa_volunteer', 'qa_donor', 'qa_beneficiary', 'qa_student')

$paths = @(
    @{ p = '/dashboard';            kind = 'page' },
    @{ p = '/auth/me';              kind = 'api' },
    @{ p = '/students';             kind = 'api' },
    @{ p = '/students/page';        kind = 'page' },
    @{ p = '/donors';               kind = 'api' },
    @{ p = '/donations';            kind = 'api' },
    @{ p = '/persons';              kind = 'api' },
    @{ p = '/aid-requests';         kind = 'api' },
    @{ p = '/care-provided';        kind = 'api' },
    @{ p = '/loans';                kind = 'api' },
    @{ p = '/loan-repayments';      kind = 'api' },
    @{ p = '/revenue';              kind = 'api' },
    @{ p = '/messages';             kind = 'api' },
    @{ p = '/notifications';        kind = 'api' },
    @{ p = '/users';                kind = 'api' },
    @{ p = '/users/page';           kind = 'page' },
    @{ p = '/audit-logs';           kind = 'api' },
    @{ p = '/audit-logs/page';      kind = 'page' },
    @{ p = '/reports/page';         kind = 'page' },
    @{ p = '/api/reports/students'; kind = 'api' },
    @{ p = '/api/reports/users';    kind = 'api' },
    @{ p = '/files';                kind = 'api' },
    @{ p = '/files/page';           kind = 'page' },
    @{ p = '/api/v1/sync/pull';     kind = 'api' }
)

$rows = New-Object System.Collections.ArrayList
foreach ($u in $users) {
    $jar = Get-Session $u
    if (-not $jar) {
        [void]$rows.Add([pscustomobject]@{ User = $u; Path = '-'; Kind = '-'; Status = 'LOGIN-FAILED' })
        continue
    }
    foreach ($t in $paths) {
        $code = Get-Status $jar GET $t.p $t.kind
        [void]$rows.Add([pscustomobject]@{ User = $u; Path = $t.p; Kind = $t.kind; Status = $code })
    }
}
$rows | Export-Csv -Path 'e:\PWAMS\_qa_rbac_matrix.csv' -NoTypeInformation -Encoding UTF8
$pivot = $rows | Group-Object Path | ForEach-Object {
    $o = [ordered]@{ Path = $_.Name }
    foreach ($u in $users) {
        $v = $_.Group | Where-Object { $_.User -eq $u }
        $o[$u] = if ($v) { $v.Status } else { '?' }
    }
    [pscustomobject]$o
}
$pivot | Format-Table -AutoSize | Out-String -Width 250
