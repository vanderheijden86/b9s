import collections
import json
import os
import socket
import subprocess
import threading
import time

listener = socket.socket()
listener.bind(('127.0.0.1', 0))
listener.listen()
listener.settimeout(0.2)
connections = []
stop = threading.Event()
def accept():
    while not stop.is_set():
        try:
            conn, _ = listener.accept()
            connections.append(conn)
        except socket.timeout:
            pass
thread = threading.Thread(target=accept)
thread.start()
port = listener.getsockname()[1]
env = dict(os.environ, B9S_TEST_DOLT_HOST='127.0.0.1', B9S_TEST_DOLT_PORT=str(port), B9S_TEST_DOLT_DSN=f'root@tcp(127.0.0.1:{port})/unused?timeout=10s&readTimeout=10s', B9S_TEST_DOLT_SCRATCH_ADDR=f'127.0.0.1:{port}', B9S_TEST_DOLT_USER='root', B9S_TEST_DOLT_SCRATCH_USER='root', B9S_TEST_DOLT_CATALOG_DB='unused', B9S_TEST_DOLT_OTHER_DB='unused', BEADS_DOLT_PASSWORD='')
command = ['timeout', '90s', 'go', 'test', './internal/datasource', '-run', 'DoltIntegration|^TestDoltReader_(NewAndClose|LoadIssues|GetHeadHash)$|^TestDoltWatcher_(NewAndStop|StartIsIdempotent|DetectsNoChangeOnPoll|SetOnChangeNotCalledOnQuiescentDB)$', '-count=1', '-timeout=30s', '-json']
start = time.monotonic()
try:
    result = subprocess.run(command, cwd='/home/omnigent/workspace/b9s', env=env, stdout=subprocess.PIPE, stderr=subprocess.STDOUT, timeout=95)
finally:
    stop.set()
    thread.join(timeout=1)
    listener.close()
    for conn in connections:
        conn.close()
elapsed = time.monotonic() - start
output = result.stdout.decode()
open('/tmp/bd-6ye3/dolt-suite-unavailable.jsonl', 'w').write(output)
events = [json.loads(line) for line in output.splitlines() if line.startswith('{')]
counts = collections.Counter(e['Action'] for e in events if e.get('Test') and e['Action'] in ('pass', 'fail', 'skip'))
report = {'command': command, 'environment': {'runtime': subprocess.check_output(['go', 'version'], text=True).strip(), 'endpoint': f'127.0.0.1:{port}', 'server': 'TCP accepts connections but withholds MySQL greeting', 'credentials': 'root, empty password'}, 'exit_code': result.returncode, 'elapsed_seconds': elapsed, 'counts': {'passed': counts['pass'], 'failed': counts['fail'], 'skipped': counts['skip'], 'interrupted': 0, 'unrun': 0}, 'diagnostics': sum('Dolt integration infrastructure unavailable' in e.get('Output','') for e in events), 'connections': len(connections)}
report['reproduced_expected_failure'] = result.returncode == 1 and counts['fail'] == 1 and counts['skip'] >= 51 and report['diagnostics'] == 1 and len(connections) == 1 and elapsed < 20
open('/tmp/bd-6ye3/dolt-suite-unavailable-report.json', 'w').write(json.dumps(report, indent=2)+'\n')
print(json.dumps(report, indent=2))
raise SystemExit(0 if report['reproduced_expected_failure'] else 1)
