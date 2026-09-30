"""_qa_restart_probe.py - isolate qa_run_tests.restart_server() behaviour.

Usage:  python -u scripts/_qa_restart_probe.py
Prints the health status of the instance before/after a server recycle so a
harness problem can be told apart from a test-case problem.
"""
import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))

import qa_run_tests as q  # noqa: E402


def health():
    c = q.Client("probe_" + str(os.getpid()) if False else "probe_one")
    s, _b = c._send("GET", "/health", None, None, None)
    return s


import os  # noqa: E402

print("python pid:", os.getpid(), flush=True)
print("health before:", health(), flush=True)
q.restart_server()
print("health after :", health(), flush=True)
print("PROBE OK", flush=True)
