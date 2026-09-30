"""_qa_accounts.py - list seeded QA accounts (used before harness runs)."""
import sys

import psycopg2

conn = psycopg2.connect(host="localhost", port=5432, user="pwams_user",
                        password="Pwams@2026Secure", dbname="pwams_db",
                        connect_timeout=5)
cur = conn.cursor()
cur.execute("SELECT username, status FROM users WHERE username LIKE 'qa%' "
            "ORDER BY username")
rows = cur.fetchall()
print(f"qa-ish accounts: {len(rows)}")
for username, status in rows:
    print(f"  {username}: {status}")
cur.execute("SELECT count(*) FROM users")
print("total users:", cur.fetchone()[0])
conn.close()
sys.exit(0)
