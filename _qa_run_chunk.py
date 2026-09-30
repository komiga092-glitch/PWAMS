"""Run one QA harness chunk in a single, self-contained command.

Starts the freshly built bin/qa_server.exe (APP_PORT=8081, SMTP -> Mailpit),
waits for /health, then executes scripts/qa_run_tests.py with the requested
chunk (a|b|c|d|export). Running the server and the harness as children of this
process keeps both alive for the whole run — a server started from a separate
shell command is torn down as soon as that command returns.

Usage:  python _qa_run_chunk.py a
"""
import os
import subprocess
import sys
import time
import urllib.request
from pathlib import Path

REPO = Path(__file__).resolve().parent
BASE = "http://127.0.0.1:8081"


def kill_listener():
    subprocess.run(
        ["powershell", "-NoProfile", "-Command",
         "(Get-NetTCPConnection -State Listen -LocalPort 8081 "
         "-ErrorAction SilentlyContinue).OwningProcess | ForEach-Object "
         "{ Stop-Process -Id $_ -Force -ErrorAction SilentlyContinue }"],
        capture_output=True, timeout=60)
    time.sleep(1.5)


def start_server(tag):
    env = dict(os.environ)
    env.update({
        "APP_PORT": "8081",
        "SMTP_HOST": "127.0.0.1",
        "SMTP_PORT": "1025",
        "SMTP_USERNAME": "qa",
        "SMTP_PASSWORD": "qa",
        "SMTP_FROM": "noreply@pwams.local",
    })
    log = open(REPO / f"qa_server_{tag}.log", "ab")
    exe = REPO / "bin" / "qa_server.exe"
    if not exe.exists():
        exe = REPO / "qa_server.exe"
    proc = subprocess.Popen([str(exe)], cwd=str(REPO), env=env,
                            stdout=log, stderr=log)
    print(f"[driver] started {exe.name} pid={proc.pid}", flush=True)

    for attempt in range(90):
        if proc.poll() is not None:
            print(f"[driver] server exited early rc={proc.returncode}",
                  flush=True)
            return proc
        try:
            with urllib.request.urlopen(BASE + "/health", timeout=3) as resp:
                if resp.status == 200:
                    print(f"[driver] /health OK after {attempt + 1}s", flush=True)
                    return proc
        except Exception:
            pass
        time.sleep(1)

    print("[driver] WARNING: /health not OK after 90s", flush=True)
    return proc


def main():
    chunk = (sys.argv[1] if len(sys.argv) > 1 else "a").lower()
    kill_listener()
    start_server(chunk * 2 if chunk in ("a", "b", "c", "d") else chunk)

    harness = REPO / "scripts" / "qa_run_tests.py"
    env = dict(os.environ)
    env["PYTHONUNBUFFERED"] = "1"
    cmd = [sys.executable, str(harness)] + ([chunk] if chunk != "export" else [])
    print(f"[driver] running {' '.join(cmd)}", flush=True)
    rc = subprocess.call(cmd, cwd=str(REPO), env=env)
    print(f"[driver] harness exit code {rc}", flush=True)
    kill_listener()
    return rc


if __name__ == "__main__":
    raise SystemExit(main())
