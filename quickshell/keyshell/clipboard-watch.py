#!/usr/bin/env python3
"""Appends one clipboard entry to the keyshell clipboard history store.

Run by `wl-paste --watch`, once per selection change: the selection arrives on
stdin and this process exits as soon as it has recorded it. All of the
retention and validation rules live here rather than in QML, so a test can drive
them directly.

Environment: KEYSHELL_CLIPBOARD_STORE, KEYSHELL_CLIPBOARD_MAX_ENTRIES,
KEYSHELL_CLIPBOARD_TTL_MINUTES, KEYSHELL_CLIPBOARD_MAX_BYTES.
"""

import json
import os
import sys
import tempfile
import time

DEFAULT_MAX_ENTRIES = 50
DEFAULT_TTL_MINUTES = 1440
DEFAULT_MAX_BYTES = 4096


def env_int(name, default):
    raw = os.environ.get(name, "")
    try:
        return int(raw)
    except (TypeError, ValueError):
        return default


def store_path():
    configured = os.environ.get("KEYSHELL_CLIPBOARD_STORE", "")
    if configured:
        return configured
    state = os.environ.get("XDG_STATE_HOME") or os.path.join(
        os.environ.get("HOME", ""), ".local", "state"
    )
    return os.path.join(state, "keyshell", "clipboard.json")


def load(path):
    try:
        with open(path, "r", encoding="utf-8") as handle:
            data = json.load(handle)
    except (OSError, ValueError):
        return []
    if not isinstance(data, list):
        return []

    entries = []
    for item in data:
        if not isinstance(item, dict):
            continue
        text = item.get("text")
        copied_at = item.get("copiedAt")
        if isinstance(text, str) and isinstance(copied_at, (int, float)):
            entries.append({"text": text, "copiedAt": copied_at})
    return entries


def prune(entries, now, ttl_minutes, max_entries):
    cutoff = now - ttl_minutes * 60000
    fresh = [entry for entry in entries if entry["copiedAt"] >= cutoff]
    fresh.sort(key=lambda entry: entry["copiedAt"], reverse=True)
    return fresh[:max_entries]


def save(path, entries):
    directory = os.path.dirname(path)
    if directory:
        os.makedirs(directory, exist_ok=True)
    handle, temporary = tempfile.mkstemp(dir=directory, prefix=".clipboard-", suffix=".tmp")
    try:
        with os.fdopen(handle, "w", encoding="utf-8") as stream:
            json.dump(entries, stream, ensure_ascii=False)
        os.replace(temporary, path)
    except BaseException:
        try:
            os.unlink(temporary)
        except OSError:
            pass
        raise


def main():
    max_bytes = env_int("KEYSHELL_CLIPBOARD_MAX_BYTES", DEFAULT_MAX_BYTES)
    raw = sys.stdin.buffer.read(max_bytes + 1)
    if len(raw) > max_bytes:
        return

    try:
        text = raw.decode("utf-8")
    except UnicodeDecodeError:
        return

    if text.strip() == "":
        return

    now = int(time.time() * 1000)
    path = store_path()

    entries = [
        entry for entry in load(path) if entry["text"] != text
    ]
    entries.insert(0, {"text": text, "copiedAt": now})

    max_entries = env_int("KEYSHELL_CLIPBOARD_MAX_ENTRIES", DEFAULT_MAX_ENTRIES)
    ttl_minutes = env_int("KEYSHELL_CLIPBOARD_TTL_MINUTES", DEFAULT_TTL_MINUTES)
    save(path, prune(entries, now, ttl_minutes, max_entries))


if __name__ == "__main__":
    main()