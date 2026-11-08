#!/usr/bin/env python3
"""Install the pinned official Linux amd64 sqlc CLI after SHA-256 verification."""
import hashlib
import io
import os
from pathlib import Path
import platform
import tarfile
import urllib.request

VERSION = "1.31.1"
SHA256 = "497ae4fcdfa64c5b0c311ffe4c2bd991e43991e82e5367792ed78bc2dca27354"


def main():
    if platform.system() != "Linux" or platform.machine() not in ("x86_64", "amd64"):
        raise SystemExit("This installer currently supports Linux amd64 only.")
    root = Path(__file__).resolve().parent.parent
    directory = root / ".local/bin"
    if (root / ".local").is_symlink() or directory.is_symlink():
        raise SystemExit("Refusing a symlink for the local installation directory.")
    directory.mkdir(parents=True, exist_ok=True)
    url = f"https://github.com/sqlc-dev/sqlc/releases/download/v{VERSION}/sqlc_{VERSION}_linux_amd64.tar.gz"
    with urllib.request.urlopen(url, timeout=60) as response:
        archive = response.read()
    if hashlib.sha256(archive).hexdigest() != SHA256:
        raise SystemExit("sqlc checksum mismatch; nothing installed.")
    with tarfile.open(fileobj=io.BytesIO(archive), mode="r:gz") as tar:
        member = tar.getmember("sqlc")
        if not member.isfile():
            raise SystemExit("Invalid sqlc release archive.")
        binary = tar.extractfile(member).read()
    target = directory / "sqlc"
    if target.exists() or target.is_symlink():
        if target.is_symlink() or hashlib.sha256(target.read_bytes()).digest() != hashlib.sha256(binary).digest():
            raise SystemExit("Existing sqlc differs from the pinned release; it was not overwritten.")
        print(f"Existing sqlc v{VERSION} verified and preserved.")
        return
    fd = os.open(target, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o700)
    with os.fdopen(fd, "wb") as file:
        file.write(binary)
    print(f"sqlc v{VERSION} installed; official release SHA-256 verified.")


if __name__ == "__main__":
    main()
