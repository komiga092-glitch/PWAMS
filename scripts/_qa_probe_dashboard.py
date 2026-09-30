"""_qa_probe_dashboard.py - inspect GET /dashboard/stats vs the list totals.

Prints the raw stats payload and what GET /loans|/aid-requests|... return for
total counts (including the pagination envelope), so DASH-004's mismatch can
be attributed to the harness measurement or to the product.
"""
import json
import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))

import qa_run_tests as q  # noqa: E402

PATHS = ("/students", "/donors", "/donations", "/persons", "/loans",
         "/aid-requests")


def main():
    admin, ok, st, _b = q.login_as("Admin")
    print("admin session:", ok, st)
    s, body = q.api(admin, "GET", "/dashboard/stats")
    print("GET /dashboard/stats ->", s)
    try:
        payload = json.loads(body)
    except Exception:
        payload = {}
    stats = payload.get("data") if isinstance(payload, dict) else payload
    print("stats:", json.dumps(stats, indent=1)[:1500])

    for path in PATHS:
        s2, b2 = q.api(admin, "GET", f"{path}?page_size=1")
        try:
            d = json.loads(b2)
        except Exception:
            d = {}
        keys = sorted(d.keys()) if isinstance(d, dict) else type(d).__name__
        items = q.list_items(b2)
        total = q._list_total(b2, items)
        print(f"{path}?page_size=1 -> {s2} top_keys={keys} "
              f"items={len(items)} _list_total={total}")
        if isinstance(d, dict):
            for k, v in d.items():
                if isinstance(v, dict):
                    print(f"    nested {k}: {sorted(v.keys())}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
