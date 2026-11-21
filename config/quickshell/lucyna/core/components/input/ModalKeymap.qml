import QtQuick

/*!
    ModalKeymap — generic vim-style key engine.

    Resolves a key event to a token, matches it (or a pending sequence such as
    "gg") against tables[mode] and emits actionTriggered(action, token).
    Knows nothing about what the actions do.
*/
Item {
    id: root

    // Consumer-defined key into tables; only "insert" is special (passes
    // unmapped keys through).
    property string mode:    "normal"
    // mode -> { token or sequence -> action name }
    property var    tables:  ({})

    // Merge baseTable under the active table; the panel table wins on the same token.
    // Unsupported with modifierPolicy "legacy" (its tokens never include <S-CR> or <Space>).
    property bool   useBase: false
    // "both" | "horizontal" | "vertical": drops the other axis from the base table.
    property string axis:    "both"
    property bool   wrap:    false
    // Text-field panels: base letters need Shift (<S-h>...) and Space is not bound.
    property bool   shiftOnly: false
    // "legacy" | "fallback" | "exactArrows" (arrow keys never fall back to the unmodified token).
    property string modifierPolicy: "legacy"
    property bool   passThroughUnmapped: mode === _modeInsert

    readonly property var baseTable: ({
        "<Left>":    "moveLeft",
        "h":         "moveLeft",
        "<Right>":   "moveRight",
        "l":         "moveRight",
        "<Up>":      "moveUp",
        "k":         "moveUp",
        "<Down>":    "moveDown",
        "j":         "moveDown",
        "<CR>":      "activate",
        "<Space>":   "activate",
        "<S-CR>":    "activateAlt",
        "<S-Space>": "activateAlt",
        "<Esc>":     "cancel"
    })

    // Vim 'timeoutlen' default.
    readonly property int pendingTimeoutMs: 1000
    readonly property string _literalLessThan: "<lt>"
    readonly property string _policyLegacy: "legacy"
    readonly property string _policyExactArrows: "exactArrows"
    readonly property string _axisHorizontal: "horizontal"
    readonly property string _axisVertical: "vertical"
    readonly property string _modeInsert: "insert"
    readonly property var _verticalActions: ["moveUp", "moveDown"]
    readonly property var _horizontalActions: ["moveLeft", "moveRight"]
    readonly property var _shiftOnlyDroppedTokens: ["<Space>", "<S-Space>"]

    property string pending: ""

    signal actionTriggered(string action, string token)

    readonly property var _namedKeys: ({
        [Qt.Key_Tab]:     "<Tab>",
        // Qt delivers Shift+Tab as Backtab.
        [Qt.Key_Backtab]: "<S-Tab>",
        [Qt.Key_Escape]:  "<Esc>",
        [Qt.Key_Left]:    "<Left>",
        [Qt.Key_Right]:   "<Right>",
        [Qt.Key_Up]:      "<Up>",
        [Qt.Key_Down]:    "<Down>",
        [Qt.Key_Return]:  "<CR>",
        [Qt.Key_Enter]:   "<CR>"
    })

    // Names used inside qualified tokens ("<C-CR>"); Backtab is Tab with Shift forced.
    readonly property var _keyNames: ({
        [Qt.Key_Tab]:     "Tab",
        [Qt.Key_Backtab]: "Tab",
        [Qt.Key_Escape]:  "Esc",
        [Qt.Key_Left]:    "Left",
        [Qt.Key_Right]:   "Right",
        [Qt.Key_Up]:      "Up",
        [Qt.Key_Down]:    "Down",
        [Qt.Key_Return]:  "CR",
        [Qt.Key_Enter]:   "CR",
        [Qt.Key_Space]:   "Space"
    })

    readonly property var _arrowKeys: [Qt.Key_Left, Qt.Key_Right, Qt.Key_Up, Qt.Key_Down]

    readonly property var _effectiveBase: {
        const result = ({});
        for (const token in baseTable) {
            const action = baseTable[token];
            if (axis === _axisHorizontal && _verticalActions.includes(action))
                continue;
            if (axis === _axisVertical && _horizontalActions.includes(action))
                continue;
            if (shiftOnly && _shiftOnlyDroppedTokens.includes(token))
                continue;
            result[shiftOnly && token.length === 1 ? "<S-" + token + ">" : token] = action;
        }
        return result;
    }

    onModeChanged: pending = ""
    onEnabledChanged: pending = ""

    Timer {
        id: pendingTimer
        interval: root.pendingTimeoutMs
        running: root.pending !== ""
        onTriggered: root.pending = ""
    }

    // event.text, not key codes: Shift-produced characters ("/", "G") vary by layout.
    function _tokenFor(event) {
        if (event.modifiers & (Qt.AltModifier | Qt.MetaModifier))
            return "";
        const ctrl = event.modifiers & Qt.ControlModifier;
        if (ctrl && (event.key === Qt.Key_Tab || event.key === Qt.Key_Backtab))
            return "";
        if (_namedKeys[event.key])
            return _namedKeys[event.key];
        if (ctrl && event.key >= Qt.Key_A && event.key <= Qt.Key_Z)
            return (event.modifiers & Qt.ShiftModifier) ? "" : "<C-" + String.fromCharCode(event.key).toLowerCase() + ">";
        // A typed "<" would otherwise prefix-match tokens like "<CR>" and "<C-d>" (vim's <lt>).
        return event.text === "<" ? _literalLessThan : event.text;
    }

    function _modifierPrefix(modifiers, includeShift) {
        return (modifiers & Qt.ControlModifier ? "C-" : "")
            + (includeShift && (modifiers & Qt.ShiftModifier) ? "S-" : "")
            + (modifiers & Qt.AltModifier ? "A-" : "")
            + (modifiers & Qt.MetaModifier ? "M-" : "");
    }

    function _characterToken(character, modifiers, includeShift) {
        const prefix = _modifierPrefix(modifiers, includeShift);
        return prefix === "" ? character : "<" + prefix + character + ">";
    }

    // Candidate tokens in lookup order: qualified first, then (named keys only) the unmodified one.
    function _qualifiedTokens(event) {
        let modifiers = event.modifiers & ~Qt.KeypadModifier;
        const name = _keyNames[event.key];
        if (name !== undefined) {
            const isBacktab = event.key === Qt.Key_Backtab;
            if (isBacktab)
                modifiers |= Qt.ShiftModifier;
            const qualified = "<" + _modifierPrefix(modifiers, true) + name + ">";
            const unmodified = isBacktab ? "<S-Tab>" : "<" + name + ">";
            if (qualified === unmodified)
                return [qualified];
            return _arrowKeys.includes(event.key) && modifierPolicy === _policyExactArrows
                ? [qualified]
                : [qualified, unmodified];
        }
        if (event.key >= Qt.Key_A && event.key <= Qt.Key_Z)
            return [_characterToken(String.fromCharCode(event.key).toLowerCase(), modifiers, true)];
        if (event.text === "<")
            return [_literalLessThan];
        // Shift is already part of a typed character ("?", "G"), so it is not prefixed.
        return event.text === "" ? [] : [_characterToken(event.text, modifiers, false)];
    }

    function _candidateTokens(event) {
        if (modifierPolicy === _policyLegacy) {
            const token = _tokenFor(event);
            return token === "" ? [] : [token];
        }
        return _qualifiedTokens(event);
    }

    function _activeTable() {
        const own = tables[mode] || ({});
        const merged = useBase ? Object.assign({}, _effectiveBase, own) : own;
        if (modifierPolicy === _policyLegacy || merged[" "] === undefined || merged["<Space>"] !== undefined)
            return merged;
        return Object.assign({ "<Space>": merged[" "] }, merged);
    }

    function _hasPrefix(table, buffer) {
        for (const key in table) {
            if (key.length > buffer.length && key.startsWith(buffer))
                return true;
        }
        return false;
    }

    function step(index, delta, count) {
        if (index < 0 || count <= 0)
            return 0;
        const next = index + delta;
        return wrap ? (next % count + count) % count : Math.max(0, Math.min(count - 1, next));
    }

    function handlerFor(handlers, action) {
        const handler = handlers[action];
        return handler === undefined && action === "activateAlt" ? handlers.activate : handler;
    }

    function handleKey(event) {
        event.accepted = false;
        if (!enabled)
            return;

        const candidates = _candidateTokens(event);
        const table = _activeTable();
        const swallowsUnmapped = !passThroughUnmapped;

        // Non-text keys (bare modifiers, F-keys, dead keys, legacy Alt/Meta combos)
        // must not disturb a pending sequence, e.g. Shift before the "T" of "gT".
        if (candidates.length === 0) {
            event.accepted = swallowsUnmapped;
            return;
        }

        for (const token of candidates) {
            const buffer = pending + token;
            if (table[buffer] !== undefined) {
                pending = "";
                event.accepted = true;
                actionTriggered(table[buffer], token);
                return;
            }
            if (_hasPrefix(table, buffer)) {
                pending = buffer;
                pendingTimer.restart();
                event.accepted = true;
                return;
            }
        }

        // Up/Down are list navigation: an unbound press must not break a pending sequence.
        if (event.key !== Qt.Key_Up && event.key !== Qt.Key_Down)
            pending = "";
        event.accepted = swallowsUnmapped;
    }
}
