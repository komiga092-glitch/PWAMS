"""PWAMS automated test execution harness v2.

Executes every test case in PWAMS_Manual_Testing_Report (2).xlsx against the
local instance (127.0.0.1:8081), records Actual Result / Status / Tester /
Date / Remarks into the workbook, refreshes Summary counts, appends evidence
to the Automation Evidence sheet, and emits docs/TEST_EXECUTION_REPORT.md.

Environment facts baked into this harness:
- Server: qa_server.exe on APP_PORT=8081. Rate limiters are in-memory
  (login 5/15m, OTP 3/10m, password-reset 3/15m, writes 30/min/IP) so
  restart_server() resets budgets proactively and reactively. Sessions are
  DB-backed and CSRF is stateless double-submit -> restarts keep clients
  logged in.
- SMTP: Mailpit on 127.0.0.1:1025 (server launched with SMTP_* env).
  OTPs are read from the Mailpit API (delivery proof), DB as fallback.
- QA accounts from cmd/qaseed (password QaPassw0rd!2026); preflight re-runs
  qaseed to clear lockouts and failed-login counters.
- CSRF: pwams_csrf cookie echoed via X-CSRF-Token header on unsafe methods.
"""
import json
import atexit
import collections
import os
import re
import subprocess
import sys
import time
import uuid
from datetime import date
from pathlib import Path

try:
    import psycopg2
except Exception:
    psycopg2 = None

import openpyxl
from openpyxl.styles import Alignment

BASE = "http://127.0.0.1:8081"
MAILPIT_API = "http://127.0.0.1:8025/api/v1"
QA_PASSWORD = "QaPassw0rd!2026"
REPO = Path(__file__).resolve().parent.parent
XLSX = REPO / "PWAMS_Manual_Testing_Report (2).xlsx"
TODAY = date.today().isoformat()
TESTER = "Cline (automated agent, HTTP-level execution)"
ENV_NOTE = (
    "Executed against LOCAL DEV instance 127.0.0.1:8081 built from this repo "
    "(commit 4c8a8ba); Postgres localhost:5432 pwams_db; SMTP via Mailpit :1025. "
    "NOT the agreed staging/live environment - re-verify there before sign-off."
)
ROLE_USERS = {
    "Super Admin": "qa_super_admin",
    "Admin": "qa_admin",
    "Manager": "qa_manager",
    "Staff": "qa_staff",
    "Volunteer": "qa_volunteer",
    "Donor": "qa_donor",
    "Beneficiary": "qa_beneficiary",
    "Student": "qa_student",
}

# rate-limit bucket counters (reset by restart_server)
_logins = 0
_writes = 0
_reset_bucket = 0    # /forgot-password + /reset-password share resetLimiter
_otp_verify_bucket = 0

STATE = {
    "pw_username": None,
    "pw_email": None,
    "pw_password": "ProbeInit1!",   # evolves: AUTH-011 -> USR-007 -> USR-009
    "user_ids": None,               # username -> uuid (from GET /users)
    "msg_sent_id": None,            # admin -> staff message id
    "msg_received_id": None,        # staff -> admin message id (for read/delete)
    "msg_third_id": None,           # staff -> donor message id (for MSG-010)
    "file_a_id": None,
    "file_b_id": None,
    "notif_admin_id": None,
    "notif_other_id": None,
    "donation_probe_id": None,
    "revenue_fixture_id": None,
    "student_fixture_id": None,
    "student_name": None,
    "student_school": None,
    "person_fixture_id": None,
    "person_name": None,
    "donor_fixture_id": None,
    "donor_name": None,
    "aid_fixture_id": None,
    "care_fixture_id": None,
    "loan_fixture_id": None,
    "rep_ids": {},
    "entity_probe_id": None,
}
FIX = {}

# ---------------------------------------------------------------------------
# small helpers
# ---------------------------------------------------------------------------
def fmt_status(s):
    return f"HTTP {s}"


def extract_msg(body):
    try:
        return json.loads(body).get("message", body[:160])
    except Exception:
        return body[:160].replace("\n", " ")


def json_ok(body):
    try:
        return json.loads(body).get("success") is True
    except Exception:
        return False


def _uid():
    return str(uuid.uuid4())


def _http(method, url, payload=None):
    cmd = ["curl.exe", "-s", "-w", "\n%{http_code}", "-X", method, url]
    if payload is not None:
        cmd += ["-H", "Content-Type: application/json", "--data", json.dumps(payload)]
    out = subprocess.run(cmd, capture_output=True, timeout=30).stdout.decode("utf-8", "replace")
    body, _, code = out.rpartition("\n")
    try:
        st = int(code.strip())
    except ValueError:
        st = 0
    return st, body


def mailpit_otp(email, keyword=None, tries=24):
    """Fetch latest OTP delivered to email via Mailpit API (delivery proof)."""
    for _ in range(tries):
        try:
            s, b = _http("GET", MAILPIT_API + "/messages?limit=10")
            if s == 200:
                for m in json.loads(b).get("messages", []):
                    tos = json.dumps(m.get("To", [])) + m.get("Subject", "")
                    if email.lower() in tos.lower():
                        if keyword and keyword.lower() not in (m.get("Subject", "") + "" ).lower():
                            continue
                        s2, b2 = _http("GET", MAILPIT_API + "/message/" + str(m.get("ID", "")))
                        if s2 == 200:
                            text = json.loads(b2).get("Text", "") or ""
                            mm = re.search(r"\b(\d{6})\b", text)
                            if mm:
                                return mm.group(1), f"mailpit message {m.get('ID')}"
        except Exception:
            pass
        time.sleep(0.5)
    # DB fallback: plaintext OTP column (same machine, white-box)
    otp = db_scalar(
        "SELECT t.otp FROM password_reset_tokens t JOIN users u ON u.id=t.user_id "
        "WHERE u.email=%s AND t.expires_at > now() ORDER BY t.created_at DESC LIMIT 1",
        (email,),
    )
    if otp:
        return otp, "DB password_reset_tokens"
    otp = db_scalar(
        "SELECT t.otp FROM account_activation_tokens t JOIN users u ON u.id=t.user_id "
        "WHERE u.email=%s AND t.expires_at > now() ORDER BY t.created_at DESC LIMIT 1",
        (email,),
    )
    if otp:
        return otp, "DB account_activation_tokens"
    return None, "none"


def db_scalar(sql, args=()):
    if psycopg2 is None:
        return None
    try:
        conn = psycopg2.connect(host="localhost", port=5432, user="pwams_user",
                                password="Pwams@2026Secure", dbname="pwams_db",
                                connect_timeout=5)
        cur = conn.cursor()
        if args:
            cur.execute(sql, args)
        else:
            # no params: execute raw so literal '%' (e.g. LIKE 'qa_%') is not
            # treated as a psycopg2 placeholder
            cur.execute(sql)
        row = cur.fetchone()
        conn.close()
        return row[0] if row else None
    except Exception:
        return None


def db_exec(sql, args=()):
    if psycopg2 is None:
        return False
    try:
        conn = psycopg2.connect(host="localhost", port=5432, user="pwams_user",
                                password="Pwams@2026Secure", dbname="pwams_db",
                                connect_timeout=5)
        cur = conn.cursor()
        if args:
            cur.execute(sql, args)
        else:
            cur.execute(sql)
        conn.commit()
        conn.close()
        return True
    except Exception:
        return False


def unlock_qa_accounts(tag="", usernames=None):
    """Clear failed-login counters / lockouts on the seeded QA accounts.

    ``AuthService.Login`` locks an account for 30 minutes after 3 failed
    attempts (internal/services/auth_service.go) and the failed-login cases
    (AUTH-003, AUTH-005) therefore lock ``qa_admin`` in the middle of a run:
    every later case that needs an Admin session then cascades into
    "Blocked".  Re-running cmd/qaseed clears the counters but is slow, so the
    harness clears them directly in Postgres before the run and after every
    server recycle.  Only rows that are actually locked/incremented are
    touched, and a non-'Locked' status (e.g. the deliberately Inactive
    accounts used by ACT-00x) is left alone.
    """
    if usernames:
        sql = ("UPDATE users SET status = CASE WHEN status = 'Locked' THEN 'Active' "
               "ELSE status END, failed_login_attempts = 0, locked_until = NULL "
               "WHERE username = ANY(%s)")
        args = (list(usernames),)
    else:
        sql = ("UPDATE users SET status = CASE WHEN status = 'Locked' THEN 'Active' "
               "ELSE status END, failed_login_attempts = 0, locked_until = NULL "
               "WHERE username LIKE 'qa\\_%' AND (status = 'Locked' "
               "OR locked_until IS NOT NULL OR failed_login_attempts <> 0)")
        args = ()
    ok = db_exec(sql, args)
    if tag:
        print(f"[unlock] QA account lockouts/counters cleared ({tag}) ok={ok}",
              flush=True)
    return ok


def find_user_id(username):
    """Find uuid id for username via GET /users (cached)."""
    if STATE["user_ids"] is None:
        _fetch_user_ids()
    ids = STATE["user_ids"] or {}
    if username in ids:
        return ids[username]
    # The user table is far larger than one page (600+ rows in QA), so a
    # single unfiltered page can miss the account: fall back to a search.
    admin = _session("Admin")
    if admin is None:
        return None
    _s, b = admin.request("GET", f"/users?search={username}&page_size=20")
    for it in list_items(b):
        if isinstance(it, dict) and it.get("username") == username and it.get("id"):
            ids[username] = str(it["id"])
            STATE["user_ids"] = ids
            return ids[username]
    return None


def _session(role):
    return ROLE_SESSIONS.get(role)


def _fetch_user_ids():
    admin = _session("Admin")
    if admin is None:
        STATE["user_ids"] = {}
        return
    s, b = admin.request("GET", "/users?page_size=100")
    ids = {}
    try:
        data = json.loads(b)
    except Exception:
        data = []

    def walk(o):
        if isinstance(o, dict):
            if "username" in o and ("id" in o or "ID" in o):
                ids[o.get("username")] = str(o.get("id") or o.get("ID"))
            for v in o.values():
                walk(v)
        elif isinstance(o, list):
            for v in o:
                walk(v)
    walk(data)
    STATE["user_ids"] = ids
    return ids


def _created_id(body):
    """Created record id from a create response.

    Never returns a foreign-key UUID: the API marshals response maps
    alphabetically, so ``created_by_id`` can precede the record's own ``id``
    in the raw JSON. Structured lookups (data/record/<entity>.id) are tried
    first and ``_first_uuid`` is only the last resort.
    """
    try:
        data = json.loads(body)
    except Exception:
        return _first_uuid(body)
    if isinstance(data, dict):
        for key in ("data", "record", "item", "result"):
            node = data.get(key)
            if isinstance(node, dict) and node.get("id"):
                return str(node["id"])
        for value in data.values():
            if isinstance(value, dict) and value.get("id"):
                return str(value["id"])
        if data.get("id"):
            return str(data["id"])
    return _first_uuid(body)


def list_items(body):
    """Tolerant extractor of the item array from a list endpoint response."""
    try:
        data = json.loads(body)
    except Exception:
        return []
    if isinstance(data, list):
        return data
    if isinstance(data, dict):
        for k in ("data", "items", "rows", "users", "persons", "students", "donors",
                  "donations", "loans", "notifications", "messages", "logs", "audit_logs"):
            v = data.get(k)
            if isinstance(v, list):
                return v
        for v in data.values():
            if isinstance(v, list):
                return v
        # some handlers nest the row set one level deeper, e.g.
        # {"data": {"loans": [...], "total": n}} - walk into nested objects.
        queue = [v for v in data.values() if isinstance(v, dict)]
        seen = 0
        while queue and seen < 50:
            node = queue.pop(0)
            seen += 1
            for v in node.values():
                if isinstance(v, list):
                    return v
            queue.extend(v for v in node.values() if isinstance(v, dict))
    return []


# ---------------------------------------------------------------------------
# server lifecycle (rate-limiters are in-memory; restart resets them.
# sessions are DB-backed and CSRF is stateless => restart is transparent)
# ---------------------------------------------------------------------------
def restart_server():
    global _logins, _writes, _reset_bucket, _otp_verify_bucket
    print("[restart] stopping listener on :8081", flush=True)
    try:
        proc = subprocess.run(
            ["powershell", "-NoProfile", "-Command",
             "(Get-NetTCPConnection -State Listen -LocalPort 8081 -ErrorAction SilentlyContinue)"
             ".OwningProcess | ForEach-Object { Stop-Process -Id $_ -Force -ErrorAction SilentlyContinue }"],
            capture_output=True, timeout=30,
        )
        print(f"[restart] kill rc={proc.returncode}", flush=True)
    except Exception as e:
        print(f"[restart] kill failed: {e}")
    time.sleep(1.5)
    env = dict(os.environ)
    env.update({
        "APP_PORT": "8081",
        "SMTP_HOST": "127.0.0.1",
        "SMTP_PORT": "1025",
        "SMTP_USERNAME": "qa",
        "SMTP_PASSWORD": "qa",
        "SMTP_FROM": "noreply@pwams.local",
    })
    logf = open(REPO / "qa_server_run.log", "ab")
    exe = REPO / "bin" / "qa_server.exe"
    if not exe.exists():
        exe = REPO / "qa_server.exe"
    subprocess.Popen([str(exe)], cwd=str(REPO), env=env,
                     stdout=logf, stderr=logf)
    # wait for health
    probe = Client("healthprobe")
    for _ in range(60):
        s, _b = probe._send("GET", "/health", None, None, None)
        if s == 200:
            break
        time.sleep(1)
    else:
        print("[restart] WARNING: /health not OK after 60s")
    _logins = _writes = _reset_bucket = _otp_verify_bucket = 0
    unlock_qa_accounts("server recycle")
    print("[restart] server recycled, rate-limit buckets cleared")


# ---------------------------------------------------------------------------
# budget gates: restart BEFORE a bucket would overflow (limits:
# login 5/15m, otp 3/10m, reset 3/15m incl forgot, writes 30/min)
# ---------------------------------------------------------------------------
_GATE_OFF = False   # set True by tests that must OBSERVE a 429 themselves


def _gate(kind):
    if _GATE_OFF:
        return
    if kind == "login" and _logins >= 4:
        restart_server()
    elif kind == "otp" and _otp_verify_bucket >= 2:
        restart_server()
    elif kind == "reset" and _reset_bucket >= 2:
        restart_server()
    elif kind == "write" and _writes >= 25:
        restart_server()


# ---------------------------------------------------------------------------
# result recording + request helpers shared by every qa_exec_*.py part
# (they live here so a single part can be executed on its own through
#  `python scripts/qa_run_tests.py c`).
# ---------------------------------------------------------------------------
RESULTS = {}
REMARK_OK = (
    "Executed against local dev instance 127.0.0.1:8081 built from this repo "
    "source; Postgres localhost:5432; SMTP via Mailpit 127.0.0.1:1025. "
    "Session/CSRF/rate-limit behaviour exercised through a real cookie jar."
)


def record(tid, status, actual, remarks=None):
    RESULTS[tid] = {
        "status": status,
        "actual": actual,
        "remarks": remarks or REMARK_OK,
    }
    RUN_IDS.add(tid)
    print(f"[{tid}] {status}: {actual}", flush=True)


def _ok(cond):
    return "Pass" if cond else "Fail"


def page(c, path, method="GET", body=None):
    """Browser-style request -> pages answer 200, unauth answers 303."""
    return c.request(method, path, json_body=body,
                     headers={"Accept": "text/html"})


def api(c, method, path, body=None, recover=True):
    """API-style request -> APIs answer 200/201/4xx, unauth answers 401."""
    return c.request(method, path, json_body=body,
                     headers={"Accept": "application/json"}, recover=recover)


def anon(tag="anon"):
    return Client(f"anon_{tag}")


def _me(c):
    s, b = api(c, "GET", "/auth/me")
    try:
        return s, json.loads(b)
    except Exception:
        return s, {}


def _short(text, n=220):
    return " ".join(str(text).split())[:n]


class Client:
    """Cookie+CSRF client with its own cookie jar file."""

    def __init__(self, name=None):
        self.name = name or uuid.uuid4().hex[:8]
        _tmp = REPO / "qa_tmp"
        _tmp.mkdir(exist_ok=True)
        self.jar = _tmp / f"qa_jar_{self.name}.txt"
        if self.jar.exists():
            self.jar.unlink()
        self.jar.touch()
        self.csrf = ""
        # prime: GET /login issues the pwams_csrf cookie (double-submit pair)
        self._send("GET", "/login", None, None, None)
        self._sync_csrf()

    def _sync_csrf(self):
        try:
            m = re.search(r"pwams_csrf\s+(\S+)", self.jar.read_text(errors="ignore"))
            if m:
                self.csrf = m.group(1)
        except Exception:
            pass

    def _send(self, method, path, data, json_body, headers):
        global _logins, _writes, _reset_bucket, _otp_verify_bucket
        # budget gates (GET never counted by server limiters)
        if method != "GET":
            if path == "/login":
                _gate("login")
            elif path == "/verify-reset-otp":
                _gate("otp")
            elif path in ("/forgot-password", "/reset-password"):
                _gate("reset")
            else:
                _gate("write")
        cmd = ["curl.exe", "-s", "--max-time", "60",
               "-b", str(self.jar), "-c", str(self.jar),
               "-w", "\n%{http_code}", "-X", method, BASE + path]
        if self.csrf:
            cmd += ["-H", f"X-CSRF-Token: {self.csrf}"]
        if json_body is not None:
            cmd += ["-H", "Content-Type: application/json", "--data", json.dumps(json_body)]
        elif data:
            for k, v in data.items():
                cmd += ["--data-urlencode", f"{k}={v}"]
        for k, v in (headers or {}).items():
            cmd += ["-H", f"{k}: {v}"]
        out = subprocess.run(cmd, capture_output=True, timeout=90).stdout.decode("utf-8", "replace")
        body, _, code = out.rpartition("\n")
        try:
            status = int(code.strip())
        except ValueError:
            status = 0
        self._sync_csrf()
        # account consumed budget exactly like the server does
        if method in ("POST", "PUT", "PATCH", "DELETE"):
            _writes += 1
            if path == "/login":
                _logins += 1
            elif path == "/verify-reset-otp":
                _otp_verify_bucket += 1
            elif path in ("/forgot-password", "/reset-password"):
                _reset_bucket += 1
        return status, body

    def request(self, method, path, data=None, json_body=None, headers=None,
                recover=True):
        """Send request; on unexpected 429, recycle server once and retry."""
        status, body = self._send(method, path, data, json_body, headers)
        if status == 429 and recover:
            print(f"[429-recover] {method} {path} -> recycling server")
            restart_server()
            status, body = self._send(method, path, data, json_body, headers)
        return status, body

    def login(self, username):
        return self.request("POST", "/login",
                            json_body={"login": username, "password": QA_PASSWORD})

    def logout(self):
        return self.request("POST", "/logout")


ROLE_SESSIONS = {}


def login_as(role):
    """Cached per-role session (sessions survive restarts: DB-backed)."""
    if role in ROLE_SESSIONS:
        return ROLE_SESSIONS[role], True, 0, ""
    c = Client(f"role_{role.replace(' ', '_')}")
    status, body = c.login(ROLE_USERS[role])
    ok = (status == 303) or ('"success": true' in body) or ('"success":true' in body)
    if ok:
        ROLE_SESSIONS[role] = c
    return c, ok, status, body


# ---------------------------------------------------------------------------
# fixtures: idempotent setup records reused across modules
# ---------------------------------------------------------------------------
FIX = {}


def _create_and_id(admin, path, payload, match_key="name"):
    """POST payload; return created id (from response or list lookup)."""
    s, b = admin.request("POST", path, json_body=payload)
    if s not in (200, 201) or not json_ok(b):
        return None, s, b
    jid = _created_id(b)
    if jid:
        return jid, s, b
    try:
        d = json.loads(b)
        cand = d.get("data") or d.get("record") or d
        if isinstance(cand, dict) and cand.get("id"):
            return str(cand["id"]), s, b
    except Exception:
        pass
    # fallback: search list for matching field value
    needle = (payload.get(match_key) or payload.get("full_name")
              or payload.get("title") or payload.get("name")
              or payload.get("description") or payload.get("subject"))
    s2, b2 = admin.request("GET", path + "?page_size=100")
    for it in list_items(b2):
        if not isinstance(it, dict):
            continue
        for k in (match_key, "full_name", "title", "name", "username", "email"):
            if needle and it.get(k) == needle and it.get("id"):
                return str(it["id"]), s, b
    return "created", s, b   # id unknown but create succeeded


def ensure_person(admin, tag="qa_fx"):
    if "person" in FIX:
        return FIX["person"]
    name = f"QA Fixture Person {tag}"
    nic = f"QAF{int(time.time()) % 100000000}"
    pid, _, _ = _create_and_id(admin, "/persons", {
        "full_name": name, "nic_passport": nic,
        "monthly_income": "1000", "phone": "+94771234567",
        "gender": "Other", "address": "QA fixture address",
    })
    FIX["person"] = pid
    FIX["person_name"] = name
    return pid


def ensure_donor(admin):
    if "donor" in FIX:
        return FIX["donor"]
    name = f"QA Fixture Donor {int(time.time()) % 1000000}"
    did, _, _ = _create_and_id(admin, "/donors", {
        "name": name, "donor_type": "Individual",
        "phone": "+94770000001", "email": "qa_fixture_donor@example.com",
        "address": "QA fixture", "notes": "auto fixture",
    })
    FIX["donor"] = did
    FIX["donor_name"] = name
    return did


def ensure_student(admin):
    if "student" in FIX:
        return FIX["student"]
    person_id = ensure_person(admin, "student")
    if not person_id:
        return None
    name = f"QA Fixture Student {int(time.time()) % 1000000}"
    sid, _, _ = _create_and_id(admin, "/students", {
        "person_id": person_id, "full_name": name,
        "school_name": "QA Test School", "grade": "10",
        "academic_year": 2026, "gender": "Other",
        "guardian_name": "QA Guardian",
    }, match_key="full_name")
    FIX["student"] = sid
    FIX["student_name"] = name
    return sid


def ensure_donation(admin):
    if "donation" in FIX:
        return FIX["donation"]
    donor_id = ensure_donor(admin)
    if not donor_id:
        return None
    did, _, _ = _create_and_id(admin, "/donations", {
        "donor_id": donor_id, "donation_type": "Cash",
        "amount": "5000", "quantity": "1", "currency": "LKR",
        "description": "QA fixture donation", "donation_date": "2026-09-01",
    })
    FIX["donation"] = did
    return did


def ensure_aid(admin):
    if "aid" in FIX:
        return FIX["aid"]
    person_id = ensure_person(admin, "aid")
    if not person_id:
        return None
    title = f"QA Fixture Aid {int(time.time()) % 1000000}"
    aid, _, _ = _create_and_id(admin, "/aid-requests", {
        "person_id": person_id, "aid_type": "Medical", "priority": "Medium",
        "title": title, "description": "QA fixture aid description",
        "requested_amount": "2500", "currency": "LKR",
        "request_date": "2026-09-01",
    }, match_key="title")
    FIX["aid"] = aid
    FIX["aid_title"] = title
    return aid


def ensure_care(admin):
    if "care" in FIX:
        return FIX["care"]
    person_id = ensure_person(admin, "care")
    aid_id = ensure_aid(admin)
    if not person_id or not aid_id:
        return None
    cid, _, _ = _create_and_id(admin, "/care-provided", {
        "aid_request_id": aid_id, "person_id": person_id,
        "amount": 750, "care_type": "medical",
        "description": "QA fixture care", "provided_by": "QA Bot",
        "provided_at": "2026-09-15T10:00:00Z",
    })
    FIX["care"] = cid
    return cid


def ensure_loan(admin):
    """Loan in Pending status (for approve/reject cases)."""
    if "loan" in FIX:
        return FIX["loan"]
    person_id = ensure_person(admin, "loan")
    if not person_id:
        return None
    lid, _, _ = _create_and_id(admin, "/loans", {
        "person_id": person_id, "loan_amount": "50000",
        "interest_rate": "5", "duration_months": 12,
        "purpose": "QA fixture loan",
    })
    FIX["loan"] = lid
    return lid


def ensure_active_loan(admin):
    """Loan in Active status (repayment prerequisite). Creates+approves+activates."""
    if "active_loan" in FIX:
        return FIX["active_loan"]
    person_id = ensure_person(admin, "loanpay")
    if not person_id:
        return None
    lid, s, b = _create_and_id(admin, "/loans", {
        "person_id": person_id, "loan_amount": "80000",
        "interest_rate": "4", "duration_months": 10,
        "purpose": "QA active loan",
    })
    if not lid or lid == "created":
        return None
    # Pending -> Approved -> Active (same reviewer OK? service forbids own review:
    # CreatedByID == reviewerID rejected, so Super Admin activates what Admin created)
    sa, ok, _, _ = login_as("Super Admin")
    if ok:
        s1, b1 = sa.request("PATCH", f"/loans/{lid}/review",
                            json_body={"status": "Approved", "review_notes": "QA approve"})
        s2, b2 = sa.request("PATCH", f"/loans/{lid}/review",
                            json_body={"status": "Active", "review_notes": "QA activate"})
    FIX["active_loan"] = lid
    return lid


def ensure_repayment(admin):
    """Pending repayment schedule entry on an active loan."""
    if "repayment" in FIX:
        return FIX["repayment"]
    lid = ensure_active_loan(admin)
    if not lid:
        return None
    rid, _, _ = _create_and_id(admin, "/loan-repayments", {
        "loan_id": lid, "installment_number": 1,
        "due_date": "2026-10-01", "amount": "8000",
        "notes": "QA fixture repayment",
    })
    FIX["repayment"] = rid
    return rid


def ensure_revenue(admin):
    if "revenue" in FIX:
        return FIX["revenue"]
    rid, _, _ = _create_and_id(admin, "/revenue", {
        "record_type": "income", "category": "Donations",
        "amount": "15000", "currency": "LKR",
        "record_date": "2026-09-10", "description": "QA fixture revenue",
    })
    FIX["revenue"] = rid
    return rid


def ensure_message(admin):
    if "message" in FIX:
        return FIX["message"]
    staff_id = find_user_id("qa_staff")
    if not staff_id:
        return None
    s, b = admin.request("POST", "/messages", json_body={
        "recipient_id": staff_id, "subject": "QA fixture message",
        "body": "Hello from QA fixture bot.",
    })
    if s not in (200, 201) or not json_ok(b):
        return None
    # find id in sent list
    s2, b2 = admin.request("GET", "/messages/sent?page_size=50")
    for it in list_items(b2):
        if isinstance(it, dict) and it.get("subject") == "QA fixture message" and it.get("id"):
            FIX["message"] = str(it["id"])
            return FIX["message"]
    FIX["message"] = "sent"
    return "sent"


def ensure_notification(admin):
    if "notification" in FIX:
        return FIX["notification"]
    admin_id = find_user_id("qa_admin")
    if not admin_id:
        return None
    s, b = admin.request("POST", "/notifications", json_body={
        "user_id": admin_id, "title": "QA fixture notification",
        "message": "Fixture body for QA tests.", "type": "info",
    })
    if s not in (200, 201) or not json_ok(b):
        return None
    s2, b2 = admin.request("GET", "/notifications?page_size=50")
    for it in list_items(b2):
        if isinstance(it, dict) and it.get("title") == "QA fixture notification" and it.get("id"):
            FIX["notification"] = str(it["id"])
            return FIX["notification"]
    FIX["notification"] = "created"
    return "created"


def ensure_pw_probe(admin):
    """Disposable user for password-change/reset test flows."""
    if "pw_username" in STATE and STATE["pw_username"]:
        return STATE["pw_username"]
    uname = "qa_pwprobe"
    email = "qa_pwprobe@qa.local"
    password = "PwProbe1!xyz"
    find_user_id("qa_admin")   # warm cache
    exists = (STATE.get("user_ids") or {}).get(uname)
    if not exists:
        admin.request("POST", "/users", json_body={
            "username": uname, "email": email,
            "password": password, "role": "Volunteer",
        })
        STATE["user_ids"] = None   # invalidate cache
        find_user_id("qa_admin")
        exists = (STATE.get("user_ids") or {}).get(uname)
    STATE["pw_username"] = uname
    STATE["pw_email"] = email
    STATE["pw_password"] = password
    STATE["pw_id"] = exists
    return uname


# ---------------------------------------------------------------------------
# shared helpers for executors
# ---------------------------------------------------------------------------
def _fresh(name):
    return Client(name)


def _login_ok(status, body):
    return status == 303 or '"success": true' in body or '"success":true' in body


def _denied(status):
    return status in (401, 403, 301, 302, 303)


def _list_total(body, items):
    try:
        d = json.loads(body)
        if isinstance(d, dict):
            for k in ("total", "count", "total_count"):
                if isinstance(d.get(k), int):
                    return d[k]
            pg = d.get("pagination")
            if isinstance(pg, dict):
                for k in ("total_items", "total", "count"):
                    if isinstance(pg.get(k), int):
                        return pg[k]
            # nested envelope, e.g. {"data": {"total": n, "loans": [...]}}
            for v in d.values():
                if isinstance(v, dict):
                    for k in ("total_items", "total", "total_count", "count"):
                        if isinstance(v.get(k), int):
                            return v[k]
    except Exception:
        pass
    return len(items)


def _all_ints(o):
    out = []
    if isinstance(o, bool):
        return out
    if isinstance(o, int):
        out.append(o)
    elif isinstance(o, dict):
        for v in o.values():
            out += _all_ints(v)
    elif isinstance(o, list):
        for v in o:
            out += _all_ints(v)
    return out


def _first_uuid(text):
    m = re.search(r"[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}", text)
    return m.group(0) if m else None


def upload_file(client, path, filepath):
    """multipart file upload via curl -F (writes count against budget)."""
    global _writes
    _gate("write")
    cmd = ["curl.exe", "-s", "--max-time", "60",
           "-b", str(client.jar), "-c", str(client.jar),
           "-w", "\n%{http_code}", "-X", "POST", BASE + path,
           "-H", f"X-CSRF-Token: {client.csrf}",
           "-F", f"file=@{filepath}"]
    out = subprocess.run(cmd, capture_output=True, timeout=120).stdout.decode("utf-8", "replace")
    body, _, code = out.rpartition("\n")
    try:
        status = int(code.strip())
    except ValueError:
        status = 0
    client._sync_csrf()
    _writes += 1
    return status, body


def fresh_person(admin, tag):
    """Always-new person (probe use)."""
    name = f"QA Probe {tag} {int(time.time()) % 1000000}"
    nic = f"QP{abs(hash(name)) % 10000000000:010d}"[:30]
    pid, s, b = _create_and_id(admin, "/persons", {
        "full_name": name, "nic_passport": nic,
        "monthly_income": "500", "phone": "+94770000002",
        "gender": "Other", "address": f"probe {tag}",
    })
    return pid, name


def fresh_donor(admin, tag):
    name = f"QA Probe Donor {tag} {int(time.time()) % 1000000}"
    did, s, b = _create_and_id(admin, "/donors", {
        "name": name, "donor_type": "Individual",
        "phone": "+94770000003", "notes": f"probe {tag}",
    })
    return did, name


def fresh_aid(admin, tag, person_id):
    title = f"QA Probe Aid {tag} {int(time.time()) % 1000000}"
    aid, s, b = _create_and_id(admin, "/aid-requests", {
        "person_id": person_id, "aid_type": "Medical", "priority": "High",
        "title": title, "description": f"probe aid description {tag}",
        "requested_amount": "1500", "currency": "LKR",
        "request_date": "2026-09-18",
    }, match_key="title")
    return aid, title


# aid/care fixtures must be created by Staff so Admin can review them
# (service rejects reviewing your own submission: ErrCannotReviewOwnSubmission)
def staff_create_aid(tag, person_id):
    st, ok, _, _ = login_as("Staff")
    if not ok:
        return None
    title = f"QA Probe Aid {tag} {int(time.time()) % 1000000}"
    aid, s, b = _create_and_id(st, "/aid-requests", {
        "person_id": person_id, "aid_type": "Food", "priority": "Medium",
        "title": title, "description": f"staff-created probe {tag}",
        "requested_amount": "1200", "currency": "LKR",
        "request_date": "2026-09-18",
    }, match_key="title")
    return aid


def staff_create_loan(person_id, purpose):
    st, ok, _, _ = login_as("Staff")
    if not ok:
        return None
    lid, s, b = _create_and_id(st, "/loans", {
        "person_id": person_id, "loan_amount": "50000",
        "interest_rate": "5", "duration_months": 12,
        "purpose": purpose,
    })
    return lid if lid and lid != "created" else None


PERSON_FIELDS = lambda name: {
    "full_name": name, "nic_passport": f"QX{abs(hash(name)) % 10000000000:010d}"[:30],
    "monthly_income": "750", "phone": "+94771112222",
    "gender": "Other", "address": "QA probe address",
}


# ---------------------------------------------------------------------------
# deliverables: qa_results.json -> workbook (xlsx) + docs/TEST_EXECUTION_REPORT.md
# ---------------------------------------------------------------------------
RESULTS_JSON = REPO / "qa_results.json"
REPORT_MD = REPO / "docs" / "TEST_EXECUTION_REPORT.md"
SHEET_SUMMARY = "Summary"
SHEET_EVIDENCE = "Automation Evidence"
NO_EXECUTOR_ACTUAL = ("No automated executor mapped for this case; needs manual "
                      "execution by a human tester on the agreed environment.")
NO_EXECUTOR_REMARK = ("Blocked - not covered by the HTTP-level automation harness "
                      "(browser/visual or environment prerequisite). Equivalent "
                      "logic has Go test coverage in the repo suite; human "
                      "execution still required for sign-off.")
ENV_LINE = ("Local dev instance http://127.0.0.1:8081 built from this repo "
            "(commit 4c8a8ba); PostgreSQL localhost:5432/pwams_db; SMTP sandbox "
            "Mailpit 127.0.0.1:1025 (SMTP AUTH enabled); QA accounts from "
            "cmd/qaseed. NOT a confirmed staging/live environment.")
BASELINE = set()     # ids merged in from a previous run
RUN_IDS = set()      # ids genuinely (re-)executed in this process


def save_results():
    """Persist RESULTS so a later partial run can merge and re-export."""
    try:
        data = {k: {"status": v["status"], "actual": v["actual"],
                    "remarks": v["remarks"]}
                for k, v in sorted(RESULTS.items())}
        RESULTS_JSON.write_text(json.dumps(data, indent=1, ensure_ascii=False),
                                encoding="utf-8")
        print(f"[export] {RESULTS_JSON.name}: {len(data)} results persisted",
              flush=True)
    except Exception as exc:
        print(f"[export] WARNING could not write {RESULTS_JSON.name}: {exc}")


def load_results():
    """Merge a previous run's results (results recorded now always win)."""
    if not RESULTS_JSON.exists():
        return 0
    try:
        data = json.loads(RESULTS_JSON.read_text(encoding="utf-8-sig"))
    except Exception:
        return 0
    for tid, rec in (data or {}).items():
        if isinstance(rec, dict) and rec.get("status"):
            RESULTS.setdefault(tid, {"status": rec["status"],
                                     "actual": rec.get("actual", ""),
                                     "remarks": rec.get("remarks") or REMARK_OK})
            BASELINE.add(tid)
    return len(BASELINE)


def _sheet_case_rows(ws):
    """(row, case id) pairs of a module sheet (column A = Test Case ID)."""
    out = []
    for row in range(2, ws.max_row + 1):
        val = ws.cell(row=row, column=1).value
        if val and re.match(r"^[A-Z][A-Z0-9]*-\d+$", str(val).strip()):
            out.append((row, str(val).strip()))
    return out


def _pass_rate(c):
    done = c["Pass"] + c["Fail"]
    return f"{100.0 * c['Pass'] / done:.0f}%" if done else "n/a"


def _module_sheets():
    """Sheet names holding test cases (everything except the two front sheets)."""
    if not XLSX.exists():
        return []
    wb = openpyxl.load_workbook(XLSX, read_only=True)
    names = [n for n in wb.sheetnames if n not in (SHEET_SUMMARY, SHEET_EVIDENCE)]
    wb.close()
    return names


def write_workbook():
    """Fill Actual Result/Status/Tested By/Date/Remarks + refresh Summary."""
    wb = openpyxl.load_workbook(XLSX)
    per_module, total = {}, collections.Counter()
    for name in wb.sheetnames:
        if name in (SHEET_SUMMARY, SHEET_EVIDENCE):
            continue
        ws = wb[name]
        c = collections.Counter()
        for row, tid in _sheet_case_rows(ws):
            res = RESULTS.get(tid)
            status = res["status"] if res else "Blocked"
            cells = {
                7: res["actual"] if res else NO_EXECUTOR_ACTUAL,
                8: status,
                10: TESTER,
                11: TODAY,
                12: res["remarks"] if res else NO_EXECUTOR_REMARK,
            }
            for col, val in cells.items():
                ws.cell(row=row, column=col, value=val)
            for col in (7, 12):
                ws.cell(row=row, column=col).alignment = Alignment(
                    wrap_text=True, vertical="top")
            c[status] += 1
            total[status] += 1
        per_module[name] = c
    # Summary sheet: environment block + per-module Pass/Fail/Blocked counts
    ws = wb[SHEET_SUMMARY]
    env_block = {
        "Test Environment": ENV_LINE,
        "Tested By": TESTER,
        "Test Period": f"{TODAY} (automated HTTP-level execution session)",
    }
    for row in range(1, ws.max_row + 1):
        if ws.cell(row=row, column=2).value in env_block:
            lab = ws.cell(row=row, column=2).value
            ws.cell(row=row, column=3, value=env_block[lab])
    hdr_row = None
    for row in range(1, ws.max_row + 1):
        if ws.cell(row=row, column=3).value == "Module":
            hdr_row = row
            break
    if hdr_row:
        for offset, title in enumerate(("Pass", "Fail", "Blocked", "Not Executed")):
            ws.cell(row=hdr_row, column=6 + offset, value=title)
        for row in range(hdr_row + 1, ws.max_row + 1):
            sheet = ws.cell(row=row, column=4).value
            if not sheet or ws.cell(row=row, column=3).value in (None, "TOTAL"):
                continue
            c = per_module.get(str(sheet), collections.Counter())
            ws.cell(row=row, column=6, value=c["Pass"])
            ws.cell(row=row, column=7, value=c["Fail"])
            ws.cell(row=row, column=8, value=c["Blocked"])
            ws.cell(row=row, column=9, value=0)
    _write_evidence_sheet(wb, per_module, total)
    try:
        wb.save(XLSX)
        print(f"[export] workbook updated: {XLSX.name}")
    except PermissionError:
        alt = XLSX.with_name(XLSX.stem + " (updated).xlsx")
        wb.save(alt)
        print(f"[export] WARNING {XLSX.name} open in Excel; wrote {alt.name}")
    return per_module, total



def _write_evidence_sheet(wb, per_module, total):
    """Rewrite the Automation Evidence sheet for THIS run (idempotent)."""
    ws = wb[SHEET_EVIDENCE]
    # the interim sheet merged cells across its sections; drop those merges so
    # the rewritten rows can be written (title merges in rows 1-3 are kept).
    for rng in list(ws.merged_cells.ranges):
        if rng.min_row >= 4:
            ws.unmerge_cells(str(rng))
    for row in range(4, ws.max_row + 3):
        for col in range(1, ws.max_column + 8):
            try:
                cell = ws.cell(row=row, column=col)
                if cell.value is not None:
                    cell.value = None
            except AttributeError:
                pass
    carried = sorted(BASELINE - RUN_IDS)
    out = [("",),
           ("PWAMS - Automated Execution Evidence (HTTP-level harness)",),
           ("Source: Cline (automated coding agent) - NOT a human manual-QA pass",),
           ("",),
           ("Run date", TODAY),
           ("Executor", TESTER),
           ("Harness", "scripts/qa_run_tests.py + scripts/qa_exec_a..d.py - curl "
                       "cookie-jar client with stateless CSRF double-submit "
                       "against a real qa_server.exe on 127.0.0.1:8081"),
           ("Target checked", "http://127.0.0.1:8081 (local dev instance)"),
           ("Environment", ENV_LINE),
           ("Cases executed this run", str(len(RUN_IDS))),
           ("Totals", f"cases={sum(total.values())}, pass={total['Pass']}, "
                      f"fail={total['Fail']}, blocked={total['Blocked']}")]
    if carried:
        out.append(("Carried over", f"{len(carried)} case(s) were NOT re-executed "
                                    f"this run and keep the previous run's "
                                    f"result: {', '.join(carried)}"))
    out += [("",), ("Per-module results",),
            ("Module", "Cases", "Pass", "Fail", "Blocked", "Pass rate")]
    for name in sorted(per_module):
        c = per_module[name]
        out.append((name, str(sum(c.values())), str(c["Pass"]), str(c["Fail"]),
                    str(c["Blocked"]), _pass_rate(c)))
    out.append(("TOTAL", str(sum(total.values())), str(total["Pass"]),
                str(total["Fail"]), str(total["Blocked"]), _pass_rate(total)))
    fails = [(t, RESULTS[t]) for t in sorted(RESULTS)
             if RESULTS[t]["status"] == "Fail"]
    out += [("",), (f"Failures observed ({len(fails)}) - product behaviour to review",)]
    if fails:
        for tid, res in fails:
            out.append((tid, _short(res["actual"], 600)))
    else:
        out.append(("none", "No automated case failed in this run."))
    blocked = [t for t in sorted(RESULTS) if RESULTS[t]["status"] == "Blocked"]
    no_exec = [t for t in blocked if RESULTS[t]["actual"] == NO_EXECUTOR_ACTUAL]
    other = [t for t in blocked if t not in no_exec]
    out += [("",), (f"Blocked / not executed ({len(blocked)})",),
            ("Note", "Cases with no automated executor keep the workbook's "
                     "manual-execution placeholder; the rest were skipped "
                     "because a live prerequisite was unavailable."),
            (f"No executor mapped ({len(no_exec)})", ", ".join(no_exec) or "none"),
            (f"Environment prerequisite missing ({len(other)})",
             ", ".join(other) or "none"),
            ("",), ("Honesty statement",),
            ("-", "Every Pass/Fail result above was captured live from the "
                  "running server; nothing is extrapolated."),
            ("-", "Rows marked Blocked were NOT executed and are not marked Pass."),
            ("-", "'Tested By' reads 'Cline (automated agent, HTTP-level)', not a "
                  "human name; human sign-off is still required."),
            ("-", "127.0.0.1:8081 is a local dev instance, not a confirmed "
                  "staging/live environment; re-verify there before sign-off.")]
    row = 4
    for cells in out:
        for i, val in enumerate(cells):
            if val is None:
                continue
            cell = ws.cell(row=row, column=2 + i, value=val)
            cell.alignment = Alignment(wrap_text=True, vertical="top")
        row += 1
    return row

def write_markdown(per_module, total):
    """Emit docs/TEST_EXECUTION_REPORT.md (same numbers as the workbook)."""
    carried = sorted(BASELINE - RUN_IDS)
    lines = [
        "# PWAMS Test Execution Report",
        "",
        f"- **Date**: {TODAY}",
        f"- **Tester / executor**: {TESTER}",
        f"- **Environment**: {ENV_LINE}",
        "- **Harness**: `scripts/qa_run_tests.py` + `scripts/qa_exec_a..d.py`",
        f"- **Cases in matrix**: {sum(total.values())}",
        f"- **Executed**: {total['Pass'] + total['Fail']} "
        f"(**Pass {total['Pass']}**, **Fail {total['Fail']}**)",
        f"- **Blocked / not executed**: {total['Blocked']}",
        "",
        "> Automated HTTP-level execution only. No browser/visual case was "
        "executed and no human tester has signed this off.",
        "",
    ]
    if carried:
        lines += [f"> {len(carried)} case(s) were not re-executed in this session "
                  f"and carry the previous run's result: {', '.join(carried)}.",
                  ""]
    lines += ["## Results per module", "",
              "| Module sheet | Cases | Pass | Fail | Blocked | Pass rate |",
              "|---|---:|---:|---:|---:|---:|"]
    for name in sorted(per_module):
        c = per_module[name]
        lines.append(f"| {name} | {sum(c.values())} | {c['Pass']} | {c['Fail']} "
                     f"| {c['Blocked']} | {_pass_rate(c)} |")
    lines.append(f"| **TOTAL** | **{sum(total.values())}** | **{total['Pass']}** | "
                 f"**{total['Fail']}** | **{total['Blocked']}** | "
                 f"**{_pass_rate(total)}** |")
    fails = [t for t in sorted(RESULTS) if RESULTS[t]["status"] == "Fail"]
    lines += ["", f"## Failures ({len(fails)})", ""]
    if fails:
        lines += ["| Case | Actual result |", "|---|---|"]
        for tid in fails:
            lines.append(f"| {tid} | {_short(RESULTS[tid]['actual'], 400)} |")
    else:
        lines.append("No automated case failed in this run.")
    blocked = [t for t in sorted(RESULTS) if RESULTS[t]["status"] == "Blocked"]
    no_exec = [t for t in blocked if RESULTS[t]["actual"] == NO_EXECUTOR_ACTUAL]
    other = [t for t in blocked if t not in no_exec]
    lines += ["", f"## Blocked / not executed ({len(blocked)})", "",
              f"- **No automated executor mapped ({len(no_exec)})**: "
              + (", ".join(no_exec) if no_exec else "none")
              + " - browser/visual or human-judgement cases.",
              f"- **Environment prerequisite missing ({len(other)})**: "
              + (", ".join(other) if other else "none")]
    for tid in other:
        lines.append(f"  - {tid}: {_short(RESULTS[tid]['actual'], 240)}")
    lines += ["", "## How to reproduce", "",
              "```powershell",
              "# 1. Mailpit SMTP sandbox with AUTH (the app uses smtp.PlainAuth):",
              "Start-Process .\\bin\\mailpit.exe -ArgumentList "
              "'--smtp-auth-file','qa_mailpit_auth.txt','--smtp-auth-allow-insecure'",
              "# 2. Full 232-case run + workbook/markdown export:",
              "python scripts/qa_run_tests.py",
              "#    one chunk only (Auth-Login / Activation / Users / Dashboard):",
              "python scripts/qa_run_tests.py b",
              "#    re-render the deliverables from qa_results.json without tests:",
              "python scripts/qa_run_tests.py export",
              "```",
              "",
              "QA accounts come from `cmd/qaseed` (password `QaPassw0rd!2026`); the "
              "harness clears failed-login counters/lockouts in Postgres before "
              "the run and after every server recycle, so the AUTH-005 lockout "
              "cannot cascade into later cases.",
              "",
              "## Honesty statement",
              "",
              "- Every Pass/Fail above was captured live from the running server; "
              "nothing is extrapolated.",
              "- `Blocked` rows were not executed and are not Pass.",
              "- `Tested By` reads `Cline (automated agent, HTTP-level)`, not a "
              "human name - human sign-off is still required.",
              "- The target is a local dev instance, not a confirmed "
              "staging/live environment."]
    REPORT_MD.parent.mkdir(exist_ok=True)
    REPORT_MD.write_text("\n".join(lines) + "\n", encoding="utf-8")
    print(f"[export] markdown report written: {REPORT_MD.relative_to(REPO)}")
    return REPORT_MD



def export_deliverables():
    """Persist results, then refresh the workbook and the markdown report."""
    save_results()
    per_module, total = write_workbook()
    write_markdown(per_module, total)
    summary = ", ".join(f"{k}={v}" for k, v in sorted(total.items()))
    print(f"[export] TOTAL {sum(total.values())} cases -> {summary}", flush=True)
    return per_module, total


if __name__ == "__main__":
    _args = [a.lower() for a in sys.argv[1:]]
    _parts = ["qa_exec_a.py", "qa_exec_b.py", "qa_exec_c.py", "qa_exec_d.py"]
    if "fresh" in _args:
        print("[harness] --fresh: ignoring any qa_results.json baseline")
    else:
        _merged = load_results()
        if _merged:
            print(f"[harness] merged {_merged} previous result(s) as baseline",
                  flush=True)
    if _args and _args[0] in ("a", "b", "c", "d"):
        _parts = [f"qa_exec_{_args[0]}.py"]
    if _args and _args[0] in ("export", "e"):
        _parts = []
    if _parts:
        atexit.register(save_results)   # keep results if a chunk raises
        unlock_qa_accounts("pre-run")
    for _part in _parts:
        _p = REPO / "scripts" / _part
        if not _p.exists():
            print(f"--- skip {_part} (not present) ---", flush=True)
            continue
        print(f"--- exec {_part} ---", flush=True)
        exec(compile(_p.read_text(encoding="utf-8-sig"), str(_p), "exec"),
             globals())
    print(f"--- recorded {len(RUN_IDS)} result(s) this run "
          f"({len(RESULTS)} total) ---", flush=True)
    export_deliverables()

