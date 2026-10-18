const assert = require("assert");
const { execFileSync } = require("child_process");
const path = require("path");
const Rank = require("../quickshell/keyshell/rank.js");
const Calc = require("../quickshell/keyshell/calc.js");
const PluginProto = require("../quickshell/keyshell/plugin-proto.js");

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

// --- plugin-proto: matchTrigger ---
assert.deepStrictEqual(PluginProto.matchTrigger(";docker ps", [";docker"]), { trigger: ";docker", remainder: "ps" }, "trigger matches and remainder extracted");
assert.strictEqual(PluginProto.matchTrigger(";docker   ps", [";docker"]).remainder, "ps", "leading space trimmed from remainder");
assert.strictEqual(PluginProto.matchTrigger(";docker", [";docker"]).remainder, "", "bare trigger yields empty remainder");
assert.strictEqual(PluginProto.matchTrigger(";docker ps", [";d", ";docker"]).trigger, ";docker", "longest match wins");
assert.strictEqual(PluginProto.matchTrigger("x;docker", [";docker"]), null, "trigger must anchor at the start");
assert.strictEqual(PluginProto.matchTrigger("=8/2", [";docker"]), null, "a built-in sigil is not a plugin trigger");
assert.strictEqual(PluginProto.matchTrigger("", [";docker"]), null, "empty query matches nothing");
assert.strictEqual(PluginProto.matchTrigger(";docker ps", []), null, "no triggers matches nothing");

// --- plugin-proto: items ---
const goodItem = { id: "a", title: "A", subtitle: "s", glyph: "x", badge: "UP", actions: [{ type: "exec", argv: ["echo", "hi"] }] };
const parsedItem = PluginProto.parseLine(JSON.stringify(goodItem));
assert.strictEqual(parsedItem.done, false, "an item line is not the terminator");
assert.strictEqual(parsedItem.item.title, "A", "title survives normalisation");
assert.strictEqual(parsedItem.item.subtitle, "s", "subtitle survives normalisation");
assert.strictEqual(PluginProto.parseLine(JSON.stringify({ done: true })).done, true, "done line detected");
assert.strictEqual(PluginProto.parseLine("not json"), null, "malformed line ignored");
assert.strictEqual(PluginProto.parseLine(""), null, "blank line ignored");
assert.strictEqual(PluginProto.parseLine(JSON.stringify({ id: "b" })), null, "item without a title is dropped");
assert.strictEqual(PluginProto.parseLine(JSON.stringify({ title: "B" })), null, "item without an id is dropped");
assert.strictEqual(PluginProto.parseLine(JSON.stringify({ id: "c", title: "C", actions: [{ type: "nope" }] })), null, "unknown action type rejected");
assert.strictEqual(PluginProto.parseLine(JSON.stringify({ id: "d", title: "D", actions: [{ type: "exec", argv: [] }] })), null, "empty argv rejected");
assert.strictEqual(PluginProto.parseLine(JSON.stringify({ id: "e", title: "E", actions: [{ type: "copy" }] })), null, "copy without text rejected");

const sibling = PluginProto.parseLine(JSON.stringify({
    id: "f", title: "F",
    actions: [{ type: "copy", text: "hi" }, { type: "nope" }]
}));
assert.strictEqual(sibling.item.actions.length, 1, "a valid action survives a rejected sibling");

const defaults = PluginProto.parseLine(JSON.stringify({ id: "g", title: "G", actions: [{ type: "set-query", query: "x" }] })).item;
assert.strictEqual(defaults.subtitle, "", "subtitle defaults to empty");
assert.strictEqual(defaults.glyph, "", "glyph defaults to empty");
assert.strictEqual(defaults.badge, "", "badge defaults to empty");

const destructiveItem = PluginProto.parseLine(JSON.stringify({ id: "h", title: "H", destructive: true, actions: [{ type: "exec", argv: ["true"] }] })).item;
assert.strictEqual(destructiveItem.destructive, true, "the destructive flag is read");

const safeItem = PluginProto.parseLine(JSON.stringify({ id: "i", title: "I", actions: [{ type: "exec", argv: ["true"] }] })).item;
assert.strictEqual(safeItem.destructive, false, "destructive defaults to false");

const truthyItem = PluginProto.parseLine(JSON.stringify({ id: "j", title: "J", destructive: "yes", actions: [{ type: "exec", argv: ["true"] }] })).item;
assert.strictEqual(truthyItem.destructive, false, "a non-boolean destructive value is not trusted");

// --- plugin-proto: manifests ---
const manifestText = PluginProto.MANIFEST_SEPARATOR + "\n" + JSON.stringify({
    name: "Docker", badge: "DOCKER", trigger: ";docker", command: ["/bin/true"], glyph: "🐳", trackable: true
});
const manifests = PluginProto.parseManifests(manifestText, 2);
assert.strictEqual(manifests.length, 1, "one manifest parsed");
assert.strictEqual(manifests[0].badge, "DOCKER", "badge read");
assert.strictEqual(manifests[0].trigger, ";docker", "trigger read");
assert.strictEqual(manifests[0].searchable, false, "searchable defaults to false");
assert.strictEqual(manifests[0].trackable, true, "trackable read");
assert.strictEqual(manifests[0].id, "plugin-docker", "id derived from the name");

assert.strictEqual(PluginProto.parseManifests(PluginProto.MANIFEST_SEPARATOR + "\n" + JSON.stringify({ name: "X", trigger: ";", command: ["/bin/true"] }), 2).length, 0, "trigger shorter than the minimum is skipped");
assert.strictEqual(PluginProto.parseManifests(PluginProto.MANIFEST_SEPARATOR + "\n" + JSON.stringify({ trigger: ";x", command: ["/bin/true"] }), 2).length, 0, "manifest without a name is skipped");
assert.strictEqual(PluginProto.parseManifests(PluginProto.MANIFEST_SEPARATOR + "\n" + JSON.stringify({ name: "X", trigger: ";xx" }), 2).length, 0, "manifest without a command is skipped");
assert.strictEqual(PluginProto.parseManifests(PluginProto.MANIFEST_SEPARATOR + "\nnot json", 2).length, 0, "malformed manifest skipped");
assert.strictEqual(PluginProto.parseManifests("", 2).length, 0, "no manifests yields nothing");

// --- process plugin: speaks the documented protocol ---
const processPlugin = path.join(__dirname, "..", "plugins", "processes", "process-plugin.py");

function runProcessPlugin(request) {
    const output = execFileSync(processPlugin, [], {
        input: JSON.stringify(request) + "\n",
        encoding: "utf8",
        timeout: 20000
    });
    return output.split("\n").filter(function (line) { return line.trim().length > 0; });
}

const procPidLines = runProcessPlugin({ query: String(process.pid), limit: 5, merged: false });
assert.strictEqual(procPidLines.length, 2, "an exact pid query returns that one process and the terminator");
assert.strictEqual(PluginProto.parseLine(procPidLines[1]).done, true, "last line is the done terminator");

const procItem = PluginProto.parseLine(procPidLines[0]).item;
assert.strictEqual(procItem.id, String(process.pid), "the item id is the pid");
assert.ok(procItem.title.length > 0, "the item has a title");
assert.ok(procItem.subtitle.indexOf("PID " + process.pid) !== -1, "the subtitle names the pid");
assert.strictEqual(procItem.actions[0].type, "exec", "the primary action is exec");
assert.ok(procItem.actions[0].argv.join(" ").indexOf("kill -TERM") !== -1, "the action terminates rather than force-kills");
assert.strictEqual(procItem.destructive, true, "the item is marked destructive so it needs a confirmation");

const procEmptyMerged = runProcessPlugin({ query: "", limit: 5, merged: true });
assert.strictEqual(procEmptyMerged.length, 1, "an empty merged query yields only the terminator");
assert.strictEqual(PluginProto.parseLine(procEmptyMerged[0]).done, true, "the only line is the terminator");

const procEmptyOwn = runProcessPlugin({ query: "", limit: 5, merged: false });
assert.strictEqual(procEmptyOwn.length, 1, "an empty query does nothing in the plugin's own mode either, since terminating is destructive");

const procPidMerged = runProcessPlugin({ query: String(process.pid), limit: 5, merged: true });
assert.strictEqual(procPidMerged.length, 2, "a named query is still answered when it comes from merged search");

const procNone = runProcessPlugin({ query: "zzzzdefinitelynotaprocess", limit: 5, merged: false });
assert.strictEqual(procNone.length, 1, "a non-matching query yields only the terminator");

const procBroad = runProcessPlugin({ query: "s", limit: 2, merged: false });
assert.ok(procBroad.length - 1 <= 2, "the limit is respected as an upper bound");

console.log("all assertions passed");