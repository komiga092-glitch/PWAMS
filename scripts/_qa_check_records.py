"""_qa_check_records.py - list Test IDs recorded more than once in executors.

Two record() calls for one ID are normal when they are the if/else branches of
a single case (no OTP captured vs captured, etc.). What this check is really
for is spotting an OLD block that survived an edit and now overwrites the
newer evidence at runtime - so it prints every repetition for eyeballing.
"""
import collections
import pathlib
import re

ROOT = pathlib.Path(__file__).resolve().parent.parent
PATTERN = re.compile(r"record\(\s*[\"']([A-Z]+-\d{3})[\"']")

for name in ("qa_exec_a.py", "qa_exec_b.py", "qa_exec_c.py", "qa_exec_d.py"):
    src = (ROOT / "scripts" / name).read_text(encoding="utf-8-sig")
    ids = PATTERN.findall(src)
    dupes = {k: v for k, v in collections.Counter(ids).items() if v > 1}
    print(f"{name}: {len(ids)} record() calls, repeated ids={dupes or 'none'}")
