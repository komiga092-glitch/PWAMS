"""_qa_probe_profile_pw.py - diagnose POST /profile/password (USR-007/008).

Logs in as qa_volunteer, posts the form-encoded change-password request with
the seeded password and prints the full response evidence. If the change
succeeds the seeded password is restored as Admin so re-runs stay idempotent.
"""
import re
import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))

import qa_run_tests as q  # noqa: E402

NEW_PW = "ProbeOwn!12345"


def show(tag, s, b):
    flat = re.sub(r"\s+", " ", b)
    m = re.search(r'id="change-password"(.{0,400})', flat)
    err = re.search(r'(password_error|alert-danger|error)[^>]*>([^<]{0,160})',
                    flat)
    print(f"{tag}: HTTP {s} len={len(b)}")
    print(f"  change-password block: {m.group(1)[:300] if m else 'n/a'}")
    print(f"  error text: {err.group(2).strip() if err else 'n/a'}")


def main():
    vol = q.Client("probe_prof")
    print("volunteer login:", vol.login("qa_volunteer")[0])
    s, b = vol.request("POST", "/profile/password",
                       data={"current_password": q.QA_PASSWORD,
                             "new_password": NEW_PW,
                             "confirm_password": NEW_PW},
                       headers={"Accept": "application/json"})
    show("form POST (correct current)", s, b)
    if s in (200, 303):
        print("password changed -> restoring seeded password")
        admin, ok, _st, _b = q.login_as("Admin")
        uid = q.find_user_id("qa_volunteer")
        print("admin ok:", ok, "uid:", uid)
        if uid:
            print("restore:", q.api(admin, "PATCH", f"/users/{uid}/password",
                                    {"new_password": q.QA_PASSWORD})[0])
    # wrong current password with a valid form payload
    s2, b2 = vol.request("POST", "/profile/password",
                         data={"current_password": "TotallyWrong1!",
                               "new_password": NEW_PW,
                               "confirm_password": NEW_PW},
                         headers={"Accept": "application/json"})
    show("form POST (wrong current)", s2, b2)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
