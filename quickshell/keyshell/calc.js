// Pure expression evaluation for the calculator backend. Imported by QML via
// `import "calc.js" as Calc` and by tests via `require`. No QML or Quickshell
// imports, and the input is parsed, never eval()ed.

const CONSTANTS = {
    "pi": Math.PI,
    "e": Math.E
};

const FUNCTIONS = {
    "sqrt": function (args) {
        return args.length === 1 ? Math.sqrt(args[0]) : null;
    },
    "abs": function (args) {
        return args.length === 1 ? Math.abs(args[0]) : null;
    },
    "round": function (args) {
        return args.length === 1 ? Math.round(args[0]) : null;
    },
    "floor": function (args) {
        return args.length === 1 ? Math.floor(args[0]) : null;
    },
    "ceil": function (args) {
        return args.length === 1 ? Math.ceil(args[0]) : null;
    },
    "pow": function (args) {
        return args.length === 2 ? Math.pow(args[0], args[1]) : null;
    },
    "min": function (args) {
        return args.length > 0 ? Math.min.apply(null, args) : null;
    },
    "max": function (args) {
        return args.length > 0 ? Math.max.apply(null, args) : null;
    }
};

function isDigit(ch) {
    return ch >= "0" && ch <= "9";
}

function isLetter(ch) {
    return (ch >= "a" && ch <= "z") || (ch >= "A" && ch <= "Z");
}

function skipSpaces(state) {
    while (state.pos < state.text.length && /\s/.test(state.text.charAt(state.pos)))
        state.pos++;
}

function parseNumber(state) {
    const start = state.pos;
    let seenDot = false;
    while (state.pos < state.text.length) {
        const ch = state.text.charAt(state.pos);
        if (isDigit(ch)) {
            state.pos++;
            continue;
        }
        if (ch === "." && !seenDot) {
            seenDot = true;
            state.pos++;
            continue;
        }
        break;
    }
    const value = Number(state.text.slice(start, state.pos));
    if (!isFinite(value))
        throw new Error("bad number");
    return value;
}

function parseIdentifier(state) {
    const start = state.pos;
    while (state.pos < state.text.length && isLetter(state.text.charAt(state.pos)))
        state.pos++;
    return state.text.slice(start, state.pos).toLowerCase();
}

function parseArguments(state) {
    const args = [];
    skipSpaces(state);
    if (state.text.charAt(state.pos) === ")") {
        state.pos++;
        return args;
    }
    for (;;) {
        args.push(parseExpression(state));
        skipSpaces(state);
        const ch = state.text.charAt(state.pos);
        if (ch === ",") {
            state.pos++;
            continue;
        }
        if (ch === ")") {
            state.pos++;
            return args;
        }
        throw new Error("expected , or )");
    }
}

function parsePrimary(state) {
    skipSpaces(state);
    const ch = state.text.charAt(state.pos);

    if (ch === "(") {
        state.pos++;
        const value = parseExpression(state);
        skipSpaces(state);
        if (state.text.charAt(state.pos) !== ")")
            throw new Error("expected )");
        state.pos++;
        return value;
    }

    if (isDigit(ch) || ch === ".")
        return parseNumber(state);

    if (isLetter(ch)) {
        const name = parseIdentifier(state);
        skipSpaces(state);
        if (state.text.charAt(state.pos) === "(") {
            state.pos++;
            const args = parseArguments(state);
            const fn = FUNCTIONS[name];
            if (!fn)
                throw new Error("unknown function");
            const result = fn(args);
            if (typeof result !== "number")
                throw new Error("bad call");
            return result;
        }
        if (Object.prototype.hasOwnProperty.call(CONSTANTS, name))
            return CONSTANTS[name];
        throw new Error("unknown identifier");
    }

    throw new Error("unexpected character");
}

function parseUnary(state) {
    skipSpaces(state);
    const ch = state.text.charAt(state.pos);
    if (ch === "-") {
        state.pos++;
        return -parseUnary(state);
    }
    if (ch === "+") {
        state.pos++;
        return parseUnary(state);
    }
    return parsePrimary(state);
}

function parsePower(state) {
    const base = parseUnary(state);
    skipSpaces(state);
    if (state.text.charAt(state.pos) === "^") {
        state.pos++;
        return Math.pow(base, parsePower(state));
    }
    return base;
}

function parseTerm(state) {
    let value = parsePower(state);
    for (;;) {
        skipSpaces(state);
        const ch = state.text.charAt(state.pos);
        if (ch === "*") {
            state.pos++;
            value = value * parsePower(state);
        } else if (ch === "/") {
            state.pos++;
            value = value / parsePower(state);
        } else if (ch === "%") {
            state.pos++;
            value = value % parsePower(state);
        } else {
            return value;
        }
    }
}

function parseExpression(state) {
    let value = parseTerm(state);
    for (;;) {
        skipSpaces(state);
        const ch = state.text.charAt(state.pos);
        if (ch === "+") {
            state.pos++;
            value = value + parseTerm(state);
        } else if (ch === "-") {
            state.pos++;
            value = value - parseTerm(state);
        } else {
            return value;
        }
    }
}

function evaluate(expression) {
    const text = (expression || "").trim();
    if (text.length === 0)
        return null;

    const state = { "text": text, "pos": 0 };
    let value;
    try {
        value = parseExpression(state);
        skipSpaces(state);
        if (state.pos !== text.length)
            return null;
    } catch (e) {
        return null;
    }

    if (typeof value !== "number" || !isFinite(value))
        return null;
    return value;
}

function groupThousands(text) {
    const negative = text.charAt(0) === "-";
    const body = negative ? text.slice(1) : text;
    const dot = body.indexOf(".");
    const digits = (dot === -1) ? body : body.slice(0, dot);
    const rest = (dot === -1) ? "" : body.slice(dot);

    let grouped = "";
    for (let i = 0; i < digits.length; i++) {
        if (i > 0 && ((digits.length - i) % 3) === 0)
            grouped += ",";
        grouped += digits.charAt(i);
    }
    return (negative ? "-" : "") + grouped + rest;
}

function format(value, maxDecimals) {
    if (typeof value !== "number" || !isFinite(value))
        return "";

    const decimals = (typeof maxDecimals === "number" && maxDecimals >= 0) ? maxDecimals : 10;
    const abs = Math.abs(value);
    if (abs !== 0 && (abs >= 1e15 || abs < 1e-9))
        return String(value);

    let text = value.toFixed(decimals);
    if (text.indexOf(".") !== -1)
        text = text.replace(/0+$/, "").replace(/\.$/, "");
    return groupThousands(text);
}

if (typeof module !== "undefined" && module.exports)
    module.exports = { evaluate: evaluate, format: format };