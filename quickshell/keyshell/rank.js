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

function mergeAndSort(query, groups, entries, nowMs, options) {
    const opts = options || {};
    const buckets = opts.buckets || [];
    const countCap = opts.countCap || 10;
    const limit = opts.limit || 60;

    const merged = [];
    for (let g = 0; g < groups.length; g++) {
        const source = groups[g].source;
        const items = groups[g].items || [];
        for (let i = 0; i < items.length; i++) {
            const item = items[i];
            const tier = matchTier(query, item);
            if (tier < 0)
                continue;

            const key = source + ":" + item.id;
            const entry = entries ? entries[key] : null;
            const copy = Object.assign({}, item);
            copy.source = source;
            copy.tier = tier;
            copy.frecency = frecencyScore(entry, nowMs, buckets, countCap);
            merged.push(copy);
        }
    }

    merged.sort(function (a, b) {
        if (a.tier !== b.tier)
            return a.tier - b.tier;
        if (a.frecency !== b.frecency)
            return b.frecency - a.frecency;
        const al = (a.title || "").length;
        const bl = (b.title || "").length;
        if (al !== bl)
            return al - bl;
        const at = (a.title || "").toLowerCase();
        const bt = (b.title || "").toLowerCase();
        return at < bt ? -1 : (at > bt ? 1 : 0);
    });

    return merged.slice(0, limit);
}

if (typeof module !== "undefined" && module.exports)
    module.exports = { matchTier: matchTier, frecencyScore: frecencyScore, mergeAndSort: mergeAndSort };