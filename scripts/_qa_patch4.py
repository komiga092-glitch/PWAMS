from pathlib import Path
import ast

p = Path(r"e:\PWAMS\scripts\qa_run_tests.py")
s = p.read_text(encoding="utf-8-sig")

# 1) prime CSRF cookie in Client.__init__ (unsafe methods 403 without it)
old_init = '        self.csrf = ""\n        self._sync_csrf()\n\n    def _sync_csrf(self):'
new_init = ('        self.csrf = ""\n'
            '        # prime: GET /login issues the pwams_csrf cookie (double-submit pair)\n'
            '        self._send("GET", "/login", None, None, None)\n'
            '        self._sync_csrf()\n\n'
            '    def _sync_csrf(self):')
assert old_init in s, "init pattern missing"
s = s.replace(old_init, new_init, 1)

# 2) _create_and_id: accept any top-level uuid in the create response,
#    and broaden the needle used for the list fallback
old_c = '''    try:
        d = json.loads(b)
        cand = d.get("data") or d.get("record") or d
        if isinstance(cand, dict) and cand.get("id"):
            return str(cand["id"]), s, b
    except Exception:
        pass
    # fallback: search list for matching field value
    needle = payload.get(match_key) or payload.get("full_name") or payload.get("title")'''
new_c = '''    jid = _first_uuid(b)
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
              or payload.get("description") or payload.get("subject"))'''
assert old_c in s, "create-id pattern missing"
s = s.replace(old_c, new_c, 1)

ast.parse(s)
p.write_text(s, encoding="utf-8")
print("prime + create-id patched, syntax ok")
