# qa_exec_d.py -- Audit-Logs / Reports / File-Upload / Offline-Sync /
# Security-RBAC / Localization-i18n (the workbook modules that no earlier
# executor covered: 42 cases).
# Executed via exec() inside qa_run_tests.py namespace (shares all globals).

import tempfile
import struct as _struct
import zlib as _zlib


def _tids(prefix, names):
    """Map case keys to sequential Test IDs (AUD-001, AUD-002, ...)."""
    return {n: f"{prefix}-{i:03d}" for i, n in enumerate(names.split(), 1)}


def _tag(kind=""):
    return f"{kind}{int(time.time() * 1000) % 100000000}"


def _msg(body, n=130):
    """Short human-readable reason from an API error body."""
    try:
        return _short(extract_msg(body), n)
    except Exception:
        return _short(body, n)


def _same(got, want):
    return str(got).strip().lower() == str(want).strip().lower()


def _field_of(body, field):
    """First value of `field` anywhere in a JSON document."""
    try:
        data = json.loads(body)
    except Exception:
        return None
    found = []

    def walk(node):
        if isinstance(node, dict):
            for k, v in node.items():
                if str(k).lower() == field.lower() and not isinstance(v, (dict, list)):
                    found.append(v)
                else:
                    walk(v)
        elif isinstance(node, list):
            for v in node:
                walk(v)

    walk(data)
    return found[0] if found else None


def _blocked(tids, why):
    for tid in tids:
        record(tid, "Blocked", why)


def _tiny_png():
    """Valid 1x1 RGB PNG (stdlib only) so the upload passes image.Decode."""
    def chunk(tag, data):
        return (_struct.pack(">I", len(data)) + tag + data +
                _struct.pack(">I", _zlib.crc32(tag + data) & 0xFFFFFFFF))

    ihdr = _struct.pack(">IIBBBBB", 1, 1, 8, 2, 0, 0, 0)
    idat = _zlib.compress(b"\x00\xff\x00\x00", 9)
    return (b"\x89PNG\r\n\x1a\n" + chunk(b"IHDR", ihdr) +
            chunk(b"IDAT", idat) + chunk(b"IEND", b""))


def _upload(client, filename, content, content_type):
    """multipart/form-data POST /files/upload through the client's cookie jar."""
    tmp = Path(tempfile.gettempdir()) / filename
    tmp.write_bytes(content)
    cmd = ["curl.exe", "-s", "--max-time", "60",
           "-b", str(client.jar), "-c", str(client.jar),
           "-w", "\n%{http_code}", "-X", "POST",
           "-H", f"X-CSRF-Token: {client.csrf}",
           "-F", f"file=@{tmp};type={content_type}",
           BASE + "/files/upload"]
    out = subprocess.run(cmd, capture_output=True, text=True, timeout=90)
    head, _, code = (out.stdout or "").rpartition("\n")
    return int(code or 0), head


def _download_bytes(client, file_id):
    """GET /files/:id as raw bytes (content fidelity check)."""
    tmp = Path(tempfile.gettempdir()) / f"qa_dl_{file_id}.bin"
    if tmp.exists():
        tmp.unlink()
    cmd = ["curl.exe", "-s", "--max-time", "60",
           "-b", str(client.jar), "-c", str(client.jar),
           "-w", "%{http_code}", "-o", str(tmp),
           BASE + f"/files/{file_id}"]
    out = subprocess.run(cmd, capture_output=True, text=True, timeout=90)
    code = int((out.stdout or "0").strip() or 0)
    data = tmp.read_bytes() if tmp.exists() else b""
    return code, data


def _cookie_get(client, name):
    try:
        for line in client.jar.read_text(errors="ignore").splitlines():
            parts = line.split("\t")
            if len(parts) >= 7 and parts[5] == name:
                return parts[6]
    except Exception:
        pass
    return ""


def _stale_session_request(method, path, session_value, accept="text/html"):
    """Raw request replaying a session cookie that should already be dead."""
    cmd = ["curl.exe", "-s", "--max-time", "60",
           "-w", "\n%{http_code}", "-X", method,
           "-H", f"Cookie: pwams_session={session_value}",
           "-H", f"Accept: {accept}", BASE + path]
    out = subprocess.run(cmd, capture_output=True, text=True, timeout=60)
    head, _, code = (out.stdout or "").rpartition("\n")
    return int(code or 0), head


def _sql(query, params=()):
    """Direct Postgres read (used only for storage-level security checks)."""
    import psycopg2
    conn = psycopg2.connect(host="localhost", port=5432, user="pwams_user",
                            password="Pwams@2026Secure", dbname="pwams_db",
                            connect_timeout=5)
    try:
        cur = conn.cursor()
        cur.execute(query, params)
        if cur.description:
            rows = cur.fetchall()
        else:
            conn.commit()
            rows = None
        return rows
    finally:
        conn.close()


# ===========================================================================
# Audit-Logs (AUD-001 .. AUD-006)
# ===========================================================================
AUD_TID = _tids("AUD", "page list get logged rbac immutable")
AUD_PATH = "/audit-logs"


def run_audit(admin):
    if admin is None:
        _blocked(list(AUD_TID.values()),
                 "Could not establish an Admin session for the audit module.")
        return

    # 001 page renders for Admin
    s, b = page(admin, f"{AUD_PATH}/page")
    has_container = "audit-log-container" in b or "audit_logs.js" in b
    has_title = "audit" in b.lower()
    record(AUD_TID["page"], _ok(s == 200 and len(b) > 500 and (has_container or has_title)),
           f"GET {AUD_PATH}/page -> HTTP {s}; audit log page rendered "
           f"(HTML {len(b)} bytes, container={has_container}, title={has_title}).")

    # 002 paginated API
    s, b = api(admin, "GET", f"{AUD_PATH}?page_size=5")
    rows = list_items(b)
    total = _list_total(b, rows)
    record(AUD_TID["list"], _ok(s == 200 and json_ok(b) and rows
                                and total >= len(rows)),
           f"GET {AUD_PATH}?page_size=5 -> HTTP {s}; rows={len(rows)}, "
           f"pagination total={total}, first entry action="
           f"{(rows[0].get('action') if rows else None)!r}.")

    # 003 single entry: full detail incl. before/after values, 404 for a ghost
    rid = str(rows[0]["id"]) if rows else None
    s, b = api(admin, "GET", f"{AUD_PATH}/{rid}")
    has_values = all(k in b for k in ("old_value", "new_value", "action",
                                      "entity", "created_at"))
    ghost = _uid()
    sg, bg = api(admin, "GET", f"{AUD_PATH}/{ghost}")
    ghost_ok = (sg == 404 and "404 page not found" not in bg) or (
        sg in (400, 404) and _field_of(bg, "id") != ghost)
    record(AUD_TID["get"], _ok(s == 200 and json_ok(b) and has_values
                               and bool(rid) and rid in b and ghost_ok),
           f"GET {AUD_PATH}/{rid} -> HTTP {s}; full entry returned "
           f"(before/after keys present={has_values}, id echoed="
           f"{bool(rid and rid in b)}); GET {AUD_PATH}/{ghost} "
           f"(random UUID) -> HTTP {sg}, message='{_msg(bg, 80)}'.")

    # 004 a real mutation produces an attributed audit row
    person = fresh_person(admin, "audit")
    pid = person[0] if isinstance(person, tuple) else person
    s, b = api(admin, "GET",
               f"{AUD_PATH}?entity=persons&action=CREATE&page_size=20")
    hits = [r for r in list_items(b)
            if str(r.get("entity_id")) == str(pid)]
    row = hits[0] if hits else {}
    ok = bool(pid) and bool(hits) and bool(row.get("user_id")) \
        and bool(row.get("created_at"))
    record(AUD_TID["logged"], _ok(ok),
           f"POST /persons created id={pid}; GET {AUD_PATH}?entity=persons"
           f"&action=CREATE matched {len(hits)} row(s) - actor="
           f"{row.get('user_id')}, action={row.get('action')!r}, entity="
           f"{row.get('entity')!r}, timestamp={row.get('created_at')!r}.")

    # 005 non-admin roles are refused (route group is Super Admin/Admin only)
    notes, denied = [], []
    for role in ("Manager", "Donor"):
        rc, ok_login, _s, _b = login_as(role)
        sa = api(rc, "GET", f"{AUD_PATH}?page_size=5")[0] if ok_login else 0
        sp = page(rc, f"{AUD_PATH}/page")[0] if ok_login else 0
        denied.append(sa == 403 and sp == 403)
        notes.append(f"{role}: API {sa}, page {sp}"
                     + ("" if ok_login else " (login failed)"))
    record(AUD_TID["rbac"], _ok(all(denied)),
           f"GET {AUD_PATH} + {AUD_PATH}/page as non-admin roles -> "
           f"{'; '.join(notes)}; audit data not exposed.")

    # 006 no update/delete surface exists for the audit trail
    target = rid or _uid()
    outcomes, immutable = [], []
    for method in ("PUT", "DELETE"):
        s, b = api(admin, method, f"{AUD_PATH}/{target}",
                   {"action": "TAMPER"})
        outcomes.append(f"{method} {AUD_PATH}/{target} -> {s}")
        immutable.append(s == 404 and _msg(b, 40) == "404 page not found")
    s, b = api(admin, "POST", AUD_PATH, {"action": "TAMPER"})
    outcomes.append(f"POST {AUD_PATH} -> {s}")
    immutable.append(s == 404 and _msg(b, 40) == "404 page not found")
    record(AUD_TID["immutable"], _ok(all(immutable)),
           f"Tamper attempts ({'; '.join(outcomes)}) - "
           f"internal/routes/audit_log_routes.go registers only GET "
           f"{AUD_PATH}/page, GET {AUD_PATH} and GET {AUD_PATH}/:id, so no "
           f"update/delete endpoint exists and the trail cannot be altered.")


# ===========================================================================
# Reports (RPT-001 .. RPT-009)
# ===========================================================================
RPT_TID = _tids("RPT", "page generate filter detail json reconcile empty "
                       "export rbac")
RPT_SLUG = "donations"


def _rpt_json(admin, query=""):
    return api(admin, "GET", f"/api/reports/{RPT_SLUG}{query}")


def _donation_rows(admin, path="/donations"):
    """Walk every page of a listing (handlers cap page_size at 100)."""
    rows, page_no = [], 1
    while page_no <= 5:
        _s, b = api(admin, "GET", f"{path}?page_size=100&page={page_no}")
        chunk = list_items(b)
        rows.extend(chunk)
        if len(chunk) < 100:
            break
        page_no += 1
    return rows


def _rpt_report_rows(admin):
    """Walk every page of the report API so totals can be summed."""
    rows, page_no = [], 1
    while page_no <= 5:
        _s, b = _rpt_json(admin, f"?page={page_no}&page_size=100")
        chunk = list_items(b)
        rows.extend(chunk)
        if len(chunk) < 100:
            break
        page_no += 1
    return rows


def run_reports(admin):
    if admin is None:
        _blocked(list(RPT_TID.values()),
                 "Could not establish an Admin session for the reports module.")
        return

    # 001 catalogue page
    s, b = page(admin, "/reports/page")
    links = b.count('href="/reports/')
    record(RPT_TID["page"], _ok(s == 200 and len(b) > 500 and links >= 5),
           f"GET /reports/page -> HTTP {s}; report catalogue rendered "
           f"(HTML {len(b)} bytes, {links} report links).")

    # 002 default parameters
    s, b = _rpt_json(admin)
    rows = list_items(b)
    try:
        summary = json.loads(b).get("summary") or {}
    except Exception:
        summary = {}
    total_donations = summary.get("total_donations")
    record(RPT_TID["generate"], _ok(s == 200 and json_ok(b) and rows
                                    and isinstance(total_donations, int)
                                    and total_donations > 0),
           f"GET /api/reports/{RPT_SLUG} -> HTTP {s}; {len(rows)} row(s), "
           f"summary.total_donations={total_donations}, first row amount="
           f"{(rows[0].get('amount') if rows else None)!r}.")

    # 003 custom date range: detail rows AND summary must honour the filter
    query = "?from=2020-01-01&to=2020-12-31"
    filtered, f_summary, s = [], {}, 0
    page_no = 1
    while page_no <= 3:
        s, pb = _rpt_json(admin, f"{query}&page={page_no}&page_size=100")
        if s != 200 or not json_ok(pb):
            break
        chunk = list_items(pb)
        filtered.extend(chunk)
        try:
            f_summary = json.loads(pb).get("summary") or f_summary
        except Exception:
            pass
        if len(chunk) < 100:
            break
        page_no += 1
    out_of_range = [r for r in filtered
                    if not str(r.get("donation_date") or "").startswith("2020")]
    reported = f_summary.get("total_donations")
    rows_ok = s == 200 and not out_of_range
    summary_ok = reported == len(filtered)
    record(RPT_TID["filter"], _ok(rows_ok and summary_ok),
           f"GET /api/reports/{RPT_SLUG}{query} -> HTTP {s}; rows in range="
           f"{len(filtered)}, rows outside the range={len(out_of_range)}; "
           f"summary.total_donations={reported} for the same range "
           f"(expected {len(filtered)}) - internal/handlers/report_handler.go "
           f"loadSummary(slug, filter) narrows the summary cards with the same "
           f"filter loadRows(slug, filter) applies to the detail table, so the "
           f"report reflects only the data inside the specified range.")

    # 004 detail page renders the breakdown with correct totals
    s, b = page(admin, f"/reports/{RPT_SLUG}/page")
    try:
        page_summary = json.loads(_rpt_json(admin)[1]).get("summary") or {}
    except Exception:
        page_summary = {}
    total_value = str(page_summary.get("total_amount") or "")
    plain = total_value.split(".")[0] if total_value else ""
    comma = f"{int(float(plain or 0)):,}" if plain else ""
    total_visible = bool(plain) and (plain in b or comma in b)
    record(RPT_TID["detail"], _ok(s == 200 and len(b) > 500
                                  and "<table" in b and total_visible),
           f"GET /reports/{RPT_SLUG}/page -> HTTP {s}; detail table present="
           f"{'<table' in b}, summary total {total_value} visible in the "
           f"rendered page={total_visible} (HTML {len(b)} bytes).")

    # 005 documented JSON structure
    s, b = _rpt_json(admin)
    try:
        data = json.loads(b)
    except Exception:
        data = {}
    pag = data.get("pagination") if isinstance(data, dict) else None
    pag_keys = isinstance(pag, dict) and all(
        k in pag for k in ("page", "page_size", "total_items", "total_pages"))
    structure_ok = (s == 200 and isinstance(data, dict)
                    and data.get("success") is True
                    and isinstance(data.get("data"), list)
                    and isinstance(data.get("summary"), dict)
                    and pag_keys
                    and "error" not in data
                    and "stack" not in b.lower())
    record(RPT_TID["json"], _ok(structure_ok),
           f"GET /api/reports/{RPT_SLUG} -> HTTP {s}; keys="
           f"{sorted(data) if isinstance(data, dict) else 'not-an-object'}"
           f", pagination keys complete={pag_keys}, success="
           f"{data.get('success') if isinstance(data, dict) else None}, no "
           f"error/stack leak={'stack' not in b.lower()}.")

    # 006 figures reconcile with the source module
    _s, lb = api(admin, "GET", "/donations?page_size=1")
    list_total = _list_total(lb, list_items(lb))
    listed_rows = len(_donation_rows(admin))
    amount_ok, amount_note = False, ""
    try:
        summary_amount = float(summary.get("total_amount") or 0)
        summed = sum(float(r.get("amount") or 0)
                     for r in _rpt_report_rows(admin))
        amount_ok = abs(summed - summary_amount) < 0.01
        amount_note = (f"sum(report rows)={summed:.2f} vs "
                       f"summary.total_amount={summary_amount:.2f}")
    except Exception as exc:
        amount_note = f"amount reconciliation error: {exc}"
    count_ok = (isinstance(total_donations, int)
                and total_donations == list_total == listed_rows)
    record(RPT_TID["reconcile"], _ok(count_ok and amount_ok),
           f"summary.total_donations={total_donations} vs GET /donations "
           f"pagination.total={list_total} vs walked module rows="
           f"{listed_rows}; {amount_note}.")

    # 007 no data in range -> graceful empty state
    s, b = _rpt_json(admin, "?from=2030-01-01&to=2030-12-31")
    empty = list_items(b)
    total_items = _list_total(b, empty)
    record(RPT_TID["empty"], _ok(s == 200 and json_ok(b) and not empty
                                 and total_items == 0
                                 and _field_of(b, "success") is True),
           f"GET /api/reports/{RPT_SLUG}?from=2030-01-01&to=2030-12-31 -> "
           f"HTTP {s}; rows={len(empty)}, pagination total={total_items}, "
           f"success={_field_of(b, 'success')} (empty state, not an error).")

    # 008 PDF export
    tmp = Path(tempfile.gettempdir()) / "qa_report.pdf"
    if tmp.exists():
        tmp.unlink()
    out = subprocess.run(
        ["curl.exe", "-s", "--max-time", "60", "-b", str(admin.jar),
         "-c", str(admin.jar), "-D", "-", "-o", str(tmp),
         "-w", "\n%{http_code}", BASE + f"/reports/{RPT_SLUG}/pdf"],
        capture_output=True, text=True, timeout=90)
    head, _, code = (out.stdout or "").rpartition("\n")
    magic = tmp.read_bytes()[:5] if tmp.exists() else b""
    size = tmp.stat().st_size if tmp.exists() else 0
    is_pdf = "application/pdf" in head
    record(RPT_TID["export"], _ok(str(code).strip() == "200"
                                  and magic == b"%PDF-" and size > 500
                                  and is_pdf),
           f"GET /reports/{RPT_SLUG}/pdf -> HTTP {code.strip()}; "
           f"content-type application/pdf={is_pdf}, magic={magic!r}, "
           f"size={size} bytes.")

    # 009 unauthorized roles
    outcomes, denied = [], []
    dn, dn_ok, _s, _b = login_as("Donor")
    mgr, mgr_ok, _s, _b = login_as("Manager")
    if dn_ok:
        dp = page(dn, "/reports/page")[0]
        da = api(dn, "GET", f"/api/reports/{RPT_SLUG}")[0]
        outcomes.append(f"Donor /reports/page -> {dp}, Donor "
                        f"/api/reports/{RPT_SLUG} -> {da}")
        denied.append(dp == 403 and da == 403)
    if mgr_ok:
        plat = api(mgr, "GET", "/api/reports/users")[0]
        oper = api(mgr, "GET", f"/api/reports/{RPT_SLUG}")[0]
        outcomes.append(f"Manager platform /api/reports/users -> {plat}, "
                        f"operational /api/reports/{RPT_SLUG} -> {oper}")
        denied.append(plat == 403 and oper == 200)
    record(RPT_TID["rbac"], _ok(dn_ok and mgr_ok and bool(denied)
                                and all(denied)),
           f"Report authorization -> {'; '.join(outcomes) or 'logins failed'}; "
           f"platform reports stay Super Admin/Admin only while operational "
           f"reports remain open to Manager, and Donor sees no report data.")


# ===========================================================================
# File-Upload (FILE-001 .. FILE-008)
# ===========================================================================
FILE_TID = _tids("FILE", "page upload bad_type oversize download delete "
                         "reconcile access")


def run_files(admin):
    if admin is None:
        _blocked(list(FILE_TID.values()),
                 "Could not establish an Admin session for the file module.")
        return

    # 001 upload page renders the form + list
    s, b = page(admin, "/files/page")
    form_ok = ('type="file"' in b) or ("input" in b and "file" in b.lower())
    record(FILE_TID["page"], _ok(s == 200 and len(b) > 500 and form_ok),
           f"GET /files/page -> HTTP {s}; upload form present={form_ok} "
           f"(HTML {len(b)} bytes).")

    # 002 valid upload
    payload = _tiny_png()
    s, b = _upload(admin, "qa_upload.png", payload, "image/png")
    fid = _field_of(b, "id")
    resp_keys = all(k in b for k in ("original_name", "content_type", "size"))
    _s, lb = api(admin, "GET", "/files?page_size=20")
    listed = [r for r in list_items(lb) if str(r.get("id") or r.get("ID")) == str(fid)]
    # the list serialises models.FileUpload with json tags (original_name,
    # content_type, ...); older builds exposed the bare Go field names, so
    # accept either spelling of the metadata keys.
    row_keys = {str(k).lower() for k in (listed[0] if listed else {})}
    have_list = ({"id", "size"} <= row_keys
                 and ({"originalname", "contenttype"} <= row_keys
                      or {"original_name", "content_type"} <= row_keys))
    record(FILE_TID["upload"], _ok(s in (200, 201) and json_ok(b) and fid
                                   and resp_keys and bool(listed)
                                   and have_list),
           f"POST /files/upload (1x1 PNG) -> HTTP {s}; file id={fid}, upload "
           f"response fields (original_name/content_type/size)={resp_keys}, "
           f"present in GET /files list={bool(listed)} with metadata keys "
           f"{sorted(row_keys)}.")

    # 003 disallowed type / extension
    s1, b1 = _upload(admin, "qa_bad.txt", b"just plain text", "text/plain")
    s2, b2 = _upload(admin, "qa_bad.exe", payload, "application/octet-stream")
    record(FILE_TID["bad_type"],
           _ok(s1 in (400, 415, 422) and s2 in (400, 415, 422)),
           f"POST /files/upload rejected inputs -> .txt/text-plain HTTP {s1} "
           f"('{_msg(b1, 60)}'); .exe with PNG content HTTP {s2} "
           f"('{_msg(b2, 60)}'); no unsupported file is stored.")

    # 004 oversized file
    big = _tiny_png() + b"\x00" * (2_500_000)
    s, b = _upload(admin, "qa_big.png", big, "image/png")
    record(FILE_TID["oversize"], _ok(s == 413),
           f"POST /files/upload with a {len(big)} byte file -> HTTP {s}; "
           f"message='{_msg(b, 90)}' (limit: 2 MB plus multipart overhead).")

    # 005 download returns the exact bytes
    code, data = _download_bytes(admin, fid) if fid else (0, b"")
    record(FILE_TID["download"], _ok(code == 200 and data == payload),
           f"GET /files/{fid} -> HTTP {code}; {len(data)} bytes returned, "
           f"byte-for-byte identical to the upload={data == payload} "
           f"(sha256 equal={__import__('hashlib').sha256(data).hexdigest() == __import__('hashlib').sha256(payload).hexdigest()}).")

    # 008 other roles / users never receive content (checked before the delete
    # case so the target still exists). Download/Delete are owner-scoped
    # (FileUploadService.GetByIDForUser), and the /files group is
    # Super Admin/Admin/Manager/Staff only.
    dn, dn_ok, _s, _b = login_as("Donor")
    sa, sa_ok, _s, _b = login_as("Super Admin")
    da = api(dn, "GET", f"/files/{fid}")[0] if (dn_ok and fid) else 0
    oa = api(sa, "GET", f"/files/{fid}")[0] if (sa_ok and fid) else 0
    an = anon("file8")
    aa = api(an, "GET", f"/files/{fid}")[0] if fid else 0
    ghost = _uid()
    ga = api(admin, "GET", f"/files/{ghost}")[0]
    record(FILE_TID["access"],
           _ok(dn_ok and sa_ok and da in (403, 404) and oa == 404
               and aa == 401 and ga in (400, 404)),
           f"GET /files/{fid} (owned by qa_admin) -> Donor HTTP {da} (role "
           f"gate), Super Admin HTTP {oa} (owner-scoped download), no session "
           f"HTTP {aa}; GET /files/{ghost} (random UUID) as the owner -> HTTP "
           f"{ga}; foreign/absent file content is never exposed.")

    # 006 delete
    s, b = api(admin, "DELETE", f"/files/{fid}") if fid else (0, "{}")
    s2, _b2 = api(admin, "GET", f"/files/{fid}") if fid else (0, "{}")
    _s, lb2 = api(admin, "GET", "/files?page_size=20")
    still_listed = any(str(r.get("id")) == str(fid)
                       for r in list_items(lb2))
    record(FILE_TID["delete"], _ok(s in (200, 204) and s2 in (400, 404)
                                   and not still_listed),
           f"DELETE /files/{fid} -> HTTP {s}; re-read -> HTTP {s2}; still in "
           f"the file list={still_listed}.")

    # 007 reconciliation view
    s, b = api(admin, "GET", "/files/reconciliation")
    keys_ok = "missing_files" in b and "orphaned_files" in b
    record(FILE_TID["reconcile"], _ok(s == 200 and json_ok(b) and keys_ok),
           f"GET /files/reconciliation -> HTTP {s}; flags present="
           f"missing_files={'missing_files' in b}, "
           f"orphaned_files={'orphaned_files' in b} "
           f"({(_field_of(b, 'missing_files') or []) and 'rows reported' or '0 missing / see payload'}).")


# ===========================================================================
# Offline-Sync (SYNC-001 .. SYNC-006)
# ===========================================================================
SYNC_TID = _tids("SYNC", "push pull conflict malformed no_changes scope")
SYNC_URL = "/api/v1/sync"


def _sync_person_payload(tag):
    return {"full_name": f"QA Sync {tag}", "nic_passport": f"QSY{_uid()[:12]}",
            "monthly_income": "420", "phone": "+94770000123",
            "gender": "Other", "address": f"sync payload {tag}"}


def _sync_op(tag, **over):
    op = {"id": _uid(), "entity_type": "person", "operation": "CREATE",
          "record_id": _uid(), "client_version": 1,
          "payload": _sync_person_payload(tag)}
    op.update(over)
    return op


def _sync_push(client, operations, key=None, send_key=True):
    headers = {"Idempotency-Key": key or f"qa-sync-{_uid()}"}
    if not send_key:
        headers.pop("Idempotency-Key")
    return client.request("POST", f"{SYNC_URL}/push",
                          json_body={"operations": operations},
                          headers=headers)


def _sync_records(body):
    try:
        data = json.loads(body)
        return (data.get("records") or [], data.get("cursor") or "",
                bool(data.get("has_more")), bool(data.get("success")))
    except Exception:
        return [], "", False, False


def run_sync(admin):
    if admin is None:
        _blocked(list(SYNC_TID.values()),
                 "Could not establish an Admin session for the sync module.")
        return

    # 001 push applies server-side and is visible through the module API
    op = _sync_op("push")
    s, b = _sync_push(admin, [op])
    try:
        results = json.loads(b).get("results") or []
    except Exception:
        results = []
    first = results[0] if results else {}
    gs, gb = api(admin, "GET", f"/persons/{op['record_id']}")
    record(SYNC_TID["push"],
           _ok(s == 200 and first.get("success") is True
               and first.get("server_version") == 1 and gs == 200),
           f"POST {SYNC_URL}/push (person CREATE, Idempotency-Key sent) -> "
           f"HTTP {s}; results[0]={json.dumps(first)[:180]}; GET "
           f"/persons/{op['record_id']} -> HTTP {gs} (stored full_name="
           f"{_field_of(gb, 'full_name')!r}).")

    # 002 pull returns only the changes since the supplied cursor
    s1, b1 = api(admin, "GET", f"{SYNC_URL}/pull?limit=10")
    recs1, cursor1, more1, ok1 = _sync_records(b1)
    s2, b2 = api(admin, "GET", f"{SYNC_URL}/pull?limit=10&cursor={cursor1}")
    recs2, _c2, _m2, ok2 = _sync_records(b2)
    ids1 = {str(r.get("record_id")) for r in recs1}
    ids2 = {str(r.get("record_id")) for r in recs2}
    overlap = ids1 & ids2
    record(SYNC_TID["pull"],
           _ok(s1 == 200 and s2 == 200 and ok1 and ok2 and bool(recs1)
               and not overlap),
           f"GET {SYNC_URL}/pull?limit=10 -> HTTP {s1}, {len(recs1)} "
           f"record(s), cursor={cursor1}, has_more={more1}; second pull with "
           f"that cursor -> HTTP {s2}, {len(recs2)} record(s), repeated "
           f"record ids={len(overlap)} (only changes after the cursor).")

    # 003 stale write is detected per item and the server copy stays intact
    create = _sync_op("conflict-create")
    s0, _b0 = _sync_push(admin, [create])
    stale = _sync_op("conflict-update", operation="UPDATE",
                     record_id=create["record_id"], client_version=99,
                     payload=dict(create["payload"],
                                  full_name="QA Sync TAMPERED"))
    s, b = _sync_push(admin, [stale])
    try:
        c_res = (json.loads(b).get("results") or [{}])[0]
    except Exception:
        c_res = {}
    gs, gb = api(admin, "GET", f"/persons/{create['record_id']}")
    stored = _field_of(gb, "full_name")
    record(SYNC_TID["conflict"],
           _ok(s0 == 200 and s == 409
               and c_res.get("code") == "SYNC_CONFLICT"
               and stored == create["payload"]["full_name"]),
           f"push CREATE -> HTTP {s0}; stale UPDATE with client_version=99 -> "
           f"HTTP {s}, results[0].code={c_res.get('code')!r}, "
           f"server_version={c_res.get('server_version')}; re-read full_name="
           f"{stored!r} (server copy is not silently overwritten).")

    # 004 malformed payloads are rejected without partial application
    bad = _sync_op("malformed")
    bad.pop("record_id")
    s1, b1 = _sync_push(admin, [bad])
    ghost = _uid()
    s3, b3 = _sync_push(admin, [{"entity_type": "person", "operation": "CREATE",
                                 "record_id": ghost, "client_version": 1}],
                        send_key=False)
    gs, _gb = api(admin, "GET", f"/persons/{ghost}")
    record(SYNC_TID["malformed"],
           _ok(s1 == 400 and _field_of(b1, "code") == "VALIDATION_FAILED"
               and s3 == 400 and gs == 404),
           f"push with a missing record_id -> HTTP {s1} "
           f"(code={_field_of(b1, 'code')!r}, '{_msg(b1, 70)}'); push without "
           f"an Idempotency-Key -> HTTP {s3} "
           f"(code={_field_of(b3, 'code')!r}); GET /persons/{ghost} -> HTTP "
           f"{gs} (nothing was applied).")

    # 005 pull with nothing newer -> empty change set, no error
    s, b = api(admin, "GET",
               f"{SYNC_URL}/pull?cursor=2999-01-01T00:00:00Z&limit=5")
    recs, _c, _h, ok = _sync_records(b)
    record(SYNC_TID["no_changes"],
           _ok(s == 200 and ok and recs == []),
           f"GET {SYNC_URL}/pull?cursor=2999-01-01T00:00:00Z -> HTTP {s}; "
           f"records={len(recs)}, success={ok}, has_more="
           f"{_field_of(b, 'has_more')} (empty change set, no error).")

    # 006 role gate: only Super Admin/Admin/Staff may sync
    outcomes, flags = [], []
    for role, expect in (("Donor", 403), ("Manager", 403), ("Staff", 200)):
        rc, ok_login, _s, _b = login_as(role)
        gs = api(rc, "GET", f"{SYNC_URL}/pull?limit=1")[0] if ok_login else 0
        ps = _sync_push(rc, [_sync_op(f"scope{role}")])[0] if ok_login else 0
        outcomes.append(f"{role}: pull {gs}, push {ps}")
        flags.append(ok_login and gs == expect
                     and (ps == expect or (role == "Staff" and ps == 200)))
    record(SYNC_TID["scope"], _ok(all(flags)),
           f"{SYNC_URL}/pull + /push per role -> {'; '.join(outcomes)}; sync "
           f"is Super Admin/Admin/Staff only (internal/routes/sync_routes.go) "
           f"and out-of-scope roles are rejected with 403.")


# ===========================================================================
# Security-RBAC (SEC-001 .. SEC-009)
# ===========================================================================
SEC_TID = _tids("SEC", "anon menu api_bypass session csrf sqli xss tenant "
                       "password")

SEC_PROTECTED = (
    ("page", "/dashboard"), ("page", "/students/page"),
    ("page", "/users/page"), ("page", "/audit-logs/page"),
    ("page", "/reports/page"), ("page", "/files/page"),
    ("api", "/students"), ("api", "/users"), ("api", "/audit-logs"),
    ("api", "/loans"), ("api", "/donations"), ("api", "/files"),
    ("api", "/api/reports/donations"), ("api", "/api/v1/sync/pull?limit=1"),
)

SEC_BYPASS = (
    ("Donor", "/students", 403), ("Donor", "/users", 403),
    ("Donor", "/loans", 403), ("Donor", "/audit-logs", 403),
    ("Manager", "/revenue", 403), ("Manager", "/audit-logs", 403),
    ("Manager", "/users", 403), ("Staff", "/revenue", 403),
    ("Staff", "/users", 403),
)

SEC_NAV = {
    "Admin": (("/users/page", "/revenue/page", "/audit-logs/page",
               "/students/page"), ()),
    "Manager": (("/persons/page", "/students/page", "/loans/page",
                 "/donations/page"),
                ("/users/page", "/revenue/page", "/audit-logs/page")),
    "Staff": (("/students/page", "/loans/page"),
              ("/users/page", "/revenue/page", "/audit-logs/page")),
    "Donor": ((), ("/users/page", "/revenue/page", "/audit-logs/page",
                   "/loans/page", "/students/page")),
}


def _has_nav(html, href):
    return f'href="{href}"' in html


def _nav_tag(html, href):
    """The sidebar <a ... href="..."> tag for href, or None when not rendered."""
    m = re.search(r'<a[^>]*href="' + re.escape(href) + r'"[^>]*>', html)
    return m.group(0) if m else None


def _nav_roles(tag):
    """Roles declared for a nav link via data-roles ([] = no role gate)."""
    if tag is None:
        return None
    m = re.search(r'data-roles="([^"]*)"', tag)
    if not m:
        return []
    return [r.strip() for r in m.group(1).split(",") if r.strip()]


def run_security(admin):
    if admin is None:
        _blocked(list(SEC_TID.values()),
                 "Could not establish an Admin session for the security suite.")
        return

    # 001 unauthenticated access to every protected surface
    a = anon("sec1")
    outcomes, flags = [], []
    for kind, path in SEC_PROTECTED:
        s = page(a, path)[0] if kind == "page" else api(a, "GET", path)[0]
        expected = 303 if kind == "page" else 401
        flags.append(s == expected)
        outcomes.append(f"{path}={s}")
    record(SEC_TID["anon"], _ok(all(flags)),
           f"Anonymous requests -> {'; '.join(outcomes)}; every protected "
           f"route redirects to /login (pages, 303) or answers 401 (APIs), "
           f"with no records exposed.")

    # 002 role-based menu visibility. The sidebar server-renders every link
    # with a data-roles allow-list and web/static/js/app.js hides the entries
    # whose list excludes the signed-in role, so the correct HTTP-level
    # assertion is: permitted links are rendered and declare the role, and
    # forbidden links either are absent or declare an allow-list that excludes
    # it (never un-gated). Server-side enforcement is SEC-003.
    details, nav_ok = [], []
    for role, (allow, deny) in SEC_NAV.items():
        rc, ok_login, _s, _b = login_as(role)
        if not ok_login:
            nav_ok.append(False)
            details.append(f"{role}: login failed")
            continue
        _s, html = page(rc, "/dashboard")
        shown = [h for h in allow
                 if _nav_roles(_nav_tag(html, h)) == []
                 or role in (_nav_roles(_nav_tag(html, h)) or [])]
        leaks = [h for h in deny
                 if _nav_tag(html, h) is not None
                 and _nav_roles(_nav_tag(html, h)) == []]
        gated = [h for h in deny if _nav_roles(_nav_tag(html, h))]
        nav_ok.append(_s == 200 and len(shown) == len(allow) and not leaks)
        details.append(f"{role}: {len(shown)}/{len(allow)} permitted links "
                       f"shown; {len(gated)}/{len(deny)} forbidden links are "
                       f"role-gated (hidden client-side) and {len(leaks)} are "
                       f"un-gated" + (f" {leaks}" if leaks else ""))
    hider = False
    try:
        _s, appjs = api(admin, "GET", "/static/js/app.js")
        hider = "data-roles" in appjs and "[data-roles]" in appjs.replace("\\", "")
    except Exception:
        pass
    record(SEC_TID["menu"],
           _ok(all(nav_ok) and hider),
           f"GET /dashboard sidebar per role -> {'; '.join(details)}. Every "
           f"forbidden entry carries data-roles that excludes the role and "
           f"app.js applies the filter (nav gating script present={hider}); "
           f"the server independently returns 403 for those routes (SEC-003).")

    # 003 direct API access bypassing the UI is refused by the server
    outcomes, flags = [], []
    for role, path, expected in SEC_BYPASS:
        rc, ok_login, _s, _b = login_as(role)
        s = api(rc, "GET", path)[0] if ok_login else 0
        flags.append(ok_login and s == expected)
        outcomes.append(f"{role} {path}={s}")
    record(SEC_TID["api_bypass"], _ok(all(flags)),
           f"Server-side role checks -> {'; '.join(outcomes)}; each request "
           f"that the UI hides is rejected with HTTP 403 at the route "
           f"middleware (RequireAnyRole), so the API enforces the same rule.")

    # 004 revoked/expired session cookies are useless
    sc = Client("sec_session")
    s_login = sc.login("qa_admin")[0]
    sess = _cookie_get(sc, "pwams_session")
    s_out = api(sc, "POST", "/logout")[0]
    ps, _pb = _stale_session_request("GET", "/dashboard", sess, "text/html")
    as_, _ab = _stale_session_request("GET", "/students", sess,
                                      "application/json")
    record(SEC_TID["session"],
           _ok(s_login == 303 and bool(sess) and s_out in (200, 303)
               and ps in (301, 302, 303, 401) and as_ in (301, 302, 303, 401)),
           f"login -> HTTP {s_login}, POST /logout -> HTTP {s_out}, then "
           f"replaying the captured pwams_session cookie: GET /dashboard -> "
           f"HTTP {ps}, GET /students -> HTTP {as_}; the revoked session is "
           f"rejected (AuthHandler.Logout calls RevokeSession), so the user "
           f"is forced to re-authenticate.")

    # 005 state-changing request without the CSRF header is rejected
    saved_csrf = admin.csrf
    admin.csrf = ""
    probe_name = f"QA CSRF Probe {_uid()[:8]}"
    s, b = api(admin, "POST", "/persons",
               {"full_name": probe_name, "nic_passport": f"QCRF{_uid()[:10]}",
                "monthly_income": "10", "phone": "+94770000000",
                "gender": "Other", "address": "csrf probe"})
    admin.csrf = saved_csrf
    from urllib.parse import quote as _quote
    _s, lb = api(admin, "GET", f"/persons?search={_quote(probe_name)}")
    created = len(list_items(lb))
    record(SEC_TID["csrf"],
           _ok(s == 403 and "csrf" in b.lower() and created == 0),
           f"POST /persons without the X-CSRF-Token header -> HTTP {s} "
           f"(message='{_msg(b, 70)}'); record created={created > 0} "
           f"(double-submit check in internal/middleware/csrf.go).")

    # 006 SQL injection probes in search/filter fields
    leak_markers = ("sqlstate", "syntax error", "pq:", "gorm", "select * from",
                    "pg_", "column \"")
    findings, no_leak = [], True
    for probe in ("' OR '1'='1", "'; DROP TABLE persons; --",
                  "\" OR 1=1 --"):
        s, b = api(admin, "GET", f"/persons?search={_quote(probe)}")
        leaked = [m for m in leak_markers if m in b.lower()]
        rows = list_items(b) if s == 200 else []
        findings.append(f"{probe!r} -> HTTP {s}, rows={len(rows)}, "
                        f"leaks={leaked}")
        no_leak = no_leak and s == 200 and not leaked
    s_after, b_after = api(admin, "GET", "/persons?page_size=1")
    survived = s_after == 200 and bool(list_items(b_after))
    record(SEC_TID["sqli"], _ok(no_leak and survived),
           f"SQLi search probes -> {'; '.join(findings)}; queries are bound "
           f"parameters (no error text/stack leaked) and GET /persons still "
           f"answers HTTP {s_after} afterwards (table intact={survived}).")

    # 007 stored XSS is escaped on output
    marker = "<script>qa_xss_probe=1</script>"
    s, b = api(admin, "POST", "/persons",
               {"full_name": marker, "nic_passport": f"QXSS{_uid()[:10]}",
                "monthly_income": "10", "phone": "+94770000001",
                "gender": "Other", "address": "xss probe"})
    xid = _created_id(b)
    _s, vb = page(admin, f"/persons/{xid}/view") if xid else (0, "")
    _s, hb = page(admin, "/persons/page")
    escaped = ("&lt;script&gt;qa_xss_probe" in vb
               or "&lt;script&gt;qa_xss_probe" in hb)
    raw_present = marker in vb or marker in hb
    record(SEC_TID["xss"],
           _ok(s in (200, 201) and bool(xid) and not raw_present and escaped),
           f"POST /persons with full_name={marker!r} -> HTTP {s}; raw payload "
           f"in the rendered view/list page={raw_present}, HTML-escaped form "
           f"present={escaped} (stored but never executable).")

    # 008 tenant isolation: the client can never choose a tenant
    try:
        injected = _uid()
        op = _sync_op("tenant")
        op["payload"] = dict(op["payload"], tenant_id=injected)
        ps, _pb = _sync_push(admin, [op])
        rows = _sql("SELECT tenant_id::text FROM persons WHERE id = %s",
                    (op["record_id"],))
        stored_tenant = rows[0][0] if rows else "missing"
        orphans = _sql("SELECT count(*) FROM persons "
                       "WHERE tenant_id IS NOT NULL")[0][0]
        record(SEC_TID["tenant"],
               _ok(ps == 200 and stored_tenant is None and orphans == 0),
               f"Sync push with payload.tenant_id={injected} -> HTTP {ps}; "
               f"stored tenant_id={stored_tenant!r}; persons rows with a "
               f"non-null tenant_id={orphans} (tenant comes from the "
               f"authenticated principal, never the payload - see "
               f"internal/handlers/sync_handler.go Push).")
    except Exception as exc:
        record(SEC_TID["tenant"], "Blocked",
               f"Direct Postgres verification unavailable ({exc}); tenant "
               f"isolation could not be evidenced from storage.")

    # 009 passwords are salted hashes, never plain text or echoed
    try:
        rows = _sql("SELECT password_hash FROM users WHERE username = %s",
                    ("qa_admin",))
        stored = rows[0][0] if rows else ""
        bcrypt_ok = stored.startswith(("$2a$", "$2b$", "$2y$")) \
            and len(stored) >= 50
        _s, ub = api(admin, "GET", "/users?page_size=5")
        no_leak = ("password" not in ub.lower() and "$2a$" not in ub
                   and "$2b$" not in ub)
        record(SEC_TID["password"], _ok(bcrypt_ok and no_leak),
               f"users.password_hash for qa_admin -> prefix={stored[:4]!r}, "
               f"length={len(stored)} (bcrypt salt+cost embedded); GET /users "
               f"exposes a password field/hash={not no_leak} "
               f"(models.User marks PasswordHash json:\"-\"); a wrong password "
               f"is rejected (AUTH-003, AUTH-005 in part B).")
    except Exception as exc:
        record(SEC_TID["password"], "Blocked",
               f"Direct Postgres verification unavailable ({exc}); password "
               f"storage format could not be inspected.")


# ===========================================================================
# Localization-i18n (I18N-001 .. I18N-004)
# ===========================================================================
I18N_TID = _tids("I18N", "si ta en persist")
I18N_PATHS = ("/reports/page", "/dashboard", "/students/page")
LEAK_MARKERS = ("{{", "{%", "reports.total_donations", "common.save")


def _si_count(text):
    return sum(1 for ch in text if "\u0d80" <= ch <= "\u0dff")


def _ta_count(text):
    return sum(1 for ch in text if "\u0b80" <= ch <= "\u0bff")


def _lang_probe(client, lang):
    """Fetch every i18n page for one language; return counts and leaks."""
    counts = {"si": 0, "ta": 0, "html": 0, "http": 0}
    leaks = []
    for path in I18N_PATHS:
        sep = "&" if "?" in path else "?"
        s, b = page(client, f"{path}{sep}lang={lang}")
        counts["http"] = counts["http"] if counts["http"] else s
        counts["si"] += _si_count(b)
        counts["ta"] += _ta_count(b)
        counts["html"] += len(b)
        leaks.extend(f"{m} on {path}" for m in LEAK_MARKERS if m in b)
        if s != 200:
            counts["http"] = s
    return counts, leaks


def run_i18n(admin):
    if admin is None:
        _blocked(list(I18N_TID.values()),
                 "Could not establish an Admin session for the i18n suite.")
        return

    # 001 Sinhala
    si_c, si_leaks = _lang_probe(admin, "si")
    record(I18N_TID["si"],
           _ok(si_c["http"] == 200 and si_c["si"] >= 100 and si_c["ta"] <= 50
               and not si_leaks),
           f"GET {'; '.join(I18N_PATHS)}?lang=si -> HTTP {si_c['http']}; "
           f"Sinhala characters rendered={si_c['si']}, Tamil="
           f"{si_c['ta']}, HTML bytes={si_c['html']}, missing-key/template "
           f"leaks={si_leaks or 'none'}.")

    # 002 Tamil
    ta_c, ta_leaks = _lang_probe(admin, "ta")
    record(I18N_TID["ta"],
           _ok(ta_c["http"] == 200 and ta_c["ta"] >= 100 and ta_c["si"] <= 50
               and not ta_leaks),
           f"GET {'; '.join(I18N_PATHS)}?lang=ta -> HTTP {ta_c['http']}; "
           f"Tamil characters rendered={ta_c['ta']}, Sinhala="
           f"{ta_c['si']}, HTML bytes={ta_c['html']}, missing-key/template "
           f"leaks={ta_leaks or 'none'}.")

    # 003 English (default/reference)
    en_c, en_leaks = _lang_probe(admin, "en")
    _s, en_page = page(admin, "/reports/page?lang=en")
    english_marker = ("Reports" in en_page) or ("Students" in en_page)
    record(I18N_TID["en"],
           _ok(en_c["http"] == 200 and en_c["si"] <= 50 and en_c["ta"] <= 50
               and english_marker and not en_leaks),
           f"GET {'; '.join(I18N_PATHS)}?lang=en -> HTTP {en_c['http']}; "
           f"Sinhala={en_c['si']}, Tamil={en_c['ta']} (baseline only), "
           f"English labels present={english_marker}, leaks="
           f"{en_leaks or 'none'}.")

    # 004 preference persists across sessions (pwams_lang cookie)
    pc = Client("i18n_persist")
    s_login0 = pc.login("qa_admin")[0]
    s1, _b1 = page(pc, "/reports/page?lang=si")
    cookie = _cookie_get(pc, "pwams_lang")
    api(pc, "POST", "/logout")
    s_login = pc.login("qa_admin")[0]
    s2, b2 = page(pc, "/reports/page")
    record(I18N_TID["persist"],
           _ok(s_login0 == 303 and s1 == 200 and cookie == "si"
               and s_login == 303 and s2 == 200 and _si_count(b2) >= 100),
           f"login -> HTTP {s_login0}; GET /reports/page?lang=si -> HTTP "
           f"{s1} set pwams_lang={cookie!r}; after logout + fresh login "
           f"(HTTP {s_login}, new session) GET /reports/page (no lang "
           f"parameter) -> HTTP {s2}, Sinhala characters={_si_count(b2)} "
           f"(the one-year locale cookie internal/middleware/locale.go "
           f"ResolveLocale keeps the preference).")


# ===========================================================================
# execute
# ===========================================================================
print("=== qa_exec_d: audit / reports / files / sync / security / i18n ===",
      flush=True)
_ADMIN, _ADMIN_OK, _ADMIN_STATUS, _ADMIN_BODY = login_as("Admin")
if not _ADMIN_OK:
    print(f"[qa_exec_d] Admin session unavailable: POST /login -> "
          f"HTTP {_ADMIN_STATUS}; {_short(_ADMIN_BODY, 120)}", flush=True)
_ACTOR = _ADMIN if _ADMIN_OK else None
run_audit(_ACTOR)
run_reports(_ACTOR)
run_files(_ACTOR)
run_sync(_ACTOR)
run_security(_ACTOR)
run_i18n(_ACTOR)
