#!/usr/bin/env python3
"""Fake keyshell plugin for host tests. Speaks the v1 protocol over stdio."""
import json, sys

def send(m):
    sys.stdout.write(json.dumps(m) + "\n")
    sys.stdout.flush()

def main():
    # hello
    send({"hello": {"protocol": 1, "id": "fake", "name": "Fake",
                    "badge": "FAKE", "trigger": ";", "controls": [],
                    "hot": False, "stable_ids": True}})
    for line in sys.stdin:
        line = line.strip()
        if not line:
            continue
        try:
            m = json.loads(line)
        except Exception:
            continue
        if "init" in m:
            send({"ready": {}})
        elif "query" in m:
            q = m["query"]
            send({"item": {"id": "x", "title": q.get("text", ""),
                            "subtitle": "fake", "actions": [
                                {"type": "copy", "value": q.get("text", "")}]}})
            send({"done": {"id": q["id"]}})
        elif "control" in m:
            c = m["control"]
            send({"state": {"item": c["item"], "controls": {c["control"]: c.get("value", 0)}}})
        elif "ping" in m:
            send({"pong": {"ready": True}})
        elif "cancel" in m:
            pass

if __name__ == "__main__":
    main()
