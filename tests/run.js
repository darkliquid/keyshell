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

// --- mergeAndSort ---
const groups = [
    { source: "apps", items: [ item("Firefox"), item("Files") ] },
    { source: "windows", items: [ item("Firefox - Tab") ] }
];
const entries = { "apps:Firefox": { count: 10, lastUsed: now - 1000 } };
const opts = { buckets: buckets, countCap: 10, limit: 10 };

const merged = Rank.mergeAndSort("fir", groups, entries, now, opts);
assert.strictEqual(merged.length, 2, "only matching items survive");
assert.strictEqual(merged[0].source, "apps", "frecency boosts the app hit");
assert.strictEqual(merged[1].source, "windows", "lower frecency sorts second");
assert.ok(merged.every(function (m) { return typeof m.tier === "number" && typeof m.frecency === "number"; }), "tier and frecency attached");

assert.strictEqual(Rank.mergeAndSort("zzz", groups, entries, now, opts).length, 0, "non-matching dropped");
assert.strictEqual(Rank.mergeAndSort("", groups, {}, now, opts).length, 3, "empty query keeps everything");

console.log("all rank assertions passed");