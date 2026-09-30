"""_qa_probe_aid_review.py - explain the 422 on Under Review -> Approved."""
import importlib.util

spec = importlib.util.spec_from_file_location("qrt", "scripts/qa_run_tests.py")
m = importlib.util.module_from_spec(spec)
spec.loader.exec_module(m)

admin = m.Client("probe_aid_admin")
print("admin login:", admin.login("qa_admin")[0])
staff = m.Client("probe_aid_staff")
print("staff login:", staff.login("qa_staff")[0])

person = m.ensure_person(admin, "aidprobe")
aid = m.staff_create_aid("aidprobe", person)
print("aid fixture:", aid)

def review(status, amount):
    s, b = admin.request("PATCH", f"/aid-requests/{aid}/review",
                         json_body={"status": status, "approved_amount": amount,
                                    "review_notes": "QA probe"})
    print(f"  review {status} amount={amount} -> HTTP {s}: {b[:220]}")
    return s

review("Under Review", "0")
for amount in ("4500", "4500.00", "1", "0"):
    s = review("Approved", amount)
    if s == 200:
        break
s, b = admin.request("GET", f"/aid-requests/{aid}")
print("re-read status:", m._field_of(b, "status"), "| approved:",
      m._field_of(b, "approved_amount"), "| requested:",
      m._field_of(b, "requested_amount"))
