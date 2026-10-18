// Protocol helpers for external plugin backends. Pure JS: imported by QML via
// `import "plugin-proto.js" as PluginProto` and by tests via `require`.

const MANIFEST_SEPARATOR = "---KEYSHELL-MANIFEST---";

function matchTrigger(query, triggers) {
    const text = query || "";
    let best = null;
    for (let i = 0; i < triggers.length; i++) {
        const trigger = triggers[i];
        if (typeof trigger !== "string" || trigger.length === 0)
            continue;
        if (text.indexOf(trigger) !== 0)
            continue;
        if (best === null || trigger.length > best.length)
            best = trigger;
    }
    if (best === null)
        return null;
    return {
        "trigger": best,
        "remainder": text.slice(best.length).replace(/^\s+/, "")
    };
}

if (typeof module !== "undefined" && module.exports)
    module.exports = { MANIFEST_SEPARATOR: MANIFEST_SEPARATOR, matchTrigger: matchTrigger };