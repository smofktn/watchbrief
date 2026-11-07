#!/usr/bin/env python3
"""Write one private local connection file using hidden input; no secret output."""
import argparse
import getpass
import json
import os
import warnings
from pathlib import Path

parser = argparse.ArgumentParser(description=__doc__)
parser.add_argument("--role", choices=["watchbrief_api", "watchbrief_user_admin"], required=True)
parser.add_argument("--file", type=Path, required=True)
parser.add_argument("--port", type=int, default=55432)
args = parser.parse_args()
if not 1 <= args.port <= 65535:
    parser.error("port must be 1-65535")
# Never overwrite an existing secret; protect the parent as well as the file.
args.file.parent.mkdir(mode=0o700, parents=True, exist_ok=True)
if args.file.parent.stat().st_mode & 0o077:
    parser.error("configuration directory must have mode 0700")
warnings.simplefilter("error", getpass.GetPassWarning)
try:
    password = getpass.getpass("DB role password: ")
except (getpass.GetPassWarning, EOFError):
    parser.error("a terminal with hidden input is required")
if not password:
    parser.error("password cannot be empty")
with os.fdopen(os.open(args.file, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600), "w") as target:
    json.dump(dict(host="127.0.0.1", port=args.port, database="watchbrief", user=args.role, password=password), target)
print("Private configuration created.")
