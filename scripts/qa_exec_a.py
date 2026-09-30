# qa_exec_a.py -- helpers + Auth-Login + Account-Activation + User-Management
# Executed via exec() inside qa_run_tests.py namespace (shares all globals).

# override: donor create requires NIC for Individual type
def ensure_donor(admin):
    if "donor" in FIX:
        return FIX["donor"]
    name = f"QA Fixture Donor {int(time.time()) % 1000000}"
    did, _, _ = _create_and_id(admin, "/donors", {
        "name": name, "donor_type": "Individual",
        "nic_passport": f"QFD{int(time.time()) % 100000000}",
        "phone": "+94770000001", "email": "qa_fixture_donor@example.com",
        "address": "QA fixture", "notes": "auto fixture",
    })
    FIX["donor"] = did
    FIX["donor_name"] = name
    return did


def fresh_donor(admin, tag):
    name = f"QA Probe Donor {tag} {int(time.time()) % 1000000}"
    did, _, _ = _create_and_id(admin, "/donors", {
        "name": name, "donor_type": "Individual",
        "nic_passport": f"QPD{int(time.time()) % 100000000}",
        "phone": "+94770000003", "notes": f"probe {tag}",
    })
    return did, name


# override: pagination uses total_items (buildPagination), not total
def _list_total(body, items):
    try:
        d = json.loads(body)
    except Exception:
        return len(items)
    if isinstance(d, dict):
        for k in ("total", "count", "total_count", "total_items"):
            if isinstance(d.get(k), int):
                return d[k]
        pg = d.get("pagination")
        if isinstance(pg, dict):
            for k in ("total_items", "total", "count"):
                if isinstance(pg.get(k), int):
                    return pg[k]
    return len(items)


def _nums(o):
    """All numeric values incl. quoted decimals (shopspring marshals strings)."""
    out = []
    if isinstance(o, bool):
        return out
    if isinstance(o, (int, float)):
        out.append(float(o))
    elif isinstance(o, str):
        try:
            out.append(float(o))
        except ValueError:
            pass
    elif isinstance(o, dict):
        for v in o.values():
            out += _nums(v)
    elif isinstance(o, list):
        for v in o:
            out += _nums(v)
    return out


def _jget(body, *keys):
    try:
        d = json.loads(body)
    except Exception:
        return None
    for k in keys:
        if isinstance(d, dict) and k in d:
            return d[k]
    return None


def _contains(body, *needles):
    low = body.lower()
    return any(n.lower() in low for n in needles)


def _ok_login(status, body):
    return status in (200, 303) and (status == 303 or json_ok(body))


def _lang_counts(body):
    si = sum(1 for ch in body if "\u0d80" <= ch <= "\u0dff")
    ta = sum(1 for ch in body if "\u0b80" <= ch <= "\u0bff")
    return si, ta


def _i18n_key_leaks(body):
    return sorted(set(re.findall(r"\b(?:auth|nav|common|dashboard|actions)\.[a-z_]{3,}\b", body)))


def auth_tests(tcid, admin):
    n = int(tcid.split("-")[1])

    if n == 1:
        c = _fresh("auth001")
        s, b = c.request("GET", "/login")
        has_pw = 'type="password"' in b
        has_user = ('name="login"' in b) or ("username" in b.lower())
        ok = s == 200 and has_pw and has_user
        return (
            "Pass" if ok else "Fail",
            f"GET /login -> {fmt_status(s)}; password field={has_pw}, login field={has_user}.",
            "",
        )

    if n == 2:
        c = _fresh("auth002")
        s, b = c.login("qa_admin")
        s2, b2 = c.request("GET", "/auth/me")
        ok = _ok_login(s, b) and s2 == 200 and "qa_admin" in b2
        return (
            "Pass" if ok else "Fail",
            f"POST /login -> {fmt_status(s)}; GET /auth/me -> {fmt_status(s2)} "
            f"(username present={'qa_admin' in b2}).",
            "",
        )

    if n == 3:
        c = _fresh("auth003")
        s, b = c.request("POST", "/login",
                         json_body={"login": "qa_admin", "password": "WrongPass!123"})
        s2, _b2 = c.request("GET", "/auth/me")
        generic = "Invalid username/email or password." in b
        leak = any(x in b.lower() for x in ("role", "disabled", "locked", "admin@"))
        ok = s in (200, 401) and generic and s2 in (401, 303) and not leak
        return (
            "Pass" if ok else "Fail",
            f"Wrong password -> {fmt_status(s)}; generic message={generic}; "
            f"no session (auth/me {fmt_status(s2)}); leak markers={leak}.",
            "",
        )

    if n == 4:
        c = _fresh("auth004")
        s, b = c.request("POST", "/login",
                         json_body={"login": "ghost_user_qa_xyz", "password": "Whatever1!"})
        generic = "Invalid username/email or password." in b
        ok = s in (200, 401) and generic
        return (
            "Pass" if ok else "Fail",
            f"Non-existent username -> {fmt_status(s)}; same generic message={generic} "
            "(no user-enumeration signal).",
            "",
        )
    # <<A2>>
