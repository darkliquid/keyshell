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

const ACTION_TYPES = ["copy", "open-url", "exec", "exec-terminal", "set-query"];

function requiredField(actionType) {
    if (actionType === "copy")
        return "text";
    if (actionType === "open-url")
        return "url";
    if (actionType === "set-query")
        return "query";
    return "argv";
}

function isValidAction(action) {
    if (!action || typeof action !== "object")
        return false;
    if (ACTION_TYPES.indexOf(action.type) === -1)
        return false;

    const field = requiredField(action.type);
    if (field === "argv") {
        if (!Array.isArray(action.argv) || action.argv.length === 0)
            return false;
        for (let i = 0; i < action.argv.length; i++) {
            if (typeof action.argv[i] !== "string")
                return false;
        }
        return true;
    }
    return typeof action[field] === "string" && action[field].length > 0;
}

function normalizeItem(raw) {
    if (!raw || typeof raw !== "object")
        return null;

    const id = (typeof raw.id === "string" && raw.id.length > 0) ? raw.id : null;
    const title = (typeof raw.title === "string" && raw.title.length > 0) ? raw.title : null;
    if (id === null || title === null)
        return null;

    const actions = [];
    const rawActions = Array.isArray(raw.actions) ? raw.actions : [];
    for (let i = 0; i < rawActions.length; i++) {
        if (isValidAction(rawActions[i]))
            actions.push(rawActions[i]);
    }
    if (actions.length === 0)
        return null;

    return {
        "id": id,
        "title": title,
        "subtitle": typeof raw.subtitle === "string" ? raw.subtitle : "",
        "glyph": typeof raw.glyph === "string" ? raw.glyph : "",
        "badge": typeof raw.badge === "string" ? raw.badge : "",
        "actions": actions
    };
}

function parseLine(line) {
    const text = (line || "").trim();
    if (text.length === 0)
        return null;

    let parsed;
    try {
        parsed = JSON.parse(text);
    } catch (e) {
        return null;
    }

    if (parsed && typeof parsed === "object" && parsed.done === true)
        return { "done": true, "item": null };

    const item = normalizeItem(parsed);
    if (item === null)
        return null;
    return { "done": false, "item": item };
}

function normalizeManifest(raw, minTriggerLength) {
    if (!raw || typeof raw !== "object")
        return null;

    const minLength = (typeof minTriggerLength === "number") ? minTriggerLength : 2;
    const name = (typeof raw.name === "string" && raw.name.length > 0) ? raw.name : null;
    const trigger = (typeof raw.trigger === "string" && raw.trigger.length >= minLength) ? raw.trigger : null;
    const command = Array.isArray(raw.command)
        ? raw.command.filter(function (part) { return typeof part === "string" && part.length > 0; })
        : [];

    if (name === null || trigger === null || command.length === 0)
        return null;

    const slug = name.toLowerCase().replace(/[^a-z0-9]+/g, "-").replace(/^-+|-+$/g, "");
    return {
        "id": "plugin-" + (slug.length > 0 ? slug : "unnamed"),
        "name": name,
        "badge": (typeof raw.badge === "string" && raw.badge.length > 0) ? raw.badge : name.toUpperCase().slice(0, 12),
        "trigger": trigger,
        "command": command,
        "glyph": typeof raw.glyph === "string" ? raw.glyph : "",
        "searchable": raw.searchable === true,
        "trackable": raw.trackable === true,
        "description": typeof raw.description === "string" ? raw.description : ""
    };
}

function parseManifests(text, minTriggerLength) {
    const manifests = [];
    const blocks = (text || "").split(MANIFEST_SEPARATOR);
    for (let i = 0; i < blocks.length; i++) {
        const block = blocks[i].trim();
        if (block.length === 0)
            continue;

        let parsed;
        try {
            parsed = JSON.parse(block);
        } catch (e) {
            continue;
        }

        const manifest = normalizeManifest(parsed, minTriggerLength);
        if (manifest !== null)
            manifests.push(manifest);
    }
    return manifests;
}

if (typeof module !== "undefined" && module.exports)
    module.exports = {
        MANIFEST_SEPARATOR: MANIFEST_SEPARATOR,
        matchTrigger: matchTrigger,
        parseLine: parseLine,
        normalizeItem: normalizeItem,
        parseManifests: parseManifests
    };