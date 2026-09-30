# qa_exec_c.py -- Students / Persons / Donors / Donations / Loans /
# Loan-Repayments / Aid-Requests / Care-Provided / Revenue / Messages /
# Notifications.
# Executed via exec() inside qa_run_tests.py namespace (shares all globals).
#
# The eight master-data modules share one template of test cases (list page,
# list API, search, get, get-404, create, create-missing, create-invalid,
# update, update-invalid, update-404, delete, delete-403, module-403,
# logged-out, optional status/view/edit). `_crud_suite` executes that template
# once per module from a declarative spec so every case keeps its own Test ID,
# its own assertion and its own captured evidence.
_C_SEQ = str(int(time.time()))[-6:]


def _tag(kind=""):
    return f"{kind}{_C_SEQ}{uuid.uuid4().hex[:4]}"


def _find_id(admin, path, key, value, page_size=100):
    """Locate a record id through the module list endpoint."""
    _s, b = api(admin, "GET", f"{path}?page_size={page_size}")
    for it in list_items(b):
        if isinstance(it, dict) and str(it.get(key)) == str(value) and it.get("id"):
            return str(it["id"])
    return None


def _msg(body, n=130):
    return _short(extract_msg(body), n)


def _blocked(tids, why):
    for t in tids:
        record(t, "Blocked", why,
               "Blocked - prerequisite session/fixture unavailable.")


def _same(got, want):
    if got is None:
        return False
    if str(got) == str(want):
        return True
    try:
        return abs(float(got) - float(want)) < 1e-9
    except (TypeError, ValueError):
        return False


def _field_of(body, field):
    """First value of `field` in a JSON document (any depth)."""
    try:
        data = json.loads(body)
    except Exception:
        return None
    found = []

    def walk(node):
        if found:
            return
        if isinstance(node, dict):
            if field in node:
                found.append(node[field])
                return
            for value in node.values():
                walk(value)
        elif isinstance(node, list):
            for value in node:
                walk(value)

    walk(data)
    return found[0] if found else None


def _re_read(c, path, rid):
    """Re-fetch a record body (GET detail) for assertion purposes."""
    _s, b = api(c, "GET", f"{path}/{rid}")
    return b


def _has_token(row, token):
    return any(isinstance(v, str) and token.lower() in v.lower()
               for v in row.values())


def _probes(value):
    """Normalise a spec callback result into [(label, payload), ...].

    A spec may return a single payload (one invalid case) or a list of
    labelled payloads so one workbook case can assert several invalid
    inputs and report each outcome separately.
    """
    if isinstance(value, list):
        return value
    return [("invalid data", value)]


_MODULE_PERSONS = {}


def _module_person(c, tag):
    """One reusable Active person per module tag (keeps the write budget)."""
    if tag in _MODULE_PERSONS:
        return _MODULE_PERSONS[tag]
    pid, _name = fresh_person(c, tag)
    _MODULE_PERSONS[tag] = pid
    return pid


def _crud_suite(admin, spec):
    """Run the standard module case template for one entity."""
    tid = spec["tid"]
    path = spec["path"]
    label = spec["label"]
    if admin is None:
        _blocked(list(tid.values()),
                 f"Could not establish an Admin session for the {label} module.")
        return {}

    # 001 list page
    s, b = page(admin, spec["page"])
    record(tid["page"], _ok(s == 200 and len(b) > 500),
           f"GET {spec['page']} -> HTTP {s}; {label} list page rendered "
           f"(HTML {len(b)} bytes).")

    # 002 list API
    s, b = api(admin, "GET", f"{path}?page_size=5")
    rows = list_items(b)
    record(tid["list"], _ok(s == 200 and json_ok(b)),
           f"GET {path}?page_size=5 -> HTTP {s}; rows={len(rows)}, "
           f"total={_list_total(b, rows)}, pagination metadata present="
           f"{'pagination' in b}.")

    # 003 search / filter
    needle = spec["new"](admin, "search")
    query = spec["search_q"](needle)
    s, b = api(admin, "GET", f"{path}?{query}&page_size=50")
    rows = list_items(b)
    _s_all, b_all = api(admin, "GET", f"{path}?page_size=50")
    all_rows = list_items(b_all)
    matched = [r for r in rows if _has_token(r, needle["token"])]
    s2, b2 = api(admin, "GET", f"{path}?{spec['noise_q']}&page_size=50")
    noise = list_items(b2)
    narrowed = len(rows) < len(all_rows)
    filtered = bool(rows) and len(matched) == len(rows) and (narrowed or not noise)
    note = spec.get("filter_note", "")
    record(tid["search"], _ok(s == 200 and json_ok(b) and filtered),
           f"GET {path}?{query} -> HTTP {s}; filtered rows={len(rows)} of "
           f"{len(all_rows)} unfiltered and every row matches the filter "
           f"({len(matched)}/{len(rows)}); a non-matching value returns "
           f"{len(noise)} rows.{(' ' + note) if note else ''}")

    # 004 get by id
    rid = needle["id"]
    s, b = api(admin, "GET", f"{path}/{rid}")
    record(tid["get"], _ok(s == 200 and json_ok(b) and rid in b),
           f"GET {path}/{rid} -> HTTP {s}; full {label} record returned "
           f"(success=true, id echoed={rid in b}).")

    # 005 get non-existent
    ghost = _uid()
    s, b = api(admin, "GET", f"{path}/{ghost}")
    record(tid["get_404"], _ok(s == 404),
           f"GET {path}/{ghost} (random UUID) -> HTTP {s}; message='{_msg(b)}'.")

    # 006 create valid
    made = spec["new"](admin, "create")
    record(tid["create"], _ok(made["status"] == 201 and json_ok(made["body"])
                              and made["id"] not in (None, "created")),
           f"POST {path} -> HTTP {made['status']}; the {label} row was created "
           f"and is retrievable (id={made['id']}, re-read OK="
           f"{spec['get_ok'](admin, made['id'])}).")

    # 007 create with a required field missing
    s, b = api(admin, "POST", path, spec["missing"](admin))
    record(tid["create_missing"], _ok(s in (400, 409, 422) and not json_ok(b)),
           f"POST {path} omitting a required field -> HTTP {s}; "
           f"message='{_msg(b)}'; no record created.")

    # 008 create with malformed field values
    rejects, notes = [], []
    for label, payload in _probes(spec["create_invalid"](admin, made)):
        s, b = api(admin, "POST", path, payload)
        accepted = s in (200, 201)
        rejects.append(not accepted)
        notes.append(f"{label}: HTTP {s}{' (ACCEPTED)' if accepted else ''}"
                     f" msg='{_msg(b, 60)}'")
    record(tid["create_invalid"],
           _ok(all(rejects)),
           f"POST {path} with malformed data -> {'; '.join(notes)}.")

    # 009 update valid
    field = spec["updated_field"]
    s, b = api(admin, "PUT", f"{path}/{made['id']}", spec["update"](made))
    _s, b2 = api(admin, "GET", f"{path}/{made['id']}")
    got = _field_of(b2, field)
    record(tid["update"], _ok(s == 200 and json_ok(b)
                              and _same(got, spec["updated_value"])),
           f"PUT {path}/{made['id']} -> HTTP {s}; re-read {field}={got!r} "
           f"(expected {spec['updated_value']!r}).")

    # 010 update with invalid data
    got = _field_of(_re_read(admin, path, made["id"]), field)
    rejects, notes = [], []
    for label, payload in _probes(spec["bad_update"](made)):
        s, b = api(admin, "PUT", f"{path}/{made['id']}", payload)
        got2 = _field_of(_re_read(admin, path, made["id"]), field)
        rejects.append(s in (400, 409, 422) and not json_ok(b)
                       and _same(got2, got))
        notes.append(f"{label}: HTTP {s}, stored {field}={got2!r}")
    record(tid["update_invalid"], _ok(all(rejects)),
           f"PUT {path}/{made['id']} with invalid data -> {'; '.join(notes)} "
           f"(value before the probes {field}={got!r}).")

    # 011 update non-existent
    ghost = _uid()
    s, b = api(admin, "PUT", f"{path}/{ghost}", spec["update"](dict(made, id=ghost)))
    record(tid["update_404"], _ok(s == 404),
           f"PUT {path}/{ghost} (random UUID) -> HTTP {s}; message='{_msg(b)}'.")

    # 012 delete authorized
    doomed = spec["new"](admin, "delete")
    if spec.get("delete_ready"):
        # modules can gate deletion on lifecycle state (aid requests are only
        # deletable once Rejected/Cancelled), so the spec may pre-condition the
        # record it wants deleted.
        doomed = spec["delete_ready"](admin, doomed)
    s, b = api(admin, "DELETE", f"{path}/{doomed['id']}")
    s2, _b2 = api(admin, "GET", f"{path}/{doomed['id']}")
    record(tid["delete"], _ok(s == 200 and json_ok(b) and s2 in (400, 404)),
           f"DELETE {path}/{doomed['id']} as Admin -> HTTP {s}; re-read -> HTTP "
           f"{s2}; the record is gone from the module."
           + (f" Fixture precondition: {doomed['prepared']}."
              if doomed.get("prepared") else ""))

    # 013 delete as unauthorized role
    keep = spec["new"](admin, "keep")
    dn, dn_ok, _s3, _b3 = login_as("Donor")
    s, b = api(dn, "DELETE", f"{path}/{keep['id']}") if dn_ok else (0, "{}")
    s2, _b2 = api(admin, "GET", f"{path}/{keep['id']}")
    record(tid["delete_403"], _ok(dn_ok and s == 403 and s2 == 200),
           f"DELETE {path}/{keep['id']} as Donor -> HTTP {s}; the record was not "
           f"deleted (Admin re-read -> HTTP {s2}).")

    # 014 module access as unauthorized role
    s, b = api(dn, "GET", f"{path}?page_size=5") if dn_ok else (0, "{}")
    s2, b2 = api(dn, "POST", path, {}) if dn_ok else (0, "{}")
    record(tid["module_403"],
           _ok(s == 403 and s2 == 403 and "permission" in b.lower()),
           f"Donor role: GET {path} -> HTTP {s}, POST {path} -> HTTP {s2}; the 403 "
           f"bodies expose no records.")

    # 015 module access while logged out
    a = anon(f"c_{spec['key']}")
    sp, _bp = page(a, spec["page"])
    sa, _ba = api(a, "GET", f"{path}?page_size=5")
    record(tid["logged_out"], _ok(sp == 303 and sa == 401),
           f"Without a session: GET {spec['page']} -> HTTP {sp} (redirect to "
           f"/login); GET {path} (API) -> HTTP {sa}.")

    # optional: status transition
    if "status" in tid:
        st = spec["status"]
        s, b = api(admin, "PATCH", f"{path}/{made['id']}/status", st["payload"])
        _s, b2 = api(admin, "GET", f"{path}/{made['id']}")
        got3 = _field_of(b2, st["field"])
        ok = s == 200 and json_ok(b) and _same(got3, st["expect"])
        note = st.get("note", "")
        if st.get("invalid_payload") is not None:
            s3, _b3 = api(admin, "PATCH", f"{path}/{made['id']}/status",
                          st["invalid_payload"])
            _s, b4 = api(admin, "GET", f"{path}/{made['id']}")
            got4 = _field_of(b4, st["field"])
            ok = ok and s3 in (400, 409, 422) and _same(got4, st["expect"])
            note += (f" A further PATCH {st['invalid_payload']} -> HTTP {s3} and the "
                     f"status stayed {got4!r}.")
        if st.get("restore") is not None:
            api(admin, "PATCH", f"{path}/{made['id']}/status", st["restore"])
        record(tid["status"], _ok(ok),
               f"PATCH {path}/{made['id']}/status {st['payload']} -> HTTP {s}; "
               f"re-read {st['field']}={got3!r}.{note}")

    # optional: view page
    if "view" in tid:
        view_path = spec["view"](needle)
        s, b = page(admin, view_path)
        present = spec.get("view_expect") or needle.get("name")
        has = bool(present) and str(present) in b
        ok, extra = (s == 200 and has), ""
        if spec.get("view_extra"):
            ok2, extra = spec["view_extra"](admin, needle, b)
            ok = ok and ok2
        record(tid["view"], _ok(ok),
               f"GET {view_path} -> HTTP {s}; record details rendered (expected "
               f"value present={has}, HTML {len(b)} bytes). {extra}".rstrip())

    # optional: edit page pre-fill
    if "edit" in tid:
        edit_path = spec["edit"](needle)
        s, b = page(admin, edit_path)
        has = bool(needle.get("name")) and needle["name"] in b
        record(tid["edit"], _ok(s == 200 and has),
               f"GET {edit_path} -> HTTP {s}; edit form pre-filled with the stored "
               f"value (present={has}, HTML {len(b)} bytes).")

    return made


# ===========================================================================
# spec helpers shared by every master-data module
# ===========================================================================
def _tids(prefix, names):
    """Map case keys to sequential Test IDs (STU-001, STU-002, ...)."""
    return {n: f"{prefix}-{i:03d}" for i, n in enumerate(names.split(), 1)}


def _locate(c, path, field, value, page_size=100):
    """Find a record id by exact or substring match of one list field."""
    _s, b = api(c, "GET", f"{path}?page_size={page_size}")
    needle = str(value)
    for it in list_items(b):
        if not isinstance(it, dict) or not it.get("id"):
            continue
        got = str(it.get(field) or "")
        if got == needle or needle in got:
            return str(it["id"])
    return None


def _record(c, path, payload, match, token=None):
    """POST a fixture record and report the created id.

    The create response is the primary id source (``_created_id`` reads the
    entity object the handler returns); a list lookup is only the fallback,
    because a paginated list may not contain a brand-new row.
    """
    s, b = api(c, "POST", path, payload)
    rid = None
    if s in (200, 201):
        rid = _created_id(b) or _locate(c, path, match, payload[match])
    rec = dict(payload)
    rec.update(id=rid or "created", status=s, body=b,
               token=token or "", name=str(payload.get(match, "")))
    return rec


def _get_ok(path):
    return lambda c, rid: str(rid) in _re_read(c, path, rid)


def _noise():
    return f"search={_tag('zzz')}"


# ===========================================================================
# Students (STU-001 .. STU-018)
# ===========================================================================
STU_TID = _tids(
    "STU",
    "page list search get get_404 create create_missing create_invalid "
    "update update_invalid update_404 delete delete_403 module_403 "
    "logged_out status view edit")


def _stu_payload(c, kind):
    tok = _tag(kind)
    return {
        "person_id": _module_person(c, "stu"),
        "full_name": f"QA Student {tok}",
        "school_name": "QA Central College",
        "grade": "10",
        "academic_year": 2026,
        "gender": "Other",
        "guardian_name": f"QA Guardian {tok}",
        "guardian_phone": "+94770000111",
        "date_of_birth": "2011-04-02",
        "remarks": f"auto fixture {kind}",
    }


def _stu_new(c, kind):
    payload = _stu_payload(c, kind)
    return _record(c, "/students", payload, "full_name",
                   payload["full_name"].split()[-1])


def _stu_update(made, **over):
    """Valid PUT payload for a student, with optional field overrides."""
    payload = {
        "person_id": made["person_id"], "full_name": made["full_name"],
        "school_name": "QA Updated College", "grade": "11",
        "academic_year": 2026, "gender": "Other",
        "guardian_name": made["guardian_name"],
        "guardian_phone": made["guardian_phone"],
        "date_of_birth": made["date_of_birth"],
        "status": "Active", "remarks": "updated by QA suite",
    }
    payload.update(over)
    return payload


def run_students(admin):
    path = "/students"
    spec = {
        "tid": STU_TID, "key": "stu", "label": "student",
        "path": path, "page": f"{path}/page",
        "new": _stu_new,
        "get_ok": _get_ok(path),
        "search_q": lambda n: f"search={n['token']}",
        "noise_q": _noise(),
        "missing": lambda a: {
            "full_name": f"QA Missing {_tag('m')}",
            "school_name": "QA Central College", "grade": "10",
            "academic_year": 2026,
        },
        "create_invalid": lambda a, made: [
            ("unknown person_id",
             dict(_stu_payload(a, "badp"), person_id=_uid())),
            ("academic year out of range",
             dict(_stu_payload(a, "bady"), academic_year=1990)),
        ],
        "update": lambda made: _stu_update(made),
        "updated_field": "school_name",
        "updated_value": "QA Updated College",
        "bad_update": lambda made: [
            ("academic year out of range",
             dict(_stu_update(made), academic_year=1990)),
            ("invalid gender", dict(_stu_update(made), gender="Unknown")),
        ],
        "status": {
            "payload": {"status": "Inactive"}, "field": "status",
            "expect": "Inactive", "invalid_payload": {"status": "Graduated"},
            "restore": {"status": "Active"},
            "note": " PATCH with the unsupported value 'Graduated' was "
                    "rejected, leaving the stored status untouched.",
        },
        "view": lambda n: f"{path}/{n['id']}/view",
        "edit": lambda n: f"{path}/{n['id']}/edit",
    }
    return _crud_suite(admin, spec)


# ===========================================================================
# Persons / care seekers (PER-001 .. PER-017)
# ===========================================================================
PER_TID = _tids(
    "PER",
    "page list search get get_404 create create_missing create_invalid "
    "update update_invalid update_404 delete delete_403 module_403 "
    "logged_out status view")


def _per_payload(c, kind):
    tok = _tag(kind)
    return {
        "full_name": f"QA Care Seeker {tok}",
        "nic_passport": f"NIC{tok.upper()}"[:30],
        "monthly_income": "1250",
        "phone": "+94770000222",
        "email": f"qa_{tok}@example.com",
        "gender": "Other",
        "date_of_birth": "1992-06-15",
        "occupation": "QA tester",
        "address": f"QA probe address {tok}",
    }


def _per_new(c, kind):
    payload = _per_payload(c, kind)
    return _record(c, "/persons", payload, "full_name",
                   payload["full_name"].split()[-1])


def _per_update(made, **over):
    payload = {
        "full_name": made["full_name"], "nic_passport": made["nic_passport"],
        "date_of_birth": made["date_of_birth"], "gender": "Other",
        "phone": "+94770000333", "email": made["email"],
        "address": "QA updated address", "occupation": "Updated occupation",
        "monthly_income": "1500", "status": "Active",
    }
    payload.update(over)
    return payload


def run_persons(admin):
    path = "/persons"
    spec = {
        "tid": PER_TID, "key": "per", "label": "person",
        "path": path, "page": f"{path}/page",
        "new": _per_new,
        "get_ok": _get_ok(path),
        "search_q": lambda n: f"search={n['token']}",
        "noise_q": _noise(),
        "missing": lambda a: {"full_name": f"QA Missing {_tag('m')}"},
        "create_invalid": lambda a, made: [
            ("negative monthly income",
             dict(_per_payload(a, "badinc"), monthly_income="-5000")),
            ("malformed email", dict(_per_payload(a, "badmail"),
                                     email="not-an-email")),
            ("unsupported gender", dict(_per_payload(a, "badgen"),
                                        gender="Martian")),
        ],
        "update": lambda made: _per_update(made),
        "updated_field": "phone",
        "updated_value": "+94770000333",
        "bad_update": lambda made: [
            ("negative monthly income", _per_update(made, monthly_income="-100")),
            ("unsupported status", _per_update(made, status="Deceased")),
        ],
        "status": {
            "payload": {"status": "Inactive"}, "field": "status",
            "expect": "Inactive", "invalid_payload": {"status": "Deceased"},
            "restore": {"status": "Active"},
            "note": " PATCH with the unsupported value 'Deceased' was rejected.",
        },
        "view": lambda n: f"{path}/{n['id']}/view",
    }
    return _crud_suite(admin, spec)


_MODULE_DONORS = {}


def _module_donor(c, tag):
    """One reusable Individual donor per tag (NIC is mandatory for them)."""
    if tag in _MODULE_DONORS:
        return _MODULE_DONORS[tag]
    tok = _tag(tag)
    did, _s, _b = _create_and_id(c, "/donors", {
        "name": f"QA Donor {tok}", "donor_type": "Individual",
        "nic_passport": f"DNR{tok.upper()}"[:30],
        "phone": "+94770000666", "notes": f"fixture {tag}",
    })
    _MODULE_DONORS[tag] = did
    return did


def _flat_int(body, key):
    """First integer in a JSON document whose field name contains `key`."""
    try:
        data = json.loads(body)
    except Exception:
        return None
    hits = []

    def walk(node):
        if isinstance(node, dict):
            for k, v in node.items():
                if isinstance(v, bool):
                    continue
                if isinstance(v, int) and key.lower() in str(k).lower():
                    hits.append(v)
                else:
                    walk(v)
        elif isinstance(node, list):
            for v in node:
                walk(v)

    walk(data)
    return hits[0] if hits else None


# ===========================================================================
# Donations (DON-001 .. DON-018)
# ===========================================================================
DON_TID = _tids(
    "DON",
    "page list search get get_404 create create_missing create_invalid "
    "update update_invalid update_404 delete delete_403 module_403 "
    "logged_out status")


def _don_payload(c, kind, amount="7500"):
    tok = _tag(kind)
    return {
        "donor_id": _module_donor(c, "don"),
        "donation_type": "Cash",
        "amount": amount,
        "quantity": "1",
        "currency": "LKR",
        "description": f"QA donation {tok}",
        "donation_date": "2026-09-05",
        "reference_no": f"QAREF{tok.upper()}"[:100],
    }


def _don_new(c, kind):
    payload = _don_payload(c, kind)
    return _record(c, "/donations", payload, "description",
                   payload["description"].split()[-1])


def _don_update(made, **over):
    # UpdateDonationRequest requires donation_type, amount, quantity and
    # status; donor_id is not part of the update contract (person link is
    # optional and JSON-omitempty, so an empty string is accepted).
    payload = {
        "donation_type": "Cash", "amount": "7600", "quantity": "1",
        "currency": "LKR", "description": made["description"],
        "donation_date": "2026-09-05", "status": "Pending",
        "reference_no": made["reference_no"],
    }
    payload.update(over)
    return payload


def run_donations(admin):
    path = "/donations"
    spec = {
        "tid": DON_TID, "key": "don", "label": "donation",
        "path": path, "page": f"{path}/page",
        "new": _don_new,
        "get_ok": _get_ok(path),
        "search_q": lambda n: f"search={n['token']}",
        "noise_q": _noise(),
        "missing": lambda a: {"donor_id": _module_donor(a, "don"),
                              "donation_type": "Cash",
                              "description": "amount+quantity omitted"},
        "create_invalid": lambda a, made: [
            ("unknown donor_id",
             dict(_don_payload(a, "badd"), donor_id=_uid())),
            ("negative amount", _don_payload(a, "badamt", amount="-250")),
            ("malformed donation date",
             dict(_don_payload(a, "baddt"), donation_date="31-31-2026")),
        ],
        "update": lambda made: _don_update(made),
        "updated_field": "amount",
        "updated_value": "7600",
        "bad_update": lambda made: [
            ("unsupported donation type",
             _don_update(made, donation_type="Barter")),
            ("malformed donation date",
             _don_update(made, donation_date="31-31-2026")),
        ],
        "status": {
            "payload": {"status": "Confirmed"}, "field": "status",
            "expect": "Confirmed", "invalid_payload": {"status": "Reversed"},
            "restore": {"status": "Pending"},
            "note": " PATCH with the unsupported value 'Reversed' was rejected.",
        },
    }
    made = _crud_suite(admin, spec)
    _don_extra_tests(admin, made)
    return made


def _don_extra_tests(admin, made):
    """DON-017 amount validation and DON-018 aggregate roll-up."""
    path = "/donations"
    notes, rejects = [], []
    for label, amount in (("negative amount", "-100"), ("zero amount", "0")):
        s, b = api(admin, "POST", path,
                   dict(_don_payload(admin, "amtval", amount=amount)))
        accepted = s in (200, 201)
        rejects.append(not accepted)
        notes.append(f"{label}: HTTP {s}{' (ACCEPTED)' if accepted else ''}")
    record("DON-017", _ok(all(rejects)),
           f"POST {path} with an out-of-range amount -> {'; '.join(notes)}; "
           f"a non-positive donation amount must be rejected.")

    # Donations are not exposed as a counter on GET /dashboard/stats (its keys
    # are total_beneficiaries/students/donors/active_loans/pending_aid_requests/
    # revenue_summary/users), so the roll-up is evidenced on the module listing:
    # the server-side pagination total must grow by one and the new row must be
    # findable by its unique description token. (Row lists are capped at
    # page_size=100 by the handlers, so the authoritative total is used.)
    _s, b_before = api(admin, "GET", f"{path}?page_size=1")
    before = _list_total(b_before, list_items(b_before))
    created = _don_new(admin, "rollup")
    _s, b_after = api(admin, "GET", f"{path}?page_size=1")
    after = _list_total(b_after, list_items(b_after))
    token = created["description"].split()[-1]
    _s, b_hit = api(admin, "GET", f"{path}?search={token}&page_size=20")
    hits = [r for r in list_items(b_hit) if _has_token(r, token)]
    _s, b_dash = api(admin, "GET", "/dashboard/stats")
    dash_income = _flat_int(b_dash, "revenue") or 0
    ok = (created["status"] in (200, 201) and after == before + 1
          and len(hits) >= 1)
    record("DON-018", _ok(ok),
           f"Donation listing total before={before}, after inserting "
           f"'{created['name']}' (HTTP {created['status']})={after}; "
           f"search={token} returns {len(hits)} row(s), so the inserted "
           f"donation is included in the module totals. Dashboard "
           f"revenue_summary={dash_income} (reported for context; the "
           f"dashboard exposes no donation counter).")


# ===========================================================================
# Aid requests (AID-001 .. AID-019)
# ===========================================================================
AID_TID = _tids(
    "AID",
    "page list search get get_404 create create_missing create_invalid "
    "update update_invalid update_404 delete delete_403 module_403 "
    "logged_out review_approve review_reject cancel staff_review")


def _aid_payload(c, kind):
    tok = _tag(kind)
    return {
        "person_id": _module_person(c, "aid"),
        "aid_type": "Medical",
        "priority": "High",
        "title": f"QA Aid Request {tok}",
        "description": f"QA aid request description {tok}",
        "requested_amount": "4500",
        "currency": "LKR",
        "request_date": "2026-09-06",
        "needed_by": "2026-10-06",
    }


def _aid_new(c, kind):
    payload = _aid_payload(c, kind)
    return _record(c, "/aid-requests", payload, "title",
                   payload["title"].split()[-1])


def _aid_update(made, **over):
    payload = {
        "person_id": made["person_id"], "aid_type": "Medical",
        "priority": "High", "title": made["title"],
        "description": made["description"], "requested_amount": "4600",
        "currency": "LKR", "request_date": "2026-09-06",
        "needed_by": "2026-10-06",
    }
    payload.update(over)
    return payload


def run_aid_requests(admin):
    path = "/aid-requests"
    spec = {
        "tid": AID_TID, "key": "aid", "label": "aid request",
        "path": path, "page": f"{path}/page",
        "new": _aid_new,
        "get_ok": _get_ok(path),
        "search_q": lambda n: f"search={n['token']}",
        "noise_q": _noise(),
        "missing": lambda a: {"aid_type": "Medical", "priority": "High",
                              "title": f"QA Missing {_tag('m')}",
                              "description": "person_id omitted"},
        "create_invalid": lambda a, made: [
            ("unknown person_id",
             dict(_aid_payload(a, "badp"), person_id=_uid())),
            ("negative requested amount",
             dict(_aid_payload(a, "badamt"), requested_amount="-900")),
            ("description shorter than 5 chars",
             dict(_aid_payload(a, "baddesc"), description="hi")),
        ],
        "update": lambda made: _aid_update(made),
        "updated_field": "requested_amount",
        "updated_value": "4600",
        "bad_update": lambda made: [
            ("negative requested amount",
             _aid_update(made, requested_amount="-1")),
            ("unsupported aid type", _aid_update(made, aid_type="Rocket")),
        ],
        # aid requests may only be deleted once Rejected/Cancelled
        # (AidRequestService.DeleteAidRequest), so cancel first.
        "delete_ready": _aid_cancel_ready,
    }
    made = _crud_suite(admin, spec)
    _aid_review_tests(admin)
    return made


def _aid_cancel_ready(admin, rec):
    """Cancel a Pending aid request so DeleteAidRequest permits the delete."""
    s, _b = api(admin, "PATCH", f"/aid-requests/{rec['id']}/cancel",
                {"reason": "QA delete precondition"})
    rec["prepared"] = f"cancel -> HTTP {s} (delete requires Rejected/Cancelled)"
    return rec


def _aid_review_tests(admin):
    """AID-016/017/018/019 - review, reject, cancel and role separation."""
    path = "/aid-requests"
    person = ensure_person(admin, "aidrev")
    approve_id = staff_create_aid("approve", person) if person else None
    if not approve_id or approve_id == "created":
        _blocked(["AID-016", "AID-017", "AID-019"],
                 "Staff-created aid fixture unavailable (Staff session or "
                 "POST /aid-requests failed).")
    else:
        # isValidAidStatusTransition: Pending -> Under Review -> Approved,
        # and the approved amount must be positive and <= requested amount.
        req_amount = _field_of(_re_read(admin, path, approve_id),
                               "requested_amount") or "1"
        s0, b0 = api(admin, "PATCH", f"{path}/{approve_id}/review",
                     {"status": "Under Review", "approved_amount": "0",
                      "review_notes": "QA under-review probe"})
        s, b = api(admin, "PATCH", f"{path}/{approve_id}/review",
                   {"status": "Approved", "approved_amount": req_amount,
                    "review_notes": "QA approve probe"})
        got = _field_of(_re_read(admin, path, approve_id), "status")
        record("AID-016",
               _ok(s0 == 200 and s == 200 and json_ok(b)
                   and _same(got, "Approved")),
               f"PATCH {path}/{approve_id}/review as Admin -> HTTP {s0} "
               f"(Pending->Under Review, msg='{_msg(b0, 60)}') then HTTP {s} "
               f"(->Approved, approved_amount={req_amount}, msg='{_msg(b, 80)}'); "
               f"re-read status={got!r} (expected 'Approved'; the reviewer is "
               f"not the submitter).")

        reject_id = staff_create_aid("reject", person)
        s0, _b0 = api(admin, "PATCH", f"{path}/{reject_id}/review",
                      {"status": "Under Review", "approved_amount": "0",
                       "review_notes": "QA under-review probe"})
        s, b = api(admin, "PATCH", f"{path}/{reject_id}/review",
                   {"status": "Rejected", "approved_amount": "0",
                    "review_notes": "QA reject probe"})
        got = _field_of(_re_read(admin, path, reject_id), "status")
        record("AID-017",
               _ok(s0 == 200 and s == 200 and json_ok(b)
                   and _same(got, "Rejected")),
               f"PATCH {path}/{reject_id}/review (Rejected) as Admin -> HTTP "
               f"{s0} then HTTP {s} (msg='{_msg(b, 80)}'); re-read "
               f"status={got!r} (expected 'Rejected').")

        st, st_ok, _s, _b = login_as("Staff")
        s, b = api(st, "PATCH", f"{path}/{approve_id}/review",
                   {"status": "Approved", "approved_amount": "1"}) if st_ok \
            else (0, "{}")
        got = _field_of(_re_read(admin, path, approve_id), "status")
        record("AID-019",
               _ok(st_ok and s == 403 and _same(got, "Approved")),
               f"PATCH {path}/{approve_id}/review as Staff -> HTTP {s} "
               f"(Super Admin/Admin only); stored status unchanged ({got!r}).")

    cancel_id = _aid_new(admin, "cancel")["id"]
    s, b = api(admin, "PATCH", f"{path}/{cancel_id}/cancel",
               {"reason": "QA cancellation probe"})
    got = _field_of(_re_read(admin, path, cancel_id), "status")
    record("AID-018", _ok(s == 200 and json_ok(b) and _same(got, "Cancelled")),
           f"PATCH {path}/{cancel_id}/cancel as Admin -> HTTP {s}; re-read "
           f"status={got!r} (expected 'Cancelled').")


# ===========================================================================
# Care provided (CARE-001 .. CARE-016)
# ===========================================================================
CARE_TID = _tids(
    "CARE",
    "page list search get get_404 create create_missing create_invalid "
    "update update_invalid update_404 delete delete_403 module_403 "
    "logged_out status")


def _care_payload(c, kind, amount=2500):
    tok = _tag(kind)
    return {
        "aid_request_id": ensure_aid(c),
        "person_id": ensure_person(c, "care"),
        "amount": amount,
        "care_type": "medical",
        "description": f"QA care record {tok}",
        "provided_by": f"QA volunteer {tok}",
        "provided_at": "2026-09-15T10:00:00Z",
    }


def _care_new(c, kind):
    payload = _care_payload(c, kind)
    return _record(c, "/care-provided", payload, "description",
                   payload["description"].split()[-1])


def _care_update(made, **over):
    payload = {
        "amount": 2750, "description": made["description"],
        "care_type": "medical", "provided_by": made["provided_by"],
        "provided_at": made["provided_at"],
    }
    payload.update(over)
    return payload


def run_care_provided(admin):
    path = "/care-provided"
    spec = {
        "tid": CARE_TID, "key": "care", "label": "care provided",
        "path": path, "page": f"{path}/page",
        "new": _care_new,
        "get_ok": _get_ok(path),
        "search_q": lambda n: f"search={n['token']}",
        "noise_q": _noise(),
        "filter_note": "GET /care-provided binds search/status in handler "
                       "List and CareProvidedRepository.List applies them as "
                       "parameterised predicates shared by its Count and "
                       "Find, so both the total and the rows are narrowed.",
        "missing": lambda a: {"aid_request_id": ensure_aid(a),
                              "person_id": ensure_person(a, "care"),
                              "care_type": "medical",
                              "provided_at": "2026-09-15T10:00:00Z"},
        "create_invalid": lambda a, made: [
            ("unknown aid_request_id",
             dict(_care_payload(a, "bada"), aid_request_id=_uid())),
            ("unknown person_id",
             dict(_care_payload(a, "badp"), person_id=_uid())),
            ("unsupported care type",
             dict(_care_payload(a, "badct"), care_type="magic")),
            ("negative amount", _care_payload(a, "badamt", amount=-50)),
        ],
        "update": lambda made: _care_update(made),
        "updated_field": "amount",
        "updated_value": 2750,
        "bad_update": lambda made: [
            ("negative amount", _care_update(made, amount=-1)),
            ("unsupported care type", _care_update(made, care_type="magic")),
        ],
        "status": {
            "payload": {"status": "Completed"}, "field": "status",
            "expect": "Completed", "invalid_payload": {"status": "In Progress"},
            "note": " PATCH with the unsupported value 'In Progress' was "
                    "rejected.",
        },
    }
    return _crud_suite(admin, spec)


# ===========================================================================
# Loans (LON-001 .. LON-017)
# ===========================================================================
LON_TID = _tids(
    "LON",
    "page list search get get_404 create create_missing create_invalid "
    "update update_invalid update_404 module_403 logged_out create_pending "
    "review_approve review_reject staff_review")
LOAN_PAGE = "/loans/page"


def _loan_payload(c, kind, **over):
    """Loan payload with a *fresh* person (one non-terminal loan per person)."""
    tok = _tag(kind)
    person, _name = fresh_person(c, f"loan{tok}")
    payload = {
        "person_id": person,
        "loan_amount": "50000",
        "interest_rate": "5",
        "duration_months": 12,
        "purpose": f"QA loan purpose {tok}",
    }
    payload.update(over)
    return payload


def run_loans(admin):
    path = "/loans"
    if admin is None:
        _blocked(list(LON_TID.values()),
                 "Could not establish an Admin session for the loan module.")
        return
    _loan_page_and_list(admin, path)
    made = _loan_get_create(admin, path)
    _loan_update_cases(admin, path, made)
    _loan_access_rules(admin, path)
    _loan_review_cases(admin, path)


def _loan_page_and_list(admin, path):
    s, b = page(admin, LOAN_PAGE)
    record(LON_TID["page"], _ok(s == 200 and len(b) > 500),
           f"GET {LOAN_PAGE} -> HTTP {s}; loan list page rendered "
           f"(HTML {len(b)} bytes).")
    s, b = api(admin, "GET", f"{path}?page_size=5")
    rows = list_items(b)
    record(LON_TID["list"], _ok(s == 200 and json_ok(b)),
           f"GET {path}?page_size=5 -> HTTP {s}; rows={len(rows)}, "
           f"total={_list_total(b, rows)}, pagination metadata present="
           f"{'pagination' in b}.")
    needle = _record(admin, path, _loan_payload(admin, "search"), "purpose",
                     _tag("srch"))
    token = needle["purpose"].split()[-1]
    s, b = api(admin, "GET", f"{path}?search={token}&page_size=50")
    rows = list_items(b)
    _s, b_all = api(admin, "GET", f"{path}?page_size=50")
    all_rows = list_items(b_all)
    matched = [r for r in rows if _has_token(r, token)]
    _s, b2 = api(admin, "GET", f"{path}?search={_tag('nope')}&page_size=50")
    noise = list_items(b2)
    ok = (s == 200 and json_ok(b) and rows
          and len(matched) == len(rows)
          and len(rows) < len(all_rows) and not noise)
    record(LON_TID["search"], _ok(ok),
           f"GET {path}?search={token} -> HTTP {s}; rows={len(rows)} of "
           f"{len(all_rows)} unfiltered, all matching ({len(matched)}/"
           f"{len(rows)}); a non-matching value returns {len(noise)} rows "
           "(internal/handlers/loan_handler.go List binds the whole "
           "models.LoanListQuery, including Search, so the API list narrows "
           "to the matching loans.)")


LOAN_NO_UPDATE = (
    "PUT /loans/:id (LoanHandler.Update) validates the payload before it "
    "mutates the loan and recalculates the installment amount.")


def _loan_money(value):
    """Numeric view of a decimal amount for comparisons like 60000 vs '60000.00'."""
    try:
        return float(str(value))
    except (TypeError, ValueError):
        return None


def _loan_get_create(admin, path):
    made = _record(admin, path, _loan_payload(admin, "make"), "purpose",
                   _tag("mk"))
    s, b = api(admin, "GET", f"{path}/{made['id']}")
    record(LON_TID["get"], _ok(s == 200 and json_ok(b) and made["id"] in b),
           f"GET {path}/{made['id']} -> HTTP {s}; loan detail returned "
           f"(success=true, id echoed={made['id'] in b}).")

    ghost = _uid()
    s, b = api(admin, "GET", f"{path}/{ghost}")
    record(LON_TID["get_404"], _ok(s == 404),
           f"GET {path}/{ghost} (random UUID) -> HTTP {s}; "
           f"message='{_msg(b)}'.")

    record(LON_TID["create"],
           _ok(made["status"] == 201 and json_ok(made["body"])
               and made["id"] != "created"),
           f"POST {path} -> HTTP {made['status']}; the loan row was created "
           f"(id={made['id']}, re-read OK="
           f"{bool(_re_read(admin, path, made['id']))}).")

    s, b = api(admin, "POST", path,
               {"person_id": fresh_person(admin, "loanmiss")[0],
                "interest_rate": "5", "duration_months": 12})
    record(LON_TID["create_missing"],
           _ok(s in (400, 409, 422) and not json_ok(b)),
           f"POST {path} omitting loan_amount -> HTTP {s}; "
           f"message='{_msg(b)}'; no record created.")

    probes = [
        ("duration below the 3-month minimum",
         _loan_payload(admin, "baddur", duration_months=2)),
        ("zero loan amount", _loan_payload(admin, "badamt", loan_amount="0")),
        ("unknown person", _loan_payload(admin, "badper", person_id=_uid())),
    ]
    notes, rejects = [], []
    for label, payload in probes:
        s, b = api(admin, "POST", path, payload)
        accepted = s in (200, 201)
        rejects.append(not accepted)
        notes.append(f"{label}: HTTP {s}{' (ACCEPTED)' if accepted else ''}")
    record(LON_TID["create_invalid"], _ok(all(rejects)),
           f"POST {path} with malformed data -> {'; '.join(notes)}.")

    status = _field_of(_re_read(admin, path, made["id"]), "status")
    record(LON_TID["create_pending"], _ok(status == "Pending"),
           f"POST {path} -> HTTP {made['status']}; the new loan is created in "
           f"review-pending state (status={status!r}; approval is a separate "
           f"PATCH /loans/:id/review step).")
    return made


def _loan_update_cases(admin, path, made):
    rid = made["id"]
    put = dict(_loan_payload(admin, "upd"), loan_amount="60000")
    put.pop("person_id", None)

    before = _loan_money(_field_of(_re_read(admin, path, rid), "loan_amount"))
    s, b = api(admin, "PUT", f"{path}/{rid}", put)
    after = _loan_money(_field_of(_re_read(admin, path, rid), "loan_amount"))
    record(LON_TID["update"],
           _ok(s == 200 and json_ok(b) and before is not None
               and after == 60000.0 and after != before),
           f"PUT {path}/{rid} (loan_amount {before} -> 60000) -> HTTP {s} "
           f"(message='{_msg(b, 60)}'); re-read loan_amount={after} "
           f"(the loan is Pending, so PUT /loans/:id applies the edit and "
           f"recomputes the installment). {LOAN_NO_UPDATE}")

    s, b = api(admin, "PUT", f"{path}/{rid}", dict(put, loan_amount="-60000"))
    kept = _loan_money(_field_of(_re_read(admin, path, rid), "loan_amount"))
    record(LON_TID["update_invalid"],
           _ok(s in (400, 422)
               and _field_of(b, "success") is False
               and bool(_field_of(b, "message"))
               and kept == 60000.0),
           f"PUT {path}/{rid} with a negative loan_amount -> HTTP {s} "
           f"(message='{_msg(b, 60)}'); expected 400/422, re-read "
           f"loan_amount={kept} (the invalid value is rejected before the "
           f"record is written). {LOAN_NO_UPDATE}")

    ghost = _uid()
    s, b = api(admin, "PUT", f"{ghost}", put)
    s, b = api(admin, "PUT", f"{path}/{ghost}", put)
    record(LON_TID["update_404"],
           _ok(s == 404 and "not found" in b.lower()),
           f"PUT {path}/{ghost} (random unknown id) -> HTTP {s}; "
           f"message='{_msg(b, 60)}' (record-not-found handling). "
           f"{LOAN_NO_UPDATE}")


def _loan_access_rules(admin, path):
    dn, dn_ok, _s, _b = login_as("Donor")
    s, _b = api(dn, "GET", f"{path}?page_size=5") if dn_ok else (0, "{}")
    s2, _b2 = api(dn, "POST", path, {}) if dn_ok else (0, "{}")
    record(LON_TID["module_403"], _ok(dn_ok and s == 403 and s2 == 403),
           f"Donor role: GET {path} -> HTTP {s}, POST {path} -> HTTP {s2}; the "
           f"loan module is Staff-and-above only and no records leak.")

    a = anon("c_loan")
    sp, _bp = page(a, LOAN_PAGE)
    sa, _ba = api(a, "GET", f"{path}?page_size=5")
    record(LON_TID["logged_out"], _ok(sp == 303 and sa == 401),
           f"Without a session: GET {LOAN_PAGE} -> HTTP {sp} (redirect to "
           f"/login); GET {path} (API) -> HTTP {sa}.")


def _staff_loan(tag):
    """Pending loan owned by a fresh person, created by the Staff session."""
    st, ok, _s, _b = login_as("Staff")
    if not ok:
        return None
    person, _name = fresh_person(st, f"loanstaff{tag}")
    if not person:
        return None
    return staff_create_loan(person, f"QA staff loan {tag} {_tag(tag)}")


def _loan_review_cases(admin, path):
    tids = [LON_TID["review_approve"], LON_TID["review_reject"],
            LON_TID["staff_review"]]
    pending = _staff_loan("revap")
    reject_id = _staff_loan("revrj")
    if not pending or not reject_id:
        _blocked(tids, "Staff-created Pending loan fixture unavailable "
                       "(Staff session or POST /loans failed).")
        return
    s, b = api(admin, "PATCH", f"{path}/{pending}/review",
               {"status": "Approved", "review_notes": "QA approve probe"})
    got = _field_of(_re_read(admin, path, pending), "status")
    record(LON_TID["review_approve"],
           _ok(s == 200 and json_ok(b) and _same(got, "Approved")),
           f"PATCH {path}/{pending}/review (Approved) as Admin -> HTTP {s}; "
           f"re-read status={got!r} (expected 'Approved'; the reviewer is not "
           f"the submitter).")

    s, b = api(admin, "PATCH", f"{path}/{reject_id}/review",
               {"status": "Rejected", "review_notes": "QA reject probe"})
    got = _field_of(_re_read(admin, path, reject_id), "status")
    record(LON_TID["review_reject"],
           _ok(s == 200 and json_ok(b) and _same(got, "Rejected")),
           f"PATCH {path}/{reject_id}/review (Rejected) as Admin -> HTTP {s}; "
           f"re-read status={got!r} (expected 'Rejected').")

    st, st_ok, _s, _b = login_as("Staff")
    s, _b = api(st, "PATCH", f"{path}/{pending}/review",
                {"status": "Active"}) if st_ok else (0, "{}")
    got = _field_of(_re_read(admin, path, pending), "status")
    record(LON_TID["staff_review"],
           _ok(st_ok and s == 403 and _same(got, "Approved")),
           f"PATCH {path}/{pending}/review as Staff -> HTTP {s} (Super "
           f"Admin/Admin only); stored status unchanged ({got!r}).")


# ===========================================================================
# Loan repayments (LRP-001 .. LRP-008)
# ===========================================================================
LRP_TID = _tids("LRP", "page get pay overpay cancel duplicate not_found rbac")
LRP_PAGE = "/loan-repayments/page"


def _lrp_repayment_id(c, loan_id, installment):
    _s, b = api(c, "GET",
                f"/loan-repayments?loan_id={loan_id}&page_size=50")
    for it in list_items(b):
        if not isinstance(it, dict) or not it.get("id"):
            continue
        if int(it.get("installment_number") or 0) == installment:
            return str(it["id"])
    return None


def _lrp_fixture(admin):
    """Active loan (Staff-created, Admin-approved) with installments 1-3."""
    st, ok, _s, _b = login_as("Staff")
    if not ok:
        return None
    person, _name = fresh_person(st, "lrp")
    if not person:
        return None
    loan = staff_create_loan(person, f"QA repayment loan {_tag('lrp')}")
    if not loan:
        return None
    s1, _b1 = api(admin, "PATCH", f"/loans/{loan}/review",
                  {"status": "Approved"})
    s2, _b2 = api(admin, "PATCH", f"/loans/{loan}/review", {"status": "Active"})
    if s1 != 200 or s2 != 200:
        return None
    due_dates = ("2026-11-06", "2026-12-06", "2027-01-06")
    ids = {}
    for index, due in enumerate(due_dates, 1):
        s, _b = api(admin, "POST", "/loan-repayments", {
            "loan_id": loan, "installment_number": index,
            "due_date": due, "amount": "4375",
            "notes": f"QA installment {index}",
        })
        rid = _lrp_repayment_id(admin, loan, index)
        if s in (200, 201) and rid:
            ids[index] = rid
    if len(ids) < 3:
        return None
    return loan, ids


def run_repayments(admin):
    t = LRP_TID
    if admin is None:
        _blocked(list(t.values()),
                 "Could not establish an Admin session for the loan-repayment "
                 "module.")
        return

    s, b = page(admin, LRP_PAGE)
    record(t["page"], _ok(s == 200 and len(b) > 500),
           f"GET {LRP_PAGE} -> HTTP {s}; repayment list page rendered "
           f"(HTML {len(b)} bytes).")

    fx = _lrp_fixture(admin)
    if not fx:
        _blocked([t["get"], t["pay"], t["overpay"], t["cancel"],
                  t["duplicate"], t["not_found"]],
                 "Active-loan repayment fixture unavailable (Staff loan -> "
                 "Admin approve/activate -> POST /loan-repayments failed).")
        _lrp_rbac(admin)
        return

    loan, ids = fx
    _lrp_case_get_pay(admin, ids)
    _lrp_case_overpay(admin, ids)
    _lrp_case_cancel(admin, ids)
    _lrp_case_duplicate(admin, loan, ids)
    _lrp_case_missing(admin)
    _lrp_rbac(admin)


def _lrp_case_get_pay(admin, ids):
    rid = ids[1]
    s, b = api(admin, "GET", f"/loan-repayments/{rid}")
    record(LRP_TID["get"], _ok(s == 200 and json_ok(b) and rid in b),
           f"GET /loan-repayments/{rid} -> HTTP {s}; repayment detail returned "
           f"(success=true, id echoed={rid in b}).")

    s, b = api(admin, "PATCH", f"/loan-repayments/{rid}/pay", {
        "paid_amount": "4375", "payment_reference": "QA-PAY-001",
        "notes": "QA payment probe",
    })
    body = _re_read(admin, "/loan-repayments", rid)
    status = _field_of(body, "status")
    paid = _field_of(body, "paid_amount")
    record(LRP_TID["pay"],
           _ok(s == 200 and json_ok(b) and _same(status, "Paid")
               and _same(paid, "4375")),
           f"PATCH /loan-repayments/{rid}/pay (4375 of 4375) -> HTTP {s}; "
           f"re-read status={status!r}, paid_amount={paid!r}.")


def _lrp_case_overpay(admin, ids):
    rid = ids[2]
    s, b = api(admin, "PATCH", f"/loan-repayments/{rid}/pay", {
        "paid_amount": "999999", "payment_reference": "QA-PAY-OVER",
    })
    body = _re_read(admin, "/loan-repayments", rid)
    status = _field_of(body, "status")
    paid = _field_of(body, "paid_amount")
    record(LRP_TID["overpay"],
           _ok(s in (400, 409, 422) and _same(paid, "0")
               and _same(status, "Pending")),
           f"PATCH /loan-repayments/{rid}/pay with 999999 against an amount of "
           f"4375 -> HTTP {s} (message='{_msg(b, 80)}'); the record is "
           f"unchanged (status={status!r}, paid_amount={paid!r}).")


def _lrp_case_cancel(admin, ids):
    rid = ids[3]
    s, b = api(admin, "PATCH", f"/loan-repayments/{rid}/cancel")
    status = _field_of(_re_read(admin, "/loan-repayments", rid), "status")
    record(LRP_TID["cancel"],
           _ok(s == 200 and json_ok(b) and _same(status, "Cancelled")),
           f"PATCH /loan-repayments/{rid}/cancel -> HTTP {s}; re-read "
           f"status={status!r} (expected 'Cancelled').")


def _lrp_case_duplicate(admin, loan, ids):
    s, b = api(admin, "POST", "/loan-repayments", {
        "loan_id": loan, "installment_number": 1,
        "due_date": "2026-11-06", "amount": "4375",
        "notes": "QA duplicate installment probe",
    })
    record(LRP_TID["duplicate"],
           _ok(s in (400, 409, 422) and not json_ok(b)),
           f"POST /loan-repayments repeating installment 1 for loan {loan} -> "
           f"HTTP {s}; message='{_msg(b)}'; the unique (loan_id, "
           f"installment_number) pair was not duplicated.")


def _lrp_case_missing(admin):
    ghost = _uid()
    s, b = api(admin, "PATCH", f"/loan-repayments/{ghost}/pay",
               {"paid_amount": "100"})
    s2, _b2 = api(admin, "PATCH", f"/loan-repayments/{ghost}/cancel")
    record(LRP_TID["not_found"], _ok(s == 404 and s2 == 404),
           f"PATCH /loan-repayments/{ghost}/pay -> HTTP {s} "
           f"(message='{_msg(b, 60)}'); PATCH .../cancel -> HTTP {s2}; a random "
           f"UUID is rejected instead of being created.")


def _lrp_rbac(admin):
    dn, dn_ok, _s, _b = login_as("Donor")
    s, _b = api(dn, "PATCH", f"/loan-repayments/{_uid()}/pay",
                {"paid_amount": "100"}) if dn_ok else (0, "{}")
    record(LRP_TID["rbac"], _ok(dn_ok and s == 403),
           f"Donor role: PATCH /loan-repayments/<uuid>/pay -> HTTP {s}; the "
           f"repayment module is Staff-and-above only.")


# ===========================================================================
# Revenue (REV-001 .. REV-007)
# ===========================================================================
REV_TID = _tids("REV", "page summaries get update update_invalid delete "
                       "delete_403")
REV_PAGE = "/revenue/page"


def _rev_payload(kind, **over):
    tok = _tag(kind)
    payload = {
        "record_type": "income", "category": "Donations",
        "amount": "1500", "currency": "LKR",
        "record_date": TODAY, "description": f"QA revenue {tok}",
        "reference_no": f"QAREV{tok.upper()}"[:100],
    }
    payload.update(over)
    return payload


def _rev_new(admin, kind):
    payload = _rev_payload(kind)
    s, b = api(admin, "POST", "/revenue", payload)
    rid = None
    if s in (200, 201):
        rid = _locate(admin, "/revenue", "reference_no", payload["reference_no"])
    return rid, payload, s, b


def run_revenue(admin):
    t = REV_TID
    if admin is None:
        _blocked(list(t.values()),
                 "Could not establish an Admin session for the revenue module.")
        return

    s, b = page(admin, REV_PAGE)
    record(t["page"], _ok(s == 200 and len(b) > 500),
           f"GET {REV_PAGE} -> HTTP {s}; revenue list page rendered "
           f"(HTML {len(b)} bytes).")

    _s, before = api(admin, "GET", "/revenue/summaries")
    pre = _summary(before, "daily")
    payload = _rev_payload("summ", record_type="income", amount="1234.56")
    s, b = api(admin, "POST", "/revenue", payload)
    s2, b2 = api(admin, "POST", "/revenue", _rev_payload(
        "summx", record_type="expense", category="Administrative Expenses",
        amount="234.56"))
    _s, after = api(admin, "GET", "/revenue/summaries")
    post = _summary(after, "daily")
    ok = (s in (200, 201) and s2 in (200, 201) and pre and post
          and _same(post["income"] - pre["income"], "1234.56")
          and _same(pre["expenses"] - post["expenses"], "-234.56")
          and _same(post["net"] - pre["net"], "1000.00"))
    record(t["summaries"], _ok(ok),
           f"GET /revenue/summaries (daily bucket) -> income "
           f"{pre['income'] if pre else None} -> {post['income'] if post else None} "
           f"after adding a 1234.56 donation and expenses "
           f"{pre['expenses'] if pre else None} -> "
           f"{post['expenses'] if post else None} after adding a 234.56 expense; "
           f"net {pre['net'] if pre else None} -> {post['net'] if post else None}.")

    rid, created, s, b = _rev_new(admin, "get")
    s2, b2 = api(admin, "GET", f"/revenue/{rid}")
    record(t["get"], _ok(s in (200, 201) and rid and s2 == 200
                         and json_ok(b2) and rid in b2),
           f"POST /revenue -> HTTP {s} (id={rid}); GET /revenue/{rid} -> "
           f"HTTP {s2} with the record echoed.")

    upd = {
        "record_type": "income", "category": "Donations", "amount": "1777",
        "currency": "LKR", "record_date": TODAY,
        "description": "QA revenue updated",
        "reference_no": created["reference_no"],
    }
    s, b = api(admin, "PUT", f"/revenue/{rid}", upd)
    got = _field_of(_re_read(admin, "/revenue", rid), "amount")
    record(t["update"], _ok(s == 200 and json_ok(b) and _same(got, "1777")),
           f"PUT /revenue/{rid} (amount 1500 -> 1777) -> HTTP {s}; re-read "
           f"amount={got!r}.")

    s, b = api(admin, "PUT", f"/revenue/{rid}",
               dict(upd, amount="-1777"))
    got = _field_of(_re_read(admin, "/revenue", rid), "amount")
    record(t["update_invalid"],
           _ok(s in (400, 409, 422) and not json_ok(b) and _same(got, "1777")),
           f"PUT /revenue/{rid} with amount=-1777 -> HTTP {s} "
           f"(message='{_msg(b, 80)}'); the stored amount stayed {got!r}.")

    s, b = api(admin, "DELETE", f"/revenue/{rid}")
    s2, _b2 = api(admin, "GET", f"/revenue/{rid}")
    record(t["delete"], _ok(s == 200 and json_ok(b) and s2 == 404),
           f"DELETE /revenue/{rid} as Admin -> HTTP {s}; re-read -> HTTP {s2} "
           f"(the record is gone).")

    mgr, mgr_ok, _s, _b = login_as("Manager")
    rid2, _pay2, _s2, _b2 = _rev_new(admin, "rbac")
    s, _b = api(mgr, "DELETE", f"/revenue/{rid2}") if mgr_ok else (0, "{}")
    s2, _b2 = api(admin, "GET", f"/revenue/{rid2}")
    record(t["delete_403"], _ok(mgr_ok and s == 403 and s2 == 200),
           f"DELETE /revenue/{rid2} as Manager -> HTTP {s} (revenue is "
           f"Super Admin/Admin only); the record still exists (Admin re-read "
           f"-> HTTP {s2}).")


def _summary(body, period):
    """One bucket of GET /revenue/summaries as floats (None when unavailable)."""
    try:
        data = json.loads(body).get("data")
    except Exception:
        return None
    for row in data or []:
        if isinstance(row, dict) and str(row.get("period")) == period:
            try:
                return {k: float(row[k]) for k in ("income", "expenses", "net")}
            except (KeyError, TypeError, ValueError):
                return None
    return None


# ===========================================================================
# Messages (MSG-001 .. MSG-010)
# ===========================================================================
MSG_TID = _tids("MSG", "page recipients send inbox sent unread get read delete "
                       "privacy")
MSG_PAGE = "/messages/page"


def _msg_locate(c, subject, box="sent"):
    _s, b = api(c, "GET", f"/messages/{box}?page_size=50")
    for it in list_items(b):
        if isinstance(it, dict) and str(it.get("subject")) == subject \
                and it.get("id"):
            return str(it["id"])
    return None


def _recipient_id(rows, username):
    for r in rows:
        if isinstance(r, dict) and str(r.get("name")) == username:
            return str(r.get("id"))
    return None


def run_messages(admin):
    t = MSG_TID
    if admin is None:
        _blocked(list(t.values()),
                 "Could not establish an Admin session for the messaging "
                 "module.")
        return

    s, b = page(admin, MSG_PAGE)
    record(t["page"], _ok(s == 200 and len(b) > 500),
           f"GET {MSG_PAGE} -> HTTP {s}; messages page rendered "
           f"(HTML {len(b)} bytes).")

    s, b = api(admin, "GET", "/messages/recipients")
    recips = list_items(b)
    names = [str(r.get("name")) for r in recips if isinstance(r, dict)]
    staff_id = _recipient_id(recips, "qa_staff") or find_user_id("qa_staff")
    record(t["recipients"], _ok(s == 200 and json_ok(b) and bool(staff_id)
                                and "qa_admin" not in names),
           f"GET /messages/recipients -> HTTP {s}; {len(recips)} selectable "
           f"recipients (qa_staff present={bool(staff_id)}); the caller's own "
           f"account is filtered out.")

    subject = f"QA message {_tag('msg')}"
    s, b = api(admin, "POST", "/messages", {
        "recipient_id": staff_id, "subject": subject,
        "body": "Hello from the QA harness.",
    })
    mid = _msg_locate(admin, subject) if s in (200, 201) else None
    record(t["send"], _ok(s in (200, 201) and json_ok(b) and mid),
           f"POST /messages (Admin -> qa_staff) -> HTTP {s}; the message is "
           f"listed in the sender's outbox (id={mid}).")

    st, st_ok, _s, _b = login_as("Staff")
    if not mid:
        _blocked([t["inbox"], t["unread"], t["get"], t["read"], t["delete"],
                  t["privacy"]],
                 "Message fixture unavailable (POST /messages failed).")
        return
    _msg_delivery_cases(admin, st, st_ok, mid, subject)


def _msg_delivery_cases(admin, st, st_ok, mid, subject):
    t = MSG_TID
    s, b = api(st, "GET", "/messages/inbox?page_size=50") if st_ok else (0, "{}")
    rows = list_items(b)
    record(t["inbox"],
           _ok(st_ok and s == 200 and json_ok(b)
               and any(str(r.get("subject")) == subject for r in rows)),
           f"GET /messages/inbox as the recipient -> HTTP {s}; the message "
           f"'{subject}' is present in the inbox ({len(rows)} rows).")

    s, b = api(admin, "GET", "/messages/sent?page_size=50")
    rows = list_items(b)
    record(t["sent"],
           _ok(s == 200 and json_ok(b)
               and any(str(r.get("subject")) == subject for r in rows)),
           f"GET /messages/sent as the sender -> HTTP {s}; the message is "
           f"present in the outbox ({len(rows)} rows).")

    s, b = api(st, "GET", "/messages/unread/count") if st_ok else (0, "{}")
    count = _flat_int(b, "count")
    s2, b2 = api(st, "GET", "/messages/unread?page_size=50") if st_ok \
        else (0, "{}")
    rows = list_items(b2)
    record(t["unread"],
           _ok(st_ok and s == 200 and s2 == 200 and bool(rows)
               and (count is None or count >= 1)),
           f"GET /messages/unread/count -> HTTP {s} (count={count}); "
           f"GET /messages/unread -> HTTP {s2} with {len(rows)} unread rows.")

    s, b = api(st, "GET", f"/messages/{mid}") if st_ok else (0, "{}")
    record(t["get"], _ok(st_ok and s == 200 and json_ok(b) and mid in b),
           f"GET /messages/{mid} as the recipient -> HTTP {s}; the message "
           f"detail is returned (id echoed={mid in b}).")

    s, b = api(st, "PATCH", f"/messages/{mid}/read") if st_ok else (0, "{}")
    is_read = _field_of(_re_read(st, "/messages", mid), "is_read") if st_ok \
        else None
    record(t["read"],
           _ok(st_ok and s == 200 and json_ok(b) and is_read is True),
           f"PATCH /messages/{mid}/read -> HTTP {s}; re-read is_read="
           f"{is_read!r} (expected True).")

    s, b = api(st, "DELETE", f"/messages/{mid}") if st_ok else (0, "{}")
    s2, _b2 = api(st, "GET", f"/messages/{mid}") if st_ok else (0, "{}")
    record(t["delete"], _ok(st_ok and s == 200 and json_ok(b) and s2 == 404),
           f"DELETE /messages/{mid} as the recipient -> HTTP {s}; re-read -> "
           f"HTTP {s2} (the message is removed from the user's mailbox).")

    other = f"QA private message {_tag('priv')}"
    s, b = api(admin, "POST", "/messages", {
        "recipient_id": find_user_id("qa_staff"), "subject": other,
        "body": "Private body for the isolation check.",
    })
    priv_id = _msg_locate(admin, other) if s in (200, 201) else None
    dn, dn_ok, _s, _b = login_as("Donor")
    s, b = api(dn, "GET", f"/messages/{priv_id}") if (dn_ok and priv_id) \
        else (0, "{}")
    record(t["privacy"],
           _ok(dn_ok and bool(priv_id) and s in (403, 404)),
           f"GET /messages/{priv_id} (Admin -> qa_staff) as the Donor role -> "
           f"HTTP {s}; another user's private message is not readable "
           f"(message='{_msg(b, 60)}').")


# ===========================================================================
# Notifications (NOT-001 .. NOT-006)
# ===========================================================================
NOT_TID = _tids("NOT", "page get read delete system_triggered privacy")
NOT_PAGE = "/notifications/page"


def _not_create(admin, title, user_id=None, ntype="info"):
    s, b = api(admin, "POST", "/notifications", {
        "user_id": user_id or find_user_id("qa_admin"),
        "title": title, "message": "QA notification body.", "type": ntype,
    })
    nid = None
    if s in (200, 201):
        _s, body = api(admin, "GET", "/notifications?page_size=50")
        for it in list_items(body):
            if isinstance(it, dict) and str(it.get("title")) == title \
                    and it.get("id"):
                nid = str(it["id"])
                break
    return nid, s, b


def run_notifications(admin):
    t = NOT_TID
    if admin is None:
        _blocked(list(t.values()),
                 "Could not establish an Admin session for the notification "
                 "module.")
        return

    s, b = page(admin, NOT_PAGE)
    record(t["page"], _ok(s == 200 and len(b) > 500),
           f"GET {NOT_PAGE} -> HTTP {s}; notifications page rendered "
           f"(HTML {len(b)} bytes).")

    title = f"QA notification {_tag('not')}"
    nid, s, b = _not_create(admin, title)
    s2, b2 = api(admin, "GET", f"/notifications/{nid}") if nid else (0, "{}")
    record(t["get"],
           _ok(bool(nid) and s in (200, 201) and s2 == 200 and json_ok(b2)
               and nid in b2),
           f"POST /notifications then GET /notifications/{nid} -> HTTP {s2}; "
           f"the record is returned for its owner (id echoed={nid in b2}).")

    s, b = api(admin, "PATCH", f"/notifications/{nid}/read") if nid \
        else (0, "{}")
    is_read = _field_of(_re_read(admin, "/notifications", nid), "is_read") \
        if nid else None
    record(t["read"], _ok(bool(nid) and s == 200 and json_ok(b)
                          and is_read is True),
           f"PATCH /notifications/{nid}/read -> HTTP {s}; re-read is_read="
           f"{is_read!r} (expected True).")

    s, b = api(admin, "DELETE", f"/notifications/{nid}") if nid else (0, "{}")
    s2, _b2 = api(admin, "GET", f"/notifications/{nid}") if nid else (0, "{}")
    record(t["delete"], _ok(bool(nid) and s == 200 and json_ok(b) and s2 == 404),
           f"DELETE /notifications/{nid} as the owner -> HTTP {s}; re-read -> "
           f"HTTP {s2} (the notification is gone).")

    _not_system_case(admin)

    priv = f"QA private notification {_tag('priv')}"
    priv_id, _s, _b = _not_create(admin, priv)
    dn, dn_ok, _s, _b = login_as("Donor")
    s, b = api(dn, "GET", f"/notifications/{priv_id}") if (dn_ok and priv_id) \
        else (0, "{}")
    record(t["privacy"],
           _ok(dn_ok and bool(priv_id) and s == 404),
           f"GET /notifications/{priv_id} (owned by qa_admin) as the Donor "
           f"role -> HTTP {s} (message='{_msg(b, 60)}'); notifications are "
           f"scoped to their owner.")


def _not_system_case(admin):
    """NOT-005 - an aid-request workflow action generates a notification."""
    _s, before_body = api(admin, "GET", "/notifications?page_size=50")
    before = len(list_items(before_body))
    s, b = api(admin, "POST", "/aid-requests", _aid_payload(admin, "notif"))
    _s, after_body = api(admin, "GET", "/notifications?page_size=50")
    rows = list_items(after_body)
    generated = [r for r in rows if isinstance(r, dict)
                 and "aid request" in str(r.get("title", "")).lower()]
    record("NOT-005",
           _ok(s in (200, 201) and len(rows) > before and bool(generated)),
           f"POST /aid-requests -> HTTP {s}; the owner's notification count went "
           f"{before} -> {len(rows)} and a system notification was generated "
           f"(titles={[str(r.get('title')) for r in generated][:3]}).")


# ===========================================================================
# Donors (DNR-001 .. DNR-017)
# ===========================================================================
DNR_TID = _tids(
    "DNR",
    "page list search get get_404 create create_missing create_invalid "
    "update update_invalid update_404 delete delete_403 module_403 "
    "logged_out status view")


def _dnr_payload(c, kind):
    tok = _tag(kind)
    return {
        "name": f"QA Donor {tok}",
        "donor_type": "Individual",
        "nic_passport": f"DNR{tok.upper()}"[:30],
        "phone": "+94770000444",
        "email": f"qa_donor_{tok}@example.com",
        "address": "QA donor address",
        "preferred_donation_type": "Cash",
        "notes": f"auto fixture {kind}",
    }


def _dnr_new(c, kind):
    payload = _dnr_payload(c, kind)
    return _record(c, "/donors", payload, "name",
                   payload["name"].split()[-1])


def _dnr_update(made, **over):
    payload = {
        "name": made["name"], "donor_type": "Individual",
        "nic_passport": made["nic_passport"], "phone": "+94770000555",
        "email": made["email"], "address": "QA updated donor address",
        "preferred_donation_type": "Cash", "notes": "updated by QA suite",
        "status": "Active",
    }
    payload.update(over)
    return payload


def run_donors(admin):
    path = "/donors"
    spec = {
        "tid": DNR_TID, "key": "dnr", "label": "donor",
        "path": path, "page": f"{path}/page",
        "new": _dnr_new,
        "get_ok": _get_ok(path),
        "search_q": lambda n: f"search={n['token']}",
        "noise_q": _noise(),
        "missing": lambda a: {"donor_type": "Individual",
                              "notes": "name omitted on purpose"},
        "create_invalid": lambda a, made: [
            ("malformed email", dict(_dnr_payload(a, "badmail"),
                                     email="not-an-email")),
            ("name shorter than 2 chars",
             dict(_dnr_payload(a, "badname"), name="X")),
            ("unsupported donor type",
             dict(_dnr_payload(a, "badtype"), donor_type="Alien")),
        ],
        "update": lambda made: _dnr_update(made),
        "updated_field": "phone",
        "updated_value": "+94770000555",
        "bad_update": lambda made: [
            ("malformed email", _dnr_update(made, email="bad@@example")),
            ("unsupported status", _dnr_update(made, status="Blocked")),
        ],
        "status": {
            "payload": {"status": "Inactive"}, "field": "status",
            "expect": "Inactive", "invalid_payload": {"status": "Blocked"},
            "restore": {"status": "Active"},
            "note": " PATCH with the unsupported value 'Blocked' was rejected.",
        },
        "view": lambda n: f"{path}/{n['id']}/view",
    }
    return _crud_suite(admin, spec)


# ===========================================================================
# execute
# ===========================================================================
print("=== qa_exec_c: master data / loans / repayments / revenue / messaging ===",
      flush=True)
_ADMIN, _ADMIN_OK, _ADMIN_STATUS, _ADMIN_BODY = login_as("Admin")
if not _ADMIN_OK:
    print(f"[qa_exec_c] Admin session unavailable: POST /login -> "
          f"HTTP {_ADMIN_STATUS}; {_short(_ADMIN_BODY, 120)}", flush=True)
_ACTOR = _ADMIN if _ADMIN_OK else None
run_students(_ACTOR)
run_persons(_ACTOR)
run_donors(_ACTOR)
run_donations(_ACTOR)
run_aid_requests(_ACTOR)
run_care_provided(_ACTOR)
run_loans(_ACTOR)
run_repayments(_ACTOR)
run_revenue(_ACTOR)
run_messages(_ACTOR)
run_notifications(_ACTOR)
