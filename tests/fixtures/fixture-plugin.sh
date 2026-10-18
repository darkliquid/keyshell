#!/bin/sh
# Minimal keyshell plugin fixture: one JSON request per line in, items plus a
# terminator out. The first item's action writes a marker file, so a test can
# observe both that a request was answered and that an action ran, without
# depending on the clipboard.
while IFS= read -r line; do
    printf '%s\n' '{"id":"one","title":"Fixture One","subtitle":"writes a marker file","glyph":"1","badge":"TEST","actions":[{"type":"exec","argv":["/bin/sh","-c","echo activated > /tmp/keyshell-fixture-activated"]}]}'
    printf '%s\n' '{"id":"two","title":"Fixture Two","subtitle":"sets the query","glyph":"2","actions":[{"type":"set-query","query":"fixture"}]}'
    printf '%s\n' '{"done":true}'
done