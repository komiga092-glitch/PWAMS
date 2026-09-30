"""Ad-hoc DB probe used while authoring the QA executors (read-only)."""
import psycopg2

conn = psycopg2.connect(host="localhost", port=5432, user="pwams_user",
                        password="Pwams@2026Secure", dbname="pwams_db",
                        connect_timeout=5)
cur = conn.cursor()
cur.execute("select table_name from information_schema.tables "
            "where table_schema='public' order by 1")
print("TABLES:", [r[0] for r in cur.fetchall()])
cur.execute("select column_name, data_type from information_schema.columns "
            "where table_name='sessions' order by ordinal_position")
print("SESSIONS:", cur.fetchall())
cur.execute("select count(1) from users")
print("USERS:", cur.fetchone())
cur.execute("select username, status from users order by username")
print("ACCOUNTS:", cur.fetchall())
cur.execute("select count(1) from audit_logs")
print("AUDIT:", cur.fetchone())
cur.execute("select count(1) from file_uploads")
print("FILES:", cur.fetchone())
cur.execute("select count(1) from organizations")
print("ORGS:", cur.fetchone())
cur.execute("select count(1) from users where tenant_id is not null")
print("TENANT USERS:", cur.fetchone())
cur.execute("select password_hash from users where username='qa_admin'")
row = cur.fetchone()
print("PW HASH:", (row[0][:20] + "...") if row else None)
cur.execute("select count(1) from persons")
print("PERSONS:", cur.fetchone())
cur.execute("select count(1) from loans")
print("LOANS:", cur.fetchone())
cur.execute("select count(1) from aid_requests")
print("AID:", cur.fetchone())
conn.close()
