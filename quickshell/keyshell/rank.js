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

if (typeof module !== "undefined" && module.exports)
    module.exports = { matchTier: matchTier };