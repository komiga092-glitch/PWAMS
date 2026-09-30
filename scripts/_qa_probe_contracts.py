"""_qa_probe_contracts.py - dump live responses for the failing QA cases."""
import importlib.util
import json

spec = importlib.util.spec_from_file_location("qrt", "scripts/qa_run_tests.py")
m = importlib.util.module_from_spec(spec)
spec.loader.exec_module(m)

c = m.Client("probe_contracts")
c.login("qa_admin")


def show(label, status, body, limit=700):
    print(f"--- {label}: HTTP {status}")
    print(body[:limit].replace("\\n", " "))


show("GET /dashboard/stats", *c.request("GET", "/dashboard/stats"))
show("GET /donations?page_size=2", *c.request("GET", "/donations?page_size=2"), limit=400)
show("GET /loans?page_size=2", *c.request("GET", "/loans?page_size=2"), limit=400)

# donation update: donor_id vs person_id
s, b = c.request("POST", "/donors", json_body={
    "name": "QA Probe Donor 1", "donor_type": "Individual",
    "phone": "+94770000002", "email": "qa_probe_donor1@example.com",
})
did = m._created_id(b)
s, b = c.request("POST", "/donations", json_body={
    "donor_id": did, "donation_type": "Cash", "amount": "100",
    "quantity": "1", "currency": "LKR", "description": "QA probe donation",
    "donation_date": "2026-09-05",
})
show("POST /donations", s, b, 400)
don = m._created_id(b)
for label, payload in (
    ("update with donor_id", {"donor_id": did, "donation_type": "Cash",
                              "amount": "101", "quantity": "1", "currency": "LKR",
                              "description": "QA probe donation",
                              "donation_date": "2026-09-05"}),
    ("update with person_id", {"person_id": "", "donation_type": "Cash",
                               "amount": "102", "quantity": "1", "currency": "LKR",
                               "description": "QA probe donation",
                               "donation_date": "2026-09-05"}),
):
    show(f"PUT /donations/<id> {label}", *c.request("PUT", f"/donations/{don}", json_body=payload), 300)
show("PUT /donations/<random>", *c.request("PUT", f"/donations/{m._uid()}", json_body={
    "donor_id": did, "donation_type": "Cash", "amount": "103", "quantity": "1",
    "currency": "LKR", "description": "QA probe", "donation_date": "2026-09-05"}), 300)

# aid review contract
s, b = c.request("POST", "/aid-requests", json_body={
    "person_id": m.ensure_person(c, "probe"), "aid_type": "Medical", "priority": "High",
    "title": "QA probe aid", "description": "QA probe aid description",
    "requested_amount": "500", "currency": "LKR", "request_date": "2026-09-06",
})
show("POST /aid-requests", s, b, 300)
aid = m._created_id(b)
for label, payload in (
    ("status Approved", {"status": "Approved", "approved_amount": "500",
                         "review_notes": "probe"}),
    ("decision approve", {"decision": "approve", "approved_amount": "500",
                          "review_notes": "probe"}),
):
    show(f"PATCH /aid-requests/<id>/review {label}",
         *c.request("PATCH", f"/aid-requests/{aid}/review", json_body=payload), 300)
show("DELETE /aid-requests/<id>", *c.request("DELETE", f"/aid-requests/{aid}"), 300)
print("json summary:", json.dumps({"aid": aid, "donation": don}, indent=0))
