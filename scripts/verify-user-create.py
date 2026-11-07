#!/usr/bin/env python3
"""Exercise the local Linux User creation CLI through a PTY using private fake credentials."""
import ctypes
import ctypes.util
import errno
import fcntl
import json
import os
from pathlib import Path
import pty
import secrets
import select
import subprocess
import termios
import time
import unicodedata

ROOT = Path(__file__).resolve().parent.parent
BINARY = ROOT / ".local/bin/user-create"


def sql(statement):
    result = subprocess.run(
        ["docker", "compose", "exec", "-T", "postgres", "psql", "-X", "-U", "postgres",
         "-d", "watchbrief", "-v", "ON_ERROR_STOP=1", "-Atc", statement],
        capture_output=True, text=True, cwd=ROOT,
    )
    if result.returncode:
        raise SystemExit("DB verification failed; details were not printed.")
    return result.stdout.strip()


def run_terminal(login_id, first, second, interrupt=False):
    master, slave = pty.openpty()
    original = termios.tcgetattr(slave)
    output = bytearray()

    def controlling_terminal():
        os.setsid()
        fcntl.ioctl(0, termios.TIOCSCTTY, 0)

    process = subprocess.Popen(
        [str(BINARY), "--login-id", login_id],
        stdin=slave, stdout=slave, stderr=slave, cwd=ROOT / "backend",
        preexec_fn=controlling_terminal,
    )

    def receive_until(marker):
        deadline = time.monotonic() + 10
        while marker not in output:
            if time.monotonic() > deadline or process.poll() is not None:
                raise AssertionError("Terminal prompt did not arrive.")
            if select.select([master], [], [], 0.1)[0]:
                output.extend(os.read(master, 65536))

    try:
        receive_until(b"Password (")
        assert not termios.tcgetattr(slave)[3] & termios.ECHO, "Password echo must be disabled"
        if interrupt:
            os.write(master, b"\x03")
        else:
            os.write(master, first.encode() + b"\n")
            receive_until(b"Confirm password:")
            assert not termios.tcgetattr(slave)[3] & termios.ECHO, "Confirmation echo must be disabled"
            os.write(master, second.encode() + b"\n")
        deadline = time.monotonic() + 20
        while process.poll() is None:
            if time.monotonic() > deadline:
                raise AssertionError("CLI did not finish.")
            if select.select([master], [], [], 0.1)[0]:
                output.extend(os.read(master, 65536))
        while select.select([master], [], [], 0.05)[0]:
            try:
                chunk = os.read(master, 65536)
            except OSError as exc:
                if exc.errno != errno.EIO:
                    raise
                break
            if not chunk:
                break
            output.extend(chunk)
        assert termios.tcgetattr(slave)[3] & termios.ECHO, "Terminal echo was not restored"
        for value in (first, second):
            if value:
                assert value.encode() not in output, "A supplied secret appeared in output"
        return process.returncode, bytes(output).decode(errors="replace")
    finally:
        if process.poll() is None:
            process.kill()
        process.wait()
        termios.tcsetattr(slave, termios.TCSANOW, original)
        os.close(master)
        os.close(slave)


def main():
    path = ROOT / ".local/user-create-verification.json"
    if path.exists() or path.is_symlink():
        info = path.lstat()
        assert not path.is_symlink() and info.st_mode & 0o077 == 0, "Unsafe verification file"
        credentials = json.loads(path.read_text())
    else:
        credentials = {"login_id": "verify_cli_" + secrets.token_hex(4),
                       "password": "が" * 15 + "-" + secrets.token_urlsafe(12)}
        fd = os.open(path, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600)
        with os.fdopen(fd, "w") as file:
            json.dump(credentials, file, ensure_ascii=False)
    login_id, password = credentials["login_id"], credentials["password"]
    assert login_id.startswith("verify_cli_") and all(c in "abcdefghijklmnopqrstuvwxyz0123456789_" for c in login_id)

    def record():
        text = sql(f"SELECT json_build_object('id',id,'hash',password_hash)::text FROM app.users WHERE login_id='{login_id}';")
        return json.loads(text) if text else None

    for suffix, first, second, interrupt in [
        ("_mismatch", password, password + "Z", False),
        ("_short", "か\u3099" * 14, "か\u3099" * 14, False),
        ("_cancel", "", "", True),
    ]:
        code, _ = run_terminal(login_id + suffix, first, second, interrupt)
        assert code != 0
        assert sql(f"SELECT count(*) FROM app.users WHERE login_id='{login_id + suffix}';") == "0"
    print("Hidden input, mismatch, NFC length rejection and Ctrl+C restoration: passed")

    result = subprocess.run([str(BINARY), "--login-id", login_id + "_notty"], cwd=ROOT / "backend",
                            start_new_session=True, stdin=subprocess.DEVNULL, capture_output=True)
    assert result.returncode != 0
    assert sql(f"SELECT count(*) FROM app.users WHERE login_id='{login_id}_notty';") == "0"
    print("No controlling terminal: rejected without User creation")

    before = record()
    if before is None:
        code, _ = run_terminal(login_id.upper(), unicodedata.normalize("NFD", password), password)
        assert code == 0, "NFC equivalent confirmation should create a User"
    saved = record()
    assert saved is not None and len(saved["id"]) == 26 and len(saved["hash"]) == 97
    assert password not in saved["hash"]
    lib = ctypes.CDLL(ctypes.util.find_library("argon2"))
    verify = lib.argon2id_verify
    verify.argtypes = [ctypes.c_char_p, ctypes.c_void_p, ctypes.c_size_t]
    verify.restype = ctypes.c_int
    raw = unicodedata.normalize("NFC", password).encode()
    assert verify(saved["hash"].encode(), raw, len(raw)) == 0, "Stored hash did not verify independently"
    code, message = run_terminal(login_id, password + "Z", password + "Z")
    assert code != 0 and "同じログインID" in message and record() == saved
    print("NFC creation, lowercase ID, independent stored-hash verification and duplicate preservation: passed")
    print("One fictional User retained; private verification credentials preserved under .local (values not displayed).")


if __name__ == "__main__":
    main()
