"""_qa_probe_partd.py - live contract probe for audit/reports/files/sync/i18n."""
import importlib.util
import json
import subprocess
import tempfile
import uuid
from pathlib import Path

spec = importlib.util.spec_from_file_location("qrt", "scripts/qa_run_tests.py")
m = importlib.util.module_from_spec(spec)
spec.loader.exec_module(m)

REPO = m.REPO
BASE = m.BASE

c = m.Client("probe_d")
print("login:", c.login("qa_admin"))


def show(label, status, body, limit=500):
    print(f"--- {label}: HTTP {status}")
    print(str(body)[:limit].replace("\n", " "))


def raw_curl(args, jar):
    cmd = ["curl.exe", "-s", "--max-time", "60", "-b", str(jar), "-c", str(jar),
           "-w", "\n%{http_code}"] + args
    out = subprocess.run(cmd, capture_output=True, text=True, timeout=90)
    text = out.stdout or ""
    head, _, code = text.rpartition("\n")
    return int(code or 0), head


def upload(client, name, content, content_type):
    tmp = Path(tempfile.gettempdir()) / name
    tmp.write_bytes(content)
    args = ["-X", "POST", "-H", f"X-CSRF-Token: {client.csrf}",
            "-F", f"file=@{tmp};type={content_type}", BASE + "/files/upload"]
    return raw_curl(args, client.jar)


def download(client, file_id, dest):
    args = ["-X", "GET", "-o", str(dest), BASE + f"/files/{file_id}"]
    return raw_curl(args, client.jar)


# ---- audit -------------------------------------------------------------
show("GET /audit-logs?page_size=2", *c.request("GET", "/audit-logs?page_size=2"))
s, b = c.request("GET", "/audit-logs/page"); print(f"--- GET /audit-logs/page: HTTP {s}, html={len(b)}")

# ---- reports -----------------------------------------------------------
show("GET /api/reports/donations", *c.request("GET", "/api/reports/donations"))
show("GET /api/reports/donations?from=2020-01-01&to=2020-12-31",
     *c.request("GET", "/api/reports/donations?from=2020-01-01&to=2020-12-31"), 400)
s, b = c.request("GET", "/reports/page"); print(f"--- GET /reports/page: HTTP {s}, html={len(b)}")
s, b = c.request("GET", "/reports/donations/page"); print(f"--- GET /reports/donations/page: HTTP {s}, html={len(b)}")

# pdf
with tempfile.NamedTemporaryFile(suffix=".pdf", delete=False) as fh:
    pdf = Path(fh.name)
s, _b = raw_curl(["-X", "GET", "-D", "-", "-o", str(pdf),
                  BASE + "/reports/donations/pdf"], c.jar)
magic = pdf.read_bytes()[:5] if pdf.exists() else b""
print(f"--- GET /reports/donations/pdf -> HTTP {s}, magic={magic!r}, "
      f"size={pdf.stat().st_size if pdf.exists() else 0}")

# ---- files -------------------------------------------------------------
s, b = c.request("GET", "/files/page"); print(f"--- GET /files/page: HTTP {s}, html={len(b)}")
show("GET /files?page_size=5", *c.request("GET", "/files?page_size=5"))
png = bytes.fromhex("89504e470d0a1a0a0000000d49484452000000010000000108060000001f15c4"
                    "890000000a49444154789c6360000002000100ffff03000006000557bfabd400"
                    "00000049454e44ae426082")
show("POST /files/upload png", *upload(c, "qa_probe.png", png, "image/png"))
show("POST /files/upload txt", *upload(c, "qa_probe.txt", b"hello text", "text/plain"))
s, b = c.request("GET", "/files?page_size=5")
items = m.list_items(b)
fid = str(items[0]["id"]) if items else None
print(f"--- first file id={fid}")
if fid:
    with tempfile.NamedTemporaryFile(suffix=".png", delete=False) as fh:
        dest = Path(fh.name)
    ds, _db = download(c, fid, dest)
    print(f"--- GET /files/{fid} -> HTTP {ds}, bytes="
          f"{dest.stat().st_size if dest.exists() else 0}")

# ---- sync --------------------------------------------------------------
op = {
    "id": str(uuid.uuid4()), "entity_type": "person", "operation": "CREATE",
    "record_id": str(uuid.uuid4()), "client_version": 1,
    "payload": {"full_name": "QA Sync Probe", "nic_passport": f"QSP{uuid.uuid4().hex[:10]}",
                "monthly_income": "300", "phone": "+94770000111",
                "gender": "Other", "address": "sync probe"},
}
show("POST /api/v1/sync/push CREATE", *c.request(
    "POST", "/api/v1/sync/push", json_body={"operations": [op]},
    headers={"Idempotency-Key": f"qa-probe-{op['id']}"}))
op2 = dict(op, id=str(uuid.uuid4()), operation="UPDATE", client_version=1)
show("POST /api/v1/sync/push UPDATE v1", *c.request(
    "POST", "/api/v1/sync/push", json_body={"operations": [op2]},
    headers={"Idempotency-Key": f"qa-probe-{op2['id']}"}))
show("GET /api/v1/sync/pull?limit=3", *c.request("GET", "/api/v1/sync/pull?limit=3"))
show("POST /api/v1/sync/push malformed", *c.request(
    "POST", "/api/v1/sync/push", json_body={"operations": []},
    headers={"Idempotency-Key": f"qa-probe-bad-{uuid.uuid4().hex[:6]}",
             "Content-Type": "application/json"}))

# ---- i18n --------------------------------------------------------------
s, b = c.request("GET", "/reports/donations/page?lang=si")
si_marks = sum(1 for ch in b if "à¶" <= ch <= "à·¿")
print(f"--- GET /reports/donations/page?lang=si -> HTTP {s}, sinhala_chars={si_marks}")
s, b = c.request("GET", "/reports/donations/page?lang=ta")
ta_marks = sum(1 for ch in b if "à®€" <= ch <= "à¯¿")
print(f"--- GET /reports/donations/page?lang=ta -> HTTP {s}, tamil_chars={ta_marks}")
print("--- jar lang cookie:",
      [ln for ln in c.jar.read_text(errors="ignore").splitlines()
       if "pwams_lang" in ln])


