#!/usr/bin/env python3
"""Verify the real supervisor with disposable fake services; Docker/DB is not used."""
import json
import os
from pathlib import Path
import shutil
import signal
import socket
import subprocess
import sys
import tempfile
import time
import unittest

ROOT = Path(__file__).resolve().parent.parent
WORKER = '''import json,os,signal,socket,sys,time
from pathlib import Path
sock=socket.socket();sock.bind(('127.0.0.1',0));sock.listen()
Path(sys.argv[1]).write_text(json.dumps(dict(pid=os.getpid(),port=sock.getsockname()[1])))
while True: time.sleep(.1)
'''
FAKE_MAKE = '''import os,subprocess,sys,time
from pathlib import Path
root=Path.cwd();target=sys.argv[-1];mode=os.environ.get('VERIFY_DEV_MODE','normal')
with (root/'calls').open('a') as f: f.write(target+'\\n')
if target=='db-up' and mode!='db-wait': sys.exit(7 if mode=='db-fail' else 0)
child=subprocess.Popen([sys.executable,'-c',os.environ['VERIFY_DEV_WORKER'],str(root/(target+'.json'))])
if mode==target+'-fail':
    time.sleep(.5);sys.exit(7)
child.wait()
'''


def wait_until(condition, message, timeout=20):
    deadline = time.monotonic() + timeout
    while time.monotonic() < deadline:
        if condition():
            return
        time.sleep(0.05)
    raise AssertionError(message)


class DevLifecycle(unittest.TestCase):
    def setUp(self):
        self.directory = tempfile.TemporaryDirectory(prefix='watchbrief-dev-check-')
        self.root = Path(self.directory.name)
        (self.root / 'scripts').mkdir()
        shutil.copy(ROOT / 'scripts/dev.py', self.root / 'scripts/dev.py')
        self.make = self.root / 'fake-make'
        self.make.write_text('#!' + sys.executable + '\n' + FAKE_MAKE)
        self.make.chmod(0o700)
        self.processes = []

    def launch(self, mode='normal'):
        process = subprocess.Popen([sys.executable, str(self.root / 'scripts/dev.py'), '--make', str(self.make)],
                                   env={**os.environ, 'VERIFY_DEV_MODE': mode, 'VERIFY_DEV_WORKER': WORKER},
                                   stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL, start_new_session=True)
        self.processes.append(process)
        return process

    def ready(self, target):
        path = self.root / (target + '.json')
        wait_until(lambda: path.exists() and path.stat().st_size > 0, 'service did not start')
        return json.loads(path.read_text())

    def closed(self, record):
        with socket.socket() as sock:
            self.assertNotEqual(sock.connect_ex(('127.0.0.1', record['port'])), 0, 'descendant still listening')

    def calls(self):
        return (self.root / 'calls').read_text().splitlines()

    def tearDown(self):
        for process in self.processes:
            if process.poll() is None:
                process.send_signal(signal.SIGTERM)
            try:
                process.wait(timeout=20)
            except subprocess.TimeoutExpired:
                process.kill()
                process.wait()
        # Failure cleanup is also limited to workers belonging to this test fixture.
        for path in self.root.glob('*.json'):
            try:
                os.kill(json.loads(path.read_text())['pid'], signal.SIGKILL)
            except (ProcessLookupError, json.JSONDecodeError):
                pass
        self.directory.cleanup()

    def test_interrupt_stops_both_apps_and_their_descendants(self):
        process = self.launch()
        backend, frontend = self.ready('backend-dev'), self.ready('frontend-dev')
        self.assertEqual(self.calls()[0], 'db-up')
        self.assertEqual((self.root / '.local').stat().st_mode & 0o077, 0)
        self.assertEqual(set(self.calls()[1:]), {'backend-dev', 'frontend-dev'})
        process.send_signal(signal.SIGINT)
        self.assertEqual(process.wait(timeout=20), 0)
        self.closed(backend)
        self.closed(frontend)
        self.assertNotIn('db-stop', self.calls())
        self.assertNotIn('db-migrate', self.calls())

    def test_interrupt_while_waiting_for_db_starts_no_apps(self):
        process = self.launch('db-wait')
        database = self.ready('db-up')
        process.send_signal(signal.SIGINT)
        self.assertEqual(process.wait(timeout=20), 0)
        self.assertEqual(self.calls(), ['db-up'])
        self.closed(database)

    def test_db_failure_starts_no_apps(self):
        process = self.launch('db-fail')
        self.assertNotEqual(process.wait(timeout=20), 0)
        self.assertEqual(self.calls(), ['db-up'])

    def test_service_failure_stops_the_other_app_and_orphan(self):
        for target in ('backend-dev', 'frontend-dev'):
            with self.subTest(target=target):
                process = self.launch(target + '-fail')
                backend, frontend = self.ready('backend-dev'), self.ready('frontend-dev')
                self.assertNotEqual(process.wait(timeout=20), 0)
                self.closed(backend)
                self.closed(frontend)
                for path in self.root.glob('*.json'):
                    path.unlink()

    def test_second_supervisor_is_rejected_and_lock_can_be_reused(self):
        first = self.launch()
        self.ready('backend-dev')
        self.ready('frontend-dev')
        second = self.launch()
        self.assertNotEqual(second.wait(timeout=5), 0)
        self.assertEqual(first.poll(), None)
        self.assertEqual(len(self.calls()), 3)
        first.send_signal(signal.SIGTERM)
        self.assertEqual(first.wait(timeout=20), 0)
        # A completed invocation does not leave the lock held.
        third = self.launch('db-fail')
        self.assertNotEqual(third.wait(timeout=5), 0)
        self.assertEqual(self.calls()[-1], 'db-up')
        self.assertEqual(len(self.calls()), 4)


if __name__ == '__main__':
    unittest.main(verbosity=2)
