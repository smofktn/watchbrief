#!/usr/bin/env python3
"""Start the local DB, then supervise the existing API and Frontend Make targets."""
import argparse
import fcntl
import os
from pathlib import Path
import signal
import subprocess
import time

ROOT = Path(__file__).resolve().parent.parent


def signal_group(process, sig):
    # Only process groups created by this invocation are ever signalled.
    try:
        os.killpg(process.pid, sig)
    except ProcessLookupError:
        pass


def group_exists(process):
    process.poll()  # Reap the group leader as well as waiting for descendants.
    try:
        os.killpg(process.pid, 0)
        return True
    except ProcessLookupError:
        return False


def shutdown(processes):
    for process in processes:
        signal_group(process, signal.SIGTERM)
    deadline = time.monotonic() + 12
    while any(group_exists(process) for process in processes) and time.monotonic() < deadline:
        time.sleep(0.1)
    for process in processes:
        signal_group(process, signal.SIGKILL)
    for process in processes:
        process.wait()


def supervise(make):
    processes = []
    stopping = False

    def stop(_signum, _frame):
        nonlocal stopping
        stopping = True

    signal.signal(signal.SIGINT, stop)
    signal.signal(signal.SIGTERM, stop)

    def start(target):
        process = subprocess.Popen([make, '--no-print-directory', target], cwd=ROOT,
                                   stdin=subprocess.DEVNULL, start_new_session=True)
        processes.append(process)
        return process

    try:
        print('DBの起動完了を待っています。初期設定・migrationは実行しません。', flush=True)
        database = start('db-up')
        while database.poll() is None and not stopping:
            time.sleep(0.1)
        if stopping:
            return 0
        if database.returncode != 0:
            print('DBを起動できません。API・画面の起動を中止しました。', flush=True)
            return 1
        processes.remove(database)
        services = []
        for name, target in [('API', 'backend-dev'), ('画面', 'frontend-dev')]:
            if stopping:
                return 0
            services.append((name, start(target)))
        print('API・画面を起動しています。準備状況とURLは各起動ログで確認してください。', flush=True)
        print('Ctrl+CでAPI・画面を終了します。DBは残ります（停止は make db-stop）。', flush=True)
        while not stopping:
            for name, process in services:
                if process.poll() is not None:
                    print(f'{name}が終了しました。もう一方のアプリも終了します。', flush=True)
                    return 1
            time.sleep(0.1)
        return 0
    except OSError:
        print('起動コマンドを実行できません。Makeと開発環境の準備を確認してください。', flush=True)
        return 1
    finally:
        shutdown(processes)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--make', default=os.environ.get('WATCHBRIEF_DEV_MAKE', 'make'))
    args = parser.parse_args()
    local = ROOT / '.local'
    local.mkdir(mode=0o700, exist_ok=True)
    # Keep the file after exit: unlinking a flock file can allow two lock holders.
    with (local / 'dev.lock').open('a') as lock:
        try:
            fcntl.flock(lock, fcntl.LOCK_EX | fcntl.LOCK_NB)
        except BlockingIOError:
            print('このフォルダーでは既に make dev が実行中です。', flush=True)
            return 1
        return supervise(args.make)


if __name__ == '__main__':
    raise SystemExit(main())
