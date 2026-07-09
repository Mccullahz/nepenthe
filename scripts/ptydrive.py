#!/usr/bin/env python3
"""Drive a TUI in a PTY, answering terminal queries like a real emulator.

usage: ptydrive.py <logfile> <keyscript> -- <cmd> [args...]
keyscript: lines of "<delay_seconds> <keys>" where keys may contain
\\r, \\e (escape), \\x?? escapes. Delay is relative to previous line.
"""
import os, pty, re, select, struct, sys, termios, time, fcntl, signal

log_path, keys_path = sys.argv[1], sys.argv[2]
assert sys.argv[3] == "--"
cmd = sys.argv[4:]

schedule = []
with open(keys_path) as f:
    for line in f:
        line = line.rstrip("\n")
        if not line or line.startswith("#"):
            continue
        delay, keys = line.split(" ", 1)
        keys = keys.replace("\\r", "\r").replace("\\e", "\x1b")
        keys = re.sub(r"\\x([0-9a-fA-F]{2})", lambda m: chr(int(m.group(1), 16)), keys)
        schedule.append((float(delay), keys.encode()))

pid, master = pty.fork()
if pid == 0:
    os.environ["TERM"] = "xterm-256color"
    os.execvp(cmd[0], cmd)

fcntl.ioctl(master, termios.TIOCSWINSZ, struct.pack("HHHH", 40, 140, 0, 0))

log = open(log_path, "wb")
buf = b""
next_key_at = time.monotonic() + schedule[0][0] if schedule else None
deadline = time.monotonic() + 60

QUERIES = [
    (re.compile(rb"\x1b\]11;\?(\x07|\x1b\\\\)"), b"\x1b]11;rgb:1e1e/1e1e/2e2e\x07"),
    (re.compile(rb"\x1b\]10;\?(\x07|\x1b\\\\)"), b"\x1b]10;rgb:cccc/cccc/cccc\x07"),
    (re.compile(rb"\x1b\[0?c"), b"\x1b[?62;22c"),
    (re.compile(rb"\x1b\[6n"), b"\x1b[1;1R"),
]

exited = False
while time.monotonic() < deadline:
    if next_key_at is not None and time.monotonic() >= next_key_at:
        _, keys = schedule.pop(0)
        try:
            os.write(master, keys)
        except OSError:
            break
        next_key_at = time.monotonic() + schedule[0][0] if schedule else None

    r, _, _ = select.select([master], [], [], 0.05)
    if master in r:
        try:
            data = os.read(master, 65536)
        except OSError:
            exited = True
            break
        if not data:
            exited = True
            break
        log.write(data)
        buf = (buf + data)[-4096:]
        for pat, resp in QUERIES:
            if pat.search(buf):
                buf = pat.sub(b"", buf)
                try:
                    os.write(master, resp)
                except OSError:
                    pass

    done, status = os.waitpid(pid, os.WNOHANG)
    if done:
        exited = True
        break

log.close()
if not exited:
    os.kill(pid, signal.SIGKILL)
    os.waitpid(pid, 0)
    print("TIMEOUT: killed", file=sys.stderr)
    sys.exit(2)
print("clean exit")
