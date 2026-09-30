"""_qa_restore_admin_pw.py - reset qa_admin's password back to the seeded value.

Only needed after a manual probe changed it: uses the product's own
password-reset API (forgot-password -> OTP from the Mailpit sandbox ->
verify-reset-otp -> reset-password) so no password hash is written directly.
"""
import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))

import qa_run_tests as q  # noqa: E402

EMAIL = "qa_admin@qa.local"


def main():
    q.restart_server()          # clears the login/reset rate-limit budgets
    c = q.Client("restore_admin_pw")
    s, b = q.api(c, "POST", "/forgot-password", {"email": EMAIL})
    print("forgot-password:", s, q.extract_msg(b) or q._short(b, 90))
    otp, src = q.mailpit_otp(EMAIL)
    print("otp:", otp, "via", src)
    if not otp:
        print("FAILED: no OTP captured")
        return 1
    print("verify-reset-otp:", q.api(c, "POST", "/verify-reset-otp",
                                    {"email": EMAIL, "otp": otp})[0])
    s3, b3 = q.api(c, "POST", "/reset-password",
                   {"email": EMAIL, "otp": otp,
                    "new_password": q.QA_PASSWORD,
                    "confirm_password": q.QA_PASSWORD})
    print("reset-password:", s3, q.extract_msg(b3) or q._short(b3, 90))
    chk = q.Client("restore_check")
    s4 = chk.login("qa_admin")[0]
    print("login qa_admin with the seeded password:", s4,
          "(303 = restored)" if s4 == 303 else "(STILL WRONG)")
    return 0 if s4 == 303 else 1


if __name__ == "__main__":
    raise SystemExit(main())
