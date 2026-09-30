from pathlib import Path
import ast

p = Path(r"e:\PWAMS\scripts\qa_run_tests.py")
s = p.read_text(encoding="utf-8-sig")

old_dead = '        sa.request("POST", "/loans", json_body=None) if False else None\n'
if old_dead in s:
    s = s.replace(old_dead, "")

old_jar = '        self.jar = REPO / f"qa_jar_{self.name}.txt"'
new_jar = ('        _tmp = REPO / "qa_tmp"\n'
           '        _tmp.mkdir(exist_ok=True)\n'
           '        self.jar = _tmp / f"qa_jar_{self.name}.txt"')
assert old_jar in s, "jar pattern not found"
s = s.replace(old_jar, new_jar)

ast.parse(s)
p.write_text(s, encoding="utf-8")
print("patched + syntax ok")
