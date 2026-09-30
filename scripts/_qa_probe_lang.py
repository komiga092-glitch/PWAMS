"""_qa_probe_lang.py - does ?lang=si/ta actually change rendered labels?"""
import importlib.util

spec = importlib.util.spec_from_file_location("qrt", "scripts/qa_run_tests.py")
m = importlib.util.module_from_spec(spec)
spec.loader.exec_module(m)


def si_count(text):
    return sum(1 for ch in text if "\u0d80" <= ch <= "\u0dff")


def ta_count(text):
    return sum(1 for ch in text if "\u0b80" <= ch <= "\u0bff")


anon = m.Client("probe_lang_anon")
admin = m.Client("probe_lang_admin")
admin.login("qa_admin")

for path in ("/login", "/reports/donations/page", "/dashboard",
             "/students/page", "/reports/page"):
    client = anon if path == "/login" else admin
    for lang in ("en", "si", "ta"):
        s, b = client.request(
            "GET", f"{path}{'&' if '?' in path else '?'}lang={lang}")
        print(f"{path}?lang={lang}: HTTP {s} len={len(b)} si={si_count(b)} "
              f"ta={ta_count(b)}")
    print("-" * 60)

# persistence: cookie was set by the last ?lang=ta request
c = admin
s, b = c.request("GET", "/reports/donations/page")
print(f"no-lang follow-up: HTTP {s} si={si_count(b)} ta={ta_count(b)}")
print("jar:", [ln for ln in c.jar.read_text(errors="ignore").splitlines()
               if "pwams_lang" in ln])

# raw key / unprocessed action leaks?
for token in ("reports.total_donations", "{{", "{% raw"):
    print(f"leak {token!r} in report page:", token in b)
