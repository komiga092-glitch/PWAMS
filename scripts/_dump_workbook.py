"""dump_workbook.py - print workbook rows for the modules still to port."""
import csv
import sys

MODS = sys.argv[1:] or ["Audit-Logs", "Reports", "File-Upload",
                        "Offline-Sync", "Security-RBAC", "Localization-i18n"]
rows = list(csv.DictReader(open("qa_cases_flat.csv", encoding="utf-8-sig")))
for m in MODS:
    print("=" * 100)
    print(m)
    for x in rows:
        if x["Module"] != m:
            continue
        print(f"{x['ID']} | {x['Title']}")
        print(f"   EXP: {x['Expected']}")
