#!/usr/bin/env python3
"""Provision local DB roles once, keeping passwords out of SQL literals and arguments."""
from pathlib import Path
import subprocess
import sys

ROOT = Path(__file__).resolve().parent.parent


def psql(arguments, content):
    result = subprocess.run(
        ["docker", "compose", "exec", "-T", "postgres", "psql", "-X", "-U", "postgres",
         "-d", "watchbrief", "-v", "ON_ERROR_STOP=1", *arguments],
        input=content, capture_output=True, text=True, cwd=ROOT,
    )
    # Do not print SQL/error details, which can contain password verifiers.
    if result.returncode:
        raise SystemExit("Local DB provisioning failed. Existing roles/data were not reset; inspect the provisioning stage before retrying.")


def main():
    root = ROOT
    subprocess.run([sys.executable, str(root / "scripts/prepare-postgres.py"), "--roles"], check=True)
    sql = (root / "backend/db/bootstrap-local-roles.sql").read_text()
    arguments = ["--single-transaction", "-q", "-c", sql]
    prompts = []
    for role, filename in [
        ("watchbrief_api", "watchbrief-api-password"),
        ("watchbrief_user_creator", "watchbrief-user-creator-password"),
        ("watchbrief_migrator", "watchbrief-migrator-password"),
        ("watchbrief_password_resetter", "watchbrief-password-resetter-password"),
    ]:
        password = (root / ".local" / filename).read_text().rstrip("\n")
        if not password or "\n" in password or "\r" in password:
            raise SystemExit("Invalid local DB password file; provisioning was not applied.")
        # psql's password prompt encrypts on the client, rather than placing the
        # plaintext password in an ALTER ROLE SQL literal. No TTY is allocated.
        arguments += ["-c", "\\password " + role]
        prompts += [password, password]
    psql(arguments, "\n".join(prompts) + "\n")
    print("Local roles, schema and separate passwords provisioned in one transaction; values not displayed.")


if __name__ == "__main__":
    main()
