#!/usr/bin/env python3
"""keyshell emoji plugin.

Reads one JSON request per line on stdin:

    {"query": "rock", "limit": 40, "merged": false}

and answers with one JSON item per line followed by the response terminator
``{"done": true}``.

The index is built once from the standard library's Unicode names, so the plugin
ships no data file and needs no network. Only single-codepoint emoji are
enumerated: ZWJ sequences such as families, professions, flags and skin-tone
variants are not included.

For an empty query the ``merged`` flag decides. Merged search gets nothing, so a
plain launcher open is never filled with emoji. The plugin's own mode gets a
starter set, which the shell then orders by usage, so the picker opens on the
emoji you reach for most.
"""

import json
import sys
import unicodedata

RANGES = [
    (0x1F300, 0x1F5FF),
    (0x1F600, 0x1F64F),
    (0x1F680, 0x1F6FF),
    (0x1F900, 0x1F9FF),
    (0x1FA70, 0x1FAFF),
    (0x2600, 0x26FF),
    (0x2700, 0x27BF),
]

DEFAULT_LIMIT = 40
MAX_LIMIT = 200


def build_index():
    """(codepoint, glyph, name, words) for every named emoji in RANGES."""
    index = []
    for low, high in RANGES:
        for codepoint in range(low, high + 1):
            glyph = chr(codepoint)
            try:
                name = unicodedata.name(glyph)
            except ValueError:
                continue
            if not name:
                continue
            index.append((codepoint, glyph, name, name.lower().split()))
    return index


INDEX = build_index()


def humanise(name):
    return " ".join(word.capitalize() for word in name.lower().split())


def rank(entry, query):
    """First-word matches first, then shorter names, then codepoint."""
    codepoint, _, name, words = entry
    first_word_hit = 0 if (words and words[0].startswith(query)) else 1
    return (first_word_hit, len(name), codepoint)


def build_item(entry):
    codepoint, glyph, name, _ = entry
    return {
        "id": "u{:04X}".format(codepoint),
        "title": humanise(name),
        "subtitle": "U+{:04X}".format(codepoint),
        "glyph": glyph,
        "badge": "",
        "actions": [{"type": "copy", "text": glyph}],
    }


def write(payload):
    sys.stdout.write(json.dumps(payload, ensure_ascii=False) + "\n")
    sys.stdout.flush()


def candidates(query, merged, limit):
    if query == "":
        if merged:
            return []
        return sorted(INDEX, key=lambda entry: (len(entry[2]), entry[0]))[:limit]

    hits = [entry for entry in INDEX if any(word.startswith(query) for word in entry[3])]
    hits.sort(key=lambda entry: rank(entry, query))
    return hits[:limit]


def handle(request):
    query = request.get("query", "")
    if not isinstance(query, str):
        query = ""
    query = query.strip().lower()

    merged = request.get("merged", False) is True

    try:
        limit = int(request.get("limit", DEFAULT_LIMIT))
    except (TypeError, ValueError):
        limit = DEFAULT_LIMIT
    limit = max(1, min(MAX_LIMIT, limit))

    for entry in candidates(query, merged, limit):
        write(build_item(entry))

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