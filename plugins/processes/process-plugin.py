#!/usr/bin/env python3
"""keyshell process plugin.

Reads one JSON request per line on stdin:

    {"query": "fire", "limit": 40}

and answers with one JSON item per line followed by the response terminator
``{"done": true}``. The process stays alive for the next request until stdin
closes.

Matching is deliberately strict so the plugin stays quiet when it answers the
merged default query: a query matches when it is a case-insensitive prefix of
the process name, or when it is exactly the pid. An empty query returns nothing
in either mode, because terminating a process is destructive: you have to name a
process or give a pid before anything is actionable.

The request carries a ``merged`` flag. This plugin does not need it, since it
behaves the same in both modes; it is there for plugins that should list entries
when opened but stay silent in the default query.
"""

import json
import os
import subprocess
import sys

PS_FIELDS = "pid=,pcpu=,pmem=,comm="
DEFAULT_LIMIT = 40
MAX_LIMIT = 200


def read_processes():
    """Return (pid, name, cpu, mem) sorted by CPU descending."""
    try:
        result = subprocess.run(
            ["ps", "-eo", PS_FIELDS],
            capture_output=True,
            text=True,
            timeout=5,
            check=False,
        )
    except (OSError, subprocess.SubprocessError):
        return []

    rows = []
    for line in result.stdout.splitlines():
        parts = line.split(None, 3)
        if len(parts) < 4:
            continue

        pid_text, cpu_text, mem_text, name = parts
        name = name.strip()
        if not pid_text.isdigit():
            continue
        if name.startswith("[") and name.endswith("]"):
            continue

        pid = int(pid_text)
        if pid == os.getpid():
            continue

        try:
            cpu = float(cpu_text)
            mem = float(mem_text)
        except ValueError:
            continue

        rows.append((pid, name, cpu, mem))

    rows.sort(key=lambda row: (-row[2], row[0]))
    return rows


def matches(query, name, pid):
    if query == "":
        return False
    lowered = query.lower()
    if lowered.isdigit():
        return pid == int(lowered)
    return name.lower().startswith(lowered)


def build_item(pid, name, cpu, mem):
    badge = "{:.0f}%".format(cpu) if cpu >= 1.0 else ""
    return {
        "id": str(pid),
        "title": name,
        "subtitle": "PID {} // CPU {:.1f}% // MEM {:.1f}%".format(pid, cpu, mem),
        "glyph": "⚙",
        "badge": badge,
        "destructive": True,
        "actions": [
            {
                "type": "exec",
                "argv": ["/bin/sh", "-c", "kill -TERM {}".format(pid)],
            }
        ],
    }


def write(payload):
    sys.stdout.write(json.dumps(payload, ensure_ascii=False) + "\n")
    sys.stdout.flush()


def handle(request):
    query = request.get("query", "")
    if not isinstance(query, str):
        query = ""

    try:
        limit = int(request.get("limit", DEFAULT_LIMIT))
    except (TypeError, ValueError):
        limit = DEFAULT_LIMIT
    limit = max(1, min(MAX_LIMIT, limit))

    emitted = 0
    for pid, name, cpu, mem in read_processes():
        if not matches(query, name, pid):
            continue
        write(build_item(pid, name, cpu, mem))
        emitted += 1
        if emitted >= limit:
            break

    write({"done": True})


def main():
    for line in sys.stdin:
        line = line.strip()
        if not line:
            continue
        try:
            request = json.loads(line)
        except json.JSONDecodeError:
            continue
        if not isinstance(request, dict):
            continue
        handle(request)


if __name__ == "__main__":
    main()