#!/usr/bin/env python3
"""Install the pinned official Linux amd64 goose CLI with its release checksum."""
import hashlib
import os
from pathlib import Path
import platform
import urllib.request

VERSION = "v3.29.0"
ASSET = "goose_linux_x86_64"


def main():
    if platform.system() != "Linux" or platform.machine() not in ("x86_64", "amd64"):
        raise SystemExit("This installer currently supports Linux amd64 only.")
    root = Path(__file__).resolve().parent.parent
    directory = root / ".local" / "bin"
    if (root / ".local").is_symlink() or directory.is_symlink():
        raise SystemExit("Refusing a symlink for the local installation directory.")
    directory.mkdir(parents=True, exist_ok=True)
    base = f"https://github.com/pressly/goose/releases/download/{VERSION}/"
    with urllib.request.urlopen(base + "checksums.txt", timeout=60) as response:
        checksums = response.read().decode()
    expected = next(line.split()[0] for line in checksums.splitlines() if line.split()[-1] == ASSET)
    target = directory / "goose"
    if target.exists() or target.is_symlink():
        if target.is_symlink() or hashlib.sha256(target.read_bytes()).hexdigest() != expected:
            raise SystemExit("Existing goose differs from the pinned release; it was not overwritten.")
        print(f"Existing goose {VERSION} verified and preserved.")
        return
    with urllib.request.urlopen(base + ASSET, timeout=60) as response:
        binary = response.read()
    if hashlib.sha256(binary).hexdigest() != expected:
        raise SystemExit("goose checksum mismatch; nothing installed.")
    fd = os.open(target, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o700)
    with os.fdopen(fd, "wb") as file:
        file.write(binary)
    print(f"goose {VERSION} installed; official release SHA-256 verified.")


if __name__ == "__main__":
    main()
