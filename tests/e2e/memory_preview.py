#!/usr/bin/env python3
"""Read-only PTY checks against a real, isolated Memory preview (requires pyte)."""

import argparse
import codecs
import fcntl
import html
import json
import os
from pathlib import Path
import platform
import pty
import re
import select
import shlex
import signal
import struct
import subprocess
import termios
import time

import pyte


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--binary", required=True, type=Path)
    parser.add_argument("--workspace", required=True, type=Path)
    parser.add_argument("--evidence", required=True, type=Path)
    args = parser.parse_args()
    workspace = args.workspace.resolve()
    metadata = json.loads((workspace / ".beads/metadata.json").read_text())
    if not (metadata.get("graph_mode") == "link" and metadata.get("graph_ready")
            and metadata.get("dolt_mode") == "embedded"):
        parser.error("workspace must be an embedded, ready graph preview")
    preview_bin = workspace / ".memory-preview/bin"
    if not (preview_bin / "bd").is_file():
        parser.error("workspace has no pinned preview bd")
    evidence = args.evidence.resolve()
    evidence.mkdir(parents=True, exist_ok=True)
    env = {
        "HOME": str(Path.home()), "TMPDIR": "/tmp", "TERM": "screen-256color",
        "PATH": f"{preview_bin}:/opt/homebrew/bin:/usr/bin:/bin",
        "BEADS_DIR": str(workspace / ".beads"), "BEADS_DOLT_AUTO_START": "0",
        "B9S_TEST_MODE": "1", "B9S_NO_BROWSER": "1", "B9S_TUI_AUTOCLOSE_MS": "180000",
        "XDG_CONFIG_HOME": str(evidence / "config"),
    }
    command = "stty cols 140 rows 42; exec " + shlex.join([
        str(args.binary.resolve()), "memories", "--project", str(workspace)])
    if platform.system() == "Darwin":
        argv = ["script", "-q", "/dev/null", "sh", "-c", command]
    else:
        argv = ["script", "-q", "-c", command, "/dev/null"]
    master, slave = pty.openpty()
    fcntl.ioctl(slave, termios.TIOCSWINSZ, struct.pack("HHHH", 42, 140, 0, 0))
    process = subprocess.Popen(argv, stdin=slave, stdout=slave, stderr=slave,
                               cwd=workspace, env=env, start_new_session=True)
    os.close(slave)
    screen = pyte.Screen(140, 42)
    stream = pyte.Stream(screen)
    decoder = codecs.getincrementaldecoder("utf-8")("replace")
    raw = bytearray()
    passed, failed = 0, 0

    def text():
        return "\n".join(screen.display)

    def pump(duration=0.1):
        if select.select([master], [], [], duration)[0]:
            data = os.read(master, 65536)
            raw.extend(data)
            stream.feed(decoder.decode(data))

    def wait_for(predicate, seconds=15):
        deadline = time.monotonic() + seconds
        while time.monotonic() < deadline:
            if predicate(text()):
                return
            if process.poll() is not None:
                raise AssertionError(f"TUI exited with {process.returncode}")
            pump()
        raise AssertionError("screen condition timed out\n" + text())

    def capture(name):
        pump(0.2)
        (evidence / f"{name}.txt").write_text(text())
        rows = []
        backgrounds = []
        def color(value, default):
            if value == "default":
                return default
            if len(value) == 6 and all(c in "0123456789abcdef" for c in value):
                return "#" + value
            return value
        for y in range(screen.lines):
            spans = []
            for x in range(screen.columns):
                char = screen.buffer[y][x]
                fg, bg = color(char.fg, "#d4d4d4"), color(char.bg, "#151515")
                if char.reverse:
                    fg, bg = bg, fg
                if bg != "#151515":
                    backgrounds.append(f'<rect x="{12 + x * 8.5}" y="{9 + y * 16}" width="8.5" height="16" fill="{bg}"/>')
                weight = "bold" if char.bold else "normal"
                decoration = "underline" if char.underscore else "none"
                spans.append(f'<tspan x="{12 + x * 8.5}" fill="{fg}" font-weight="{weight}" text-decoration="{decoration}">{html.escape(char.data)}</tspan>')
            rows.append(f'<text y="{22 + y * 16}">' + "".join(spans) + "</text>")
        svg = '<svg xmlns="http://www.w3.org/2000/svg" width="1214" height="692"><rect width="100%" height="100%" fill="#151515"/>' + "".join(backgrounds) + '<g font-family="monospace" font-size="14">' + "".join(rows) + '</g></svg>'
        (evidence / f"{name}.svg").write_text(svg)

    def check(name, keys, predicate, seconds=15):
        nonlocal passed
        for key in keys:
            os.write(master, key.encode())
            deadline = time.monotonic() + 0.15
            while time.monotonic() < deadline:
                pump(0.05)
        wait_for(predicate, seconds)
        capture(name)
        passed += 1
        print(f"PASS {name}", flush=True)

    try:
        check("list", "", lambda s: "loading Memories" not in s and "current version" in s and "adr-" in s)
        check("matrix", "3", lambda s: "Work · Issues" in s and "legend" in s, 90)
        old_cell = next(line for line in text().splitlines() if "cell  " in line)
        check("matrix-row", "j", lambda s: old_cell not in s)
        old_cell = next(line for line in text().splitlines() if "cell  " in line)
        check("matrix-column", "l", lambda s: old_cell not in s)
        check("matrix-sort", "s", lambda s: "by use" in s)
        check("matrix-problems", "p", lambda s: "problems only" in s)
        check("matrix-reset", "ps", lambda s: "by status" in s and "problems only" not in s)
        check("focus", "2", lambda s: "Knowledge · Memories" in s)
        check("chips", "4", lambda s: "filled = follows" in s)
        check("constellation", "5", lambda s: "Graph health" in s)
        check("matrix-return", "3", lambda s: "Work · Issues" in s and "cell  " in s)
        cell = next(line for line in text().splitlines() if "cell  " in line)
        match = re.search(r"ADR (\d{4})", cell)
        if not match:
            raise AssertionError("selected cell has no ADR number: " + cell)
        selected_id = "adr-" + match[1]
        check("open-decision", "\r", lambda s: selected_id + " · current version" in s and "Source:" in s and "Reading " not in s)
        os.write(master, b"q")
        deadline = time.monotonic() + 5
        while process.poll() is None and time.monotonic() < deadline:
            pump()
        if process.poll() is None:
            raise AssertionError("q did not exit within five seconds")
        if process.returncode:
            raise AssertionError(f"TUI exit status {process.returncode}")
        passed += 1
        print("PASS quit", flush=True)
    except (AssertionError, OSError) as err:
        failed += 1
        print(f"FAIL {err}", flush=True)
        capture("failure")
    finally:
        if process.poll() is None:
            os.killpg(process.pid, signal.SIGTERM)
            try:
                process.wait(timeout=3)
            except subprocess.TimeoutExpired:
                os.killpg(process.pid, signal.SIGKILL)
                process.wait(timeout=3)
        os.close(master)
        (evidence / "terminal.raw").write_bytes(raw)
        print(f"=== MEMORY_PREVIEW DONE pass={passed} fail={failed} ===", flush=True)
    return int(failed != 0)


if __name__ == "__main__":
    raise SystemExit(main())
