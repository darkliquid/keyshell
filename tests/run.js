const assert = require("assert");
const Rank = require("../quickshell/keyshell/rank.js");

function item(title, subtitle, keywords) {
    return { "id": title, "title": title, "subtitle": subtitle || "", "keywords": keywords || "" };
}

// --- matchTier ---
assert.strictEqual(Rank.matchTier("fir", item("Firefox")), 0, "prefix -> tier 0");
assert.strictEqual(Rank.matchTier("editor", item("Code Editor")), 1, "word start -> tier 1");
assert.strictEqual(Rank.matchTier("fox", item("Firefox")), 2, "substring -> tier 2");
assert.strictEqual(Rank.matchTier("fire", item("Code Editor", "firefox profile")), 3, "subtitle -> tier 3");
assert.strictEqual(Rank.matchTier("fire", item("Code Editor", "", ["firefox"])), 3, "keyword -> tier 3");
assert.strictEqual(Rank.matchTier("zzz", item("Firefox")), -1, "no match -> -1");
assert.strictEqual(Rank.matchTier("", item("Firefox")), 0, "empty query -> tier 0");

console.log("all rank assertions passed");