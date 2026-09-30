"""_qa_probe_person.py - why do person-id lookups pick the wrong UUID?"""
import importlib.util
import json

spec = importlib.util.spec_from_file_location("qrt", "scripts/qa_run_tests.py")
m = importlib.util.module_from_spec(spec)
spec.loader.exec_module(m)

c = m.Client("probe_person")
status, body = c.login("qa_admin")
print("login:", status, body[:120])

payload = {
    "full_name": "QA Probe Person 99887766",
    "nic_passport": "QPPROBE99887766",
    "monthly_income": "500", "phone": "+94770000009",
    "gender": "Other", "address": "probe",
}
s, b = c.request("POST", "/persons", json_body=payload)
print("POST /persons ->", s)
print("body:", b[:600])

first = m._first_uuid(b)
print("_first_uuid ->", first)
try:
    data = json.loads(b).get("data") or {}
    print("data.id     ->", data.get("id"))
    print("id match    ->", str(data.get("id")) == str(first))
except Exception as exc:  # pragma: no cover - diagnostic only
    print("json parse failed:", exc)

pid, s2, b2 = m._create_and_id(c, "/persons", {
    "full_name": "QA Probe Person 99887777",
    "nic_passport": "QPPROBE99887777",
    "monthly_income": "500",
})
print("_create_and_id ->", pid, s2)
print("GET /persons/<id> ->", c.request("GET", f"/persons/{pid}")[0])

# student creation with the resolved person id
s3, b3 = c.request("POST", "/students", json_body={
    "person_id": pid, "full_name": "QA Probe Student 99887777",
    "school_name": "Probe School", "grade": "10", "academic_year": 2026,
})
print("POST /students with that id ->", s3, b3[:200])
