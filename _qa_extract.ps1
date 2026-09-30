param([string]$Root = 'e:\PWAMS\_qa_xlsx', [string]$OutCsv = 'e:\PWAMS\_qa_report_flat.csv')

$ErrorActionPreference = 'Stop'
[xml]$wb = Get-Content (Join-Path $Root 'xl\workbook.xml') -Encoding UTF8
[xml]$rels = Get-Content (Join-Path $Root 'xl\_rels\workbook.xml.rels') -Encoding UTF8

$relMap = @{}
foreach ($r in $rels.Relationships.Relationship) { $relMap[$r.Id] = $r.Target }

$sheets = @()
foreach ($s in $wb.workbook.sheets.sheet) {
    $rid = $null
    foreach ($a in $s.Attributes) { if ($a.LocalName -eq 'id') { $rid = $a.Value } }
    $target = $relMap[$rid]
    $target = $target.TrimStart('/')
    if ($target -notmatch '^xl/') { $target = 'xl/' + $target }
    $sheets += [pscustomobject]@{ Name = $s.name; Path = (Join-Path $Root $target.Replace('/', '\')) }
}

$rowsOut = New-Object System.Collections.ArrayList
$cols = @('A','B','C','D','E','F','G','H','I','J','K','L','M','N')
foreach ($sh in $sheets) {
    [xml]$doc = Get-Content $sh.Path -Encoding UTF8
    $rIdx = 0
    foreach ($row in $doc.worksheet.sheetData.row) {
        $rIdx++
        $cells = @{}
        foreach ($c in $row.c) {
            $ref = $c.r
            $col = ($ref -replace '\d+', '')
            $txt = ''
            if ($c.t -eq 'inlineStr' -and $null -ne $c.is) {
                foreach ($n in $c.is.ChildNodes) {
                    if ($n.LocalName -eq 't') { $txt += $n.InnerText }
                }
            } elseif ($null -ne $c.v) {
                $txt = [string]$c.v
            }
            $cells[$col] = $txt
        }
        $obj = [ordered]@{ Sheet = $sh.Name; Row = $rIdx }
        foreach ($k in $cols) { $obj[$k] = [string]$cells[$k] }
        [void]$rowsOut.Add([pscustomobject]$obj)
    }
}

$rowsOut | Export-Csv -Path $OutCsv -NoTypeInformation -Encoding UTF8
Write-Output ("sheets=" + $sheets.Count + " rows=" + $rowsOut.Count)
foreach ($s in $sheets) { Write-Output $s.Name }
