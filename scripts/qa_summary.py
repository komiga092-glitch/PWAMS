"""qa_summary.py - summarise a qa_run_tests.py capture.

Usage: python scripts/qa_summary.py qa_c_run.log [--show fail|blocked|all]

Handles the UTF-16 logs written by PowerShell's ``*> file`` redirection as
well as plain UTF-8 captures.  Prints per-prefix pass/fail/blocked counts and
then the failure lines (test id + evidence) so a run can be triaged quickly.
"""
import collections
import re
import sys

LINE = re.compile(r"^\[(?P<id>[A-Z]{3}-\d{3})\]\s+(?P<result>Pass|Fail|Blocked):\s*(?P<text>.*)$")


def read_log(path):
    for enc in ("utf-8-sig", "utf-16", "utf-8", "cp1252"):
        try:
            with open(path, encoding=enc) as fh:
                text = fh.read()
        except (UnicodeDecodeError, UnicodeError):
            continue
        if text.count("\x00") < len(text) / 10:
            return text.splitlines()
    with open(path, encoding="utf-8", errors="replace") as fh:
        return fh.read().splitlines()


def main():
    if len(sys.argv) < 2:
        print(__doc__)
        return 2
    path = sys.argv[1]
    show = "fail"
    if "--show" in sys.argv:
        show = sys.argv[sys.argv.index("--show") + 1]

    rows = []
    for ln in read_log(path):
        m = LINE.match(ln.strip())
        if m:
            rows.append(m.groupdict())

    per_module = collections.defaultdict(collections.Counter)
    totals = collections.Counter()
    for r in rows:
        module = r["id"].split("-")[0]
        per_module[module][r["result"]] += 1
        totals[r["result"]] += 1

    print(f"capture : {path}")
    print(f"recorded: {len(rows)} cases -> " + ", ".join(
        f"{k}={v}" for k, v in sorted(totals.items())))
    print("-" * 78)
    for module in sorted(per_module):
        c = per_module[module]
        print(f"  {module}: pass={c['Pass']:<3} fail={c['Fail']:<3} blocked={c['Blocked']:<3}")
    print("-" * 78)

    if show in ("fail", "all"):
        print("FAILURES")
        for r in rows:
            if r["result"] != "Pass":
                if show == "fail" and r["result"] == "Blocked":
                    continue
                print(f"  [{r['id']}] {r['result']}: {r['text']}")
        print("-" * 78)
    if show in ("blocked", "all"):
        print("BLOCKED")
        for r in rows:
            if r["result"] == "Blocked":
                print(f"  [{r['id']}] {r['text']}")
    counts = per_module
    unrecorded = [r for r in []]
    del unrecorded
    print(f"modules={len(counts)} total_cases={len(rows)}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
