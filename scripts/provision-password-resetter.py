#!/usr/bin/env python3
"""Explicit existing-environment upgrade: add only the resetter role, never replace roles/passwords."""
import stat
from pathlib import Path
import runpy

ROOT = Path(__file__).resolve().parent.parent

def main():
    helper = runpy.run_path(str(ROOT / "scripts/prepare-postgres.py"))
    directory = ROOT / ".local"
    if directory.is_symlink() or not directory.is_dir():
        raise SystemExit("Prepare the local database first; unsafe/missing .local directory.")
    path = directory / "watchbrief-password-resetter-password"
    helper["prepare"](path)
    info = path.lstat()
    if not stat.S_ISREG(info.st_mode) or info.st_mode & 0o077 or info.st_size > 4096:
        raise SystemExit("Unsafe resetter credential file.")
    password = path.read_text().rstrip("\n")
    if not password or "\n" in password or "\r" in password:
        raise SystemExit("Invalid resetter credential file.")
    sql = """CREATE ROLE watchbrief_password_resetter LOGIN NOINHERIT NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS;
GRANT CONNECT ON DATABASE watchbrief TO watchbrief_password_resetter;
GRANT USAGE ON SCHEMA app TO watchbrief_password_resetter;
ALTER ROLE watchbrief_password_resetter IN DATABASE watchbrief SET search_path = app, pg_catalog;"""
    bootstrap = runpy.run_path(str(ROOT / "scripts/bootstrap-postgres-roles.py"))
    bootstrap["psql"](["--single-transaction", "-q", "-c", sql, "-c", "\\password watchbrief_password_resetter"], password + "\n" + password + "\n")
    print("Resetter role added; existing roles/passwords/data preserved. Run make db-migrate next.")

if __name__ == "__main__":
    main()
