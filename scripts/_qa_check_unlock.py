"""_qa_check_unlock.py - self-test the harness lockout reset.

Run this before/after a QA run to prove the AUTH-005 lockout cannot cascade:

    python scripts/_qa_check_unlock.py

It reports how many seeded QA accounts are locked / carry failed-login
counters, clears them with the harness helper and re-checks.
"""
import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))

import qa_run_tests as q  # noqa: E402  (path insert must come first)

NEEDS_RESET = ("SELECT count(*) FROM users WHERE username LIKE 'qa%' AND "
               "(status = 'Locked' OR locked_until IS NOT NULL "
               "OR failed_login_attempts <> 0)")
ADMIN_STATE = ("SELECT status || ' / failed=' || failed_login_attempts || ' / "
               "locked_until=' || coalesce(locked_until::text, 'null') "
               "FROM users WHERE username = 'qa_admin'")


def main():
    # leave a deliberately locked row behind to prove the helper clears one
    q.db_exec("UPDATE users SET failed_login_attempts = 3, locked_until = now() "
              "+ interval '30 minutes', status = 'Locked' "
              "WHERE username = 'qa_admin'")
    print("qa_admin (forced lock):", q.db_scalar(ADMIN_STATE))
    print("rows needing reset     :", q.db_scalar(NEEDS_RESET))
    print("unlock_qa_accounts()   :", q.unlock_qa_accounts("self-test"))
    print("qa_admin (after)       :", q.db_scalar(ADMIN_STATE))
    print("rows needing reset     :", q.db_scalar(NEEDS_RESET))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
