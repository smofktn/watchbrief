#!/usr/bin/env python3
"""Verify local schema constraints and role permissions with rolled-back fake data."""
from pathlib import Path
import subprocess

ROOT = Path(__file__).resolve().parent.parent
USER = "01ARZ3NDEKTSV4RRFFQ69G5FAV"
OTHER = "01ARZ3NDEKTSV4RRFFQ69G5FAX"
SESSION = "01ARZ3NDEKTSV4RRFFQ69G5FAW"
NEW_SESSION = "01ARZ3NDEKTSV4RRFFQ69G5FAY"


def psql(sql):
    return subprocess.run(
        ["docker", "compose", "exec", "-T", "postgres", "psql", "-X", "-U", "postgres",
         "-d", "watchbrief", "-v", "ON_ERROR_STOP=1", "-v", "VERBOSITY=sqlstate", "-At"],
        input=sql, capture_output=True, text=True, cwd=ROOT,
    )


def main():
    # No real password hashes or tokens: synthetic rows stay inside each transaction.
    user_insert = f"INSERT INTO app.users(id,login_id,password_hash) VALUES ('{OTHER}','verification_other','verification-placeholder');"
    session_insert = f"INSERT INTO app.auth_sessions(id,user_id,token_hash) VALUES ('{NEW_SESSION}','{USER}',decode(repeat('02',32),'hex'));"
    seed = f"""BEGIN;
SET LOCAL ROLE watchbrief_migrator;
INSERT INTO app.users(id,login_id,password_hash) VALUES ('{USER}','verification_user','verification-placeholder');
INSERT INTO app.auth_sessions(id,user_id,token_hash) VALUES ('{SESSION}','{USER}',decode(repeat('01',32),'hex'));
"""
    cases = [
        ("API reads users", "watchbrief_api", "SELECT login_id FROM app.users;", None),
        ("API reads sessions", "watchbrief_api", "SELECT id FROM app.auth_sessions;", None),
        ("API inserts session", "watchbrief_api", session_insert, None),
        ("API cannot create user", "watchbrief_api", user_insert, "42501"),
        ("API cannot update user", "watchbrief_api", "UPDATE app.users SET login_id='changed';", "42501"),
        ("API cannot delete user", "watchbrief_api", "DELETE FROM app.users;", "42501"),
        ("API cannot update session", "watchbrief_api", "UPDATE app.auth_sessions SET token_hash=decode(repeat('03',32),'hex');", "42501"),
        ("API cannot delete session", "watchbrief_api", "DELETE FROM app.auth_sessions;", "42501"),
        ("API cannot drop table", "watchbrief_api", "DROP TABLE app.auth_sessions;", "42501"),
        ("API cannot read migration state", "watchbrief_api", "SELECT * FROM app.goose_db_version;", "42501"),
        ("Creator inserts user", "watchbrief_user_creator", user_insert, None),
        ("Creator cannot read users", "watchbrief_user_creator", "SELECT * FROM app.users;", "42501"),
        ("Creator cannot update users", "watchbrief_user_creator", "UPDATE app.users SET login_id='changed';", "42501"),
        ("Creator cannot delete users", "watchbrief_user_creator", "DELETE FROM app.users;", "42501"),
        ("Creator cannot read sessions", "watchbrief_user_creator", "SELECT * FROM app.auth_sessions;", "42501"),
        ("Creator cannot insert sessions", "watchbrief_user_creator", session_insert, "42501"),
        ("ULID format", "watchbrief_migrator", user_insert.replace(OTHER, "invalid"), "23514"),
        ("ULID overflow", "watchbrief_migrator", user_insert.replace(OTHER, "81ARZ3NDEKTSV4RRFFQ69G5FAX"), "23514"),
        ("Duplicate user ID", "watchbrief_migrator", user_insert.replace(OTHER, USER), "23505"),
        ("Duplicate login ID", "watchbrief_migrator", user_insert.replace("verification_other", "verification_user"), "23505"),
        ("Uppercase login ID", "watchbrief_migrator", user_insert.replace("verification_other", "Invalid"), "23514"),
        ("Oversized login ID", "watchbrief_migrator", user_insert.replace("verification_other", "a" * 65), "23514"),
        ("Empty password hash", "watchbrief_migrator", user_insert.replace("verification-placeholder", ""), "23514"),
        ("Missing password hash", "watchbrief_migrator", user_insert.replace("'verification-placeholder'", "NULL"), "23502"),
        ("Missing User FK", "watchbrief_migrator", session_insert.replace(USER, OTHER), "23503"),
        ("Short token hash", "watchbrief_migrator", session_insert.replace("'02',32", "'02',31"), "23514"),
        ("Duplicate token hash", "watchbrief_migrator", session_insert.replace("'02',32", "'01',32"), "23505"),
        ("Duplicate session ID", "watchbrief_migrator", session_insert.replace(NEW_SESSION, SESSION), "23505"),
        ("Referenced User deletion", "watchbrief_migrator", f"DELETE FROM app.users WHERE id='{USER}';", "23001"),
    ]
    for name, role, statement, expected in cases:
        result = psql(seed + f"SET LOCAL ROLE {role};\n" + statement + "\nROLLBACK;")
        if expected is None:
            passed = result.returncode == 0
        else:
            # ON_ERROR_STOP exits; disconnect rolls back the open transaction.
            passed = result.returncode != 0 and f"ERROR:  {expected}" in result.stderr
        if not passed:
            raise SystemExit(f"Schema verification failed: {name}. No response data printed.")
    print(f"Schema constraints and permissions: {len(cases)} checks passed; fixture transactions rolled back.")


if __name__ == "__main__":
    main()
