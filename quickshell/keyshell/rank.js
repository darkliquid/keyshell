// Pure ranking math. Imported by QML via `import "rank.js" as Rank` and by
// tests via `require`. No QML or Quickshell imports.

function normalizeQuery(q) {
    return (q || "").trim().toLowerCase();
}

function keywordText(raw) {
    if (Array.isArray(raw))
        return raw.join(" ").toLowerCase();
    return (raw || "").toLowerCase();
}

function matchTier(query, item) {
    const q = normalizeQuery(query);
    if (q === "")
        return 0;

    const title = (item.title || "").toLowerCase();
    if (title.indexOf(q) === 0)
        return 0;

    const parts = title.split(/[\s\-_.]+/);
    for (let i = 0; i < parts.length; i++) {
        if (parts[i].length > 0 && parts[i].indexOf(q) === 0)
            return 1;
    }

    if (title.indexOf(q) !== -1)
        return 2;

    const subtitle = (item.subtitle || "").toLowerCase();
    if (subtitle.indexOf(q) !== -1)
        return 3;
    if (keywordText(item.keywords).indexOf(q) !== -1)
        return 3;

    return -1;
}

function frecencyScore(entry, nowMs, buckets, countCap) {
    if (!entry || !entry.lastUsed)
        return 0;

    const age = nowMs - entry.lastUsed;
    if (age < 0)
        return 0;

    let weight = 0;
    for (let i = 0; i < buckets.length; i++) {
        if (age <= buckets[i].maxAgeMs) {
            weight = buckets[i].weight;
            break;
        }
    }
    if (weight === 0)
        return 0;

    const top = buckets[0].weight;
    const cap = countCap || 10;
    const countFactor = Math.min(1, Math.log1p(entry.count || 1) / Math.log1p(cap));
    const score = (weight / top) * countFactor;
    return Math.max(0, Math.min(1, score));
}

if (typeof module !== "undefined" && module.exports)
    module.exports = { matchTier: matchTier, frecencyScore: frecencyScore };