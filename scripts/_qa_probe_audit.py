"""_qa_probe_audit.py - which mutations produce audit rows?"""
import importlib.util
import json

spec = importlib.util.spec_from_file_location("qrt", "scripts/qa_run_tests.py")
m = importlib.util.module_from_spec(spec)
spec.loader.exec_module(m)

c = m.Client("probe_audit")
print("login:", c.login("qa_admin"))
admin_uid = m.find_user_id("qa_admin")
print("admin uid:", admin_uid)

before = {}
for entity in ("persons", "users", "donors"):
    s, b = c.request("GET", f"/audit-logs?entity={entity}&page_size=5")
    before[entity] = len(m.list_items(b))
    print(f"before {entity}: rows={before[entity]} http={s}")

s, b = c.request("POST", "/persons", json_body={
    "full_name": "QA Audit Probe Person", "nic_passport": "QAPROBEAUDIT1",
    "monthly_income": "100", "phone": "+94770000777",
    "gender": "Other", "address": "audit probe"})
pid = m._created_id(b)
print("create person:", s, pid)

s, b = c.request("POST", "/donors", json_body={
    "name": "QA Audit Probe Donor", "donor_type": "Individual",
    "nic_passport": "QAPROBEAUDIT2", "phone": "+94770000778",
    "email": "qa_audit_probe@example.com"})
did = m._created_id(b)
print("create donor:", s, did)

for entity, rid in (("persons", pid), ("donors", did)):
    s, b = c.request("GET", f"/audit-logs?entity={entity}&page_size=10")
    rows = m.list_items(b)
    hits = [r for r in rows if str(r.get("entity_id")) == str(rid)]
    print(f"after {entity}: rows={len(rows)} hits_for_new_id={len(hits)}")
    if hits:
        print("   sample:", json.dumps(hits[0])[:300])
    else:
        print("   newest:", json.dumps(rows[0])[:300] if rows else "(none)")

s, b = c.request("GET", f"/audit-logs?action=CREATE&user_id={admin_uid}&page_size=10")
print("admin CREATE rows:", len(m.list_items(b)))
