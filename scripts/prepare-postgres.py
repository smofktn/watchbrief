#!/usr/bin/env python3
"""Prepare a local-only bootstrap credential without printing or overwriting it."""
import os
import argparse
from pathlib import Path
import secrets
import stat


def prepare(path):
    try:
        fd = os.open(path, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600)
    except FileExistsError:
        info = path.lstat()
        if not stat.S_ISREG(info.st_mode) or info.st_mode & 0o077 or info.st_size == 0:
            raise SystemExit("Existing credential must be a nonempty regular file with mode 600")
        print(f"Existing local credential preserved: {path.name}")
        return
    with os.fdopen(fd, "w") as file:
        file.write(secrets.token_urlsafe(32) + "\n")
    print(f"Local credential created: {path.name}; value not displayed.")


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--roles", action="store_true", help="Also prepare separate DB role passwords")
    args = parser.parse_args()
    directory = Path(__file__).resolve().parent.parent / ".local"
    if directory.is_symlink():
        raise SystemExit("Refusing a symlink for .local")
    directory.mkdir(mode=0o700, exist_ok=True)
    names = ["postgres-admin-password"]
    if args.roles:
        names += ["watchbrief-api-password", "watchbrief-user-creator-password", "watchbrief-migrator-password", "watchbrief-password-resetter-password"]
    for name in names:
        prepare(directory / name)


if __name__ == "__main__":
    main()
