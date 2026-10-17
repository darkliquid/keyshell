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

// --- frecencyScore ---
const buckets = [
    { "maxAgeMs": 3600000, "weight": 4 },
    { "maxAgeMs": 86400000, "weight": 3 },
    { "maxAgeMs": 604800000, "weight": 2 },
    { "maxAgeMs": 2592000000, "weight": 1 }
];
const now = 1000000000000;
const freshMany = Rank.frecencyScore({ count: 10, lastUsed: now - 1000 }, now, buckets, 10);
const freshOne = Rank.frecencyScore({ count: 1, lastUsed: now - 1000 }, now, buckets, 10);
const oldMany = Rank.frecencyScore({ count: 10, lastUsed: now - 900000000 }, now, buckets, 10);

assert.ok(freshMany > freshOne, "more hits scores higher in the same bucket");
assert.ok(freshOne > oldMany, "recent beats old at equal count");
assert.strictEqual(Rank.frecencyScore(null, now, buckets, 10), 0, "no entry -> 0");
assert.strictEqual(Rank.frecencyScore({ count: 5, lastUsed: now - 99999999999 }, now, buckets, 10), 0, "beyond oldest bucket -> 0");

console.log("all rank assertions passed");