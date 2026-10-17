const assert = require("assert");
const Rank = require("../quickshell/keyshell/rank.js");
const Calc = require("../quickshell/keyshell/calc.js");

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

// --- calc: evaluate ---
assert.strictEqual(Calc.evaluate("2+3*4"), 14, "precedence");
assert.strictEqual(Calc.evaluate("(2+3)*4"), 20, "parentheses");
assert.strictEqual(Calc.evaluate("-5+2"), -3, "leading unary minus");
assert.strictEqual(Calc.evaluate("3*-2"), -6, "unary minus after an operator");
assert.strictEqual(Calc.evaluate("2^10"), 1024, "power");
assert.strictEqual(Calc.evaluate("2^3^2"), 512, "power is right-associative");
assert.strictEqual(Calc.evaluate("10%3"), 1, "modulo");
assert.strictEqual(Calc.evaluate(" 2 + 2 "), 4, "whitespace tolerated");
assert.strictEqual(Calc.evaluate("sqrt(16)"), 4, "sqrt");
assert.strictEqual(Calc.evaluate("max(1,5,3)"), 5, "max");
assert.strictEqual(Calc.evaluate("min(4,2)"), 2, "min");
assert.strictEqual(Calc.evaluate("abs(-3)"), 3, "abs");
assert.strictEqual(Calc.evaluate("round(2.6)"), 3, "round");
assert.strictEqual(Calc.evaluate("floor(2.9)"), 2, "floor");
assert.strictEqual(Calc.evaluate("ceil(2.1)"), 3, "ceil");
assert.strictEqual(Calc.evaluate("pow(2,8)"), 256, "pow");
assert.ok(Math.abs(Calc.evaluate("pi") - Math.PI) < 1e-9, "pi");
assert.ok(Math.abs(Calc.evaluate("e") - Math.E) < 1e-9, "e");
assert.strictEqual(Calc.evaluate("1/0"), null, "divide by zero is invalid");
assert.strictEqual(Calc.evaluate("0/0"), null, "zero over zero is invalid");
assert.strictEqual(Calc.evaluate("2+"), null, "trailing operator");
assert.strictEqual(Calc.evaluate("(("), null, "unbalanced parens");
assert.strictEqual(Calc.evaluate(""), null, "empty input");
assert.strictEqual(Calc.evaluate("abc"), null, "unknown identifier");
assert.strictEqual(Calc.evaluate("2+*3"), null, "double operator");
assert.strictEqual(Calc.evaluate("sqrt()"), null, "empty argument list");
assert.strictEqual(Calc.evaluate("nope(1)"), null, "unknown function");

// --- calc: format ---
assert.strictEqual(Calc.format(14, 10), "14", "integer formatting");
assert.strictEqual(Calc.format(0.1 + 0.2, 10), "0.3", "float noise trimmed");
assert.strictEqual(Calc.format(1234567, 10), "1,234,567", "thousands separators");
assert.strictEqual(Calc.format(-42, 10), "-42", "negative integer");
assert.strictEqual(Calc.format(1 / 3, 4), "0.3333", "decimals limited");
assert.strictEqual(Calc.format(1e21, 10), "1e+21", "very large uses exponent form");

console.log("all assertions passed");