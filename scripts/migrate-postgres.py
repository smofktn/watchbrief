#!/usr/bin/env python3
"""Run local goose with the migration role; only nondestructive commands are exposed."""
import argparse
import fcntl
import os
from pathlib import Path
import stat
import subprocess
import tempfile


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("command", choices=["up", "status", "version"])
    args = parser.parse_args()
    root = Path(__file__).resolve().parent.parent
    with (root / ".local/migration.lock").open("a") as lock:
        try:
            fcntl.flock(lock, fcntl.LOCK_EX | fcntl.LOCK_NB)
        except BlockingIOError:
            raise SystemExit("Another local migration command is running.")
        run_migration(root, args.command)


def run_migration(root, command):
    port = os.environ.get("DB_PORT") or "5432"
    try:
        valid_port = port.isascii() and port.isdecimal() and 1 <= int(port) <= 65535
    except ValueError:
        valid_port = False
    if not valid_port:
        raise SystemExit("DB_PORT must be 1-65535.")
    port = str(int(port))
    binary = root / ".local/bin/goose"
    if not binary.is_file():
        raise SystemExit("Run make goose-install first.")
    version = subprocess.run([str(binary), "-version"], capture_output=True, text=True)
    if version.returncode or "v3.29.0" not in version.stdout + version.stderr:
        raise SystemExit("Expected goose v3.29.0; migration not started.")
    secret = root / ".local/watchbrief-migrator-password"
    info = secret.lstat()
    if not stat.S_ISREG(info.st_mode) or info.st_mode & 0o077:
        raise SystemExit("Migration password must be a regular file with mode 600.")
    password = secret.read_text().rstrip("\n")
    if not password or "\n" in password or "\r" in password:
        raise SystemExit("Invalid migration password file.")
    environment = dict(os.environ)
    for name in list(environment):
        if name.startswith("PG") or name.startswith("GOOSE_"):
            del environment[name]
    with tempfile.NamedTemporaryFile(mode="w", dir=root / ".local", prefix="pgpass-") as passfile:
        escaped = password.replace("\\", "\\\\").replace(":", "\\:")
        passfile.write(f"127.0.0.1:{port}:watchbrief:watchbrief_migrator:{escaped}\n")
        passfile.flush()
        environment.update({
            "PGPASSFILE": passfile.name,
            "GOOSE_DRIVER": "postgres",
            "GOOSE_DBSTRING": f"host=127.0.0.1 port={port} dbname=watchbrief user=watchbrief_migrator sslmode=disable connect_timeout=5",
        })
        result = subprocess.run(
            [str(binary), "-env=none", "-dir", str(root / "backend/db/goose"), "-table", "app.goose_db_version", "-timeout", "30s", command],
            env=environment, cwd=root, capture_output=True, text=True,
        )
    if result.returncode:
        # Driver errors can contain connection data; do not echo captured output.
        raise SystemExit("Migration failed. Existing data was not reset; inspect the migration stage before retrying.")
    print(result.stdout, end="")
    print(result.stderr, end="")


if __name__ == "__main__":
    main()
