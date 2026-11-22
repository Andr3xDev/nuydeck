import "../../../core/components/panel"
import "../../../core/services/panel" as Services
import "../../../core/theme" as Theme
import QtQuick
import QtQuick.Layouts
import Quickshell.Io
import "../../../core/components/atomic"
import "components"
import "../../../core/components/input"
import "state"

/*!
    ClipboardPanel — modal clipboard history / favorites panel hosted on quickshell-top.
    Keeps the legacy ClipLauncher registration name ("clipLauncher") and the toggleClip IPC target.
*/
PanelBase {
    id: root

    // Measured from the content's text-dependent layout; ZoneSlot warns when it drifts.
    readonly property int measuredHeight: 317

    name: "clipLauncher"
    placement: PanelPlacement {
        type: Services.PanelRules.Type.Edge
        shape: Services.PanelRules.Shape.Wide
        size: Qt.size(Theme.Tokens.comp.clip.contentW, root.measuredHeight)
    }
    onShown: {
        ClipboardState.reset();
        ClipboardHistory.refresh();
        keymap.mode = root.modeNormal;
    }
    onKeyboardFocused: root._restoreFocus()
    // Decoded images are removed only once the close animation has finished.
    onPresentChanged: {
        if (!root.present)
            ClipboardHistory.cleanup();
    }

    readonly property string modeNormal:  "normal"
    readonly property string modeInsert:  "insert"
    readonly property string modePick:    "pick"
    readonly property string modeConfirm: "confirm"
    readonly property string promptText:  "text"
    readonly property string reasonEmpty:     "cannot be empty"
    readonly property string reasonDuplicate: "already exists"
    readonly property string optionNewCollection: "+ new collection"
    readonly property string noticeTextOnly:       "Only text can be favorited"
    readonly property string noticePreviewLoading: "Preview still loading, try again"
    readonly property string noticeNotReady:       "Favorites are not loaded yet"
    readonly property string noticeNotFound:       "That item no longer exists"
    readonly property string noticeReadOnly:       "Favorites are read-only"
    readonly property string noticeProtected:      "The General collection cannot be deleted or renamed"
    readonly property string noticeAlreadySecret:  "Already secret"
    readonly property string noticeFailed:         "Could not apply the change"
    readonly property string promptPick:    "pick"
    readonly property string promptConfirm: "confirm"
    readonly property string promptNotice:  "notice"
    readonly property string titleKeyName: "Key name"
    readonly property string titleNewCollection: "New collection name"
    readonly property string titleRenameCollection: "Rename collection"
    readonly property string titleRenameEntry: "Rename entry"
    readonly property string titleAddToCollection: "Add to collection"
    readonly property string titleWipeHistory: "Wipe all clipboard history?"
    readonly property string titleValuePrefix: "Value for '"
    readonly property string titleDeletePrefix: "Delete collection '"
    readonly property string titleDeleteSuffix: "' and its entries?"
    readonly property string quoteSuffix: "'"
    readonly property string titleSecretPrefix: "Mark '"
    readonly property string titleSecretSuffix: "' as secret? This can't be undone."
    readonly property string placeholderCollection: "Collection name"
    readonly property string placeholderKey: "e.g. only binary"
    readonly property string placeholderValue: "Value"
    readonly property int confirmYes: 0
    readonly property int confirmNo:  1
    readonly property int halfPage: Math.floor(Theme.Tokens.comp.clip.listRows / 2)

    // ContentPane has its own `keymap` property, which shadows the id there.
    readonly property Item panelKeymap: keymap
    readonly property bool _favorites: ClipboardState.section === ClipboardState.sectionFavorites
    readonly property string _promptKind: ClipboardState.prompt ? ClipboardState.prompt.kind : ""

    function _restoreFocus() {
        const item = root.contentItem;
        if (!item)
            return;
        if (root._promptKind === root.promptText) {
            item.focusPrompt();
        } else if (keymap.mode === root.modeInsert) {
            item.focusFilter();
        } else {
            item.forceActiveFocus();
        }
    }

    // One pending step: called with the picked index (pick), the typed text (text) or nothing (confirm).
    // A text step returns "" or an err* constant; it chains by opening the next prompt itself.
    property var _pending: null
    property int _pickIndex: 0

    function _openPrompt(descriptor, onResolve) {
        root._pending = onResolve;
        root._pickIndex = 0;
        ClipboardState.prompt = descriptor;
    }

    function _closePrompt() {
        ClipboardState.prompt = null;
    }

    function _askText(message, text, secret, placeholder, onText) {
        root._openPrompt({ kind: root.promptText, message: message, text: text, secret: secret, placeholder: placeholder }, onText);
    }

    function _askConfirm(message, onConfirm) {
        root._openPrompt({ kind: root.promptConfirm, message: message }, onConfirm);
        root._pickIndex = root.confirmNo;
    }

    function _notify(message) {
        root._openPrompt({ kind: root.promptNotice, message: message }, null);
    }

    function _guardWritable() {
        if (!FavoritesStore.readOnly)
            return true;
        root._notify(root.noticeReadOnly);
        return false;
    }

    function _noticeFor(error) {
        if (error === FavoritesStore.errNotReady)
            return root.noticeNotReady;
        if (error === FavoritesStore.errNotFound)
            return root.noticeNotFound;
        if (error === FavoritesStore.errProtected)
            return root.noticeProtected;
        return error === FavoritesStore.errReadOnly ? root.noticeReadOnly : root.noticeFailed;
    }

    function _hasEntry(collection, key) {
        const target = FavoritesStore.collections.find(c => c.name === collection);
        const wanted = key.trim();
        return !!target && target.entries.some(e => e.key === wanted);
    }

    function _reasonFor(error) {
        if (error === FavoritesStore.errEmpty)
            return root.reasonEmpty;
        return error === FavoritesStore.errDuplicate ? root.reasonDuplicate : "";
    }

    // Empty/duplicate keep the prompt open with the reason; any other result ends it.
    function _acceptText(text) {
        const run = root._pending;
        if (!run || root._promptKind !== root.promptText)
            return;
        const submitted = ClipboardState.prompt;
        const error = run(text);
        const reason = root._reasonFor(error);
        if (reason !== "")
            ClipboardState.prompt = Object.assign({}, submitted, { text: text, error: reason });
        else if (root._pending !== run)
            return;
        else if (error !== "")
            root._notify(root._noticeFor(error));
        else
            root._closePrompt();
    }

    function _resolvePick() {
        const run = root._pending;
        const index = root._pickIndex;
        root._closePrompt();
        if (run)
            run(index);
    }

    function _resolveConfirm(accepted) {
        const run = root._pending;
        root._closePrompt();
        if (accepted && run)
            run();
    }

    function _toggleConfirm() {
        root._pickIndex = root._pickIndex === root.confirmYes ? root.confirmNo : root.confirmYes;
    }

    function _movePick(delta) {
        const last = ClipboardState.prompt.options.length - 1;
        root._pickIndex = Math.max(0, Math.min(root._pickIndex + delta, last));
    }

    function _copySelected() {
        const source = ClipboardState.selectedSource;
        if (!source)
            return;
        if (!root._favorites)
            ClipboardHistory.copy(source.id);
        else if (ClipboardState.collection !== "")
            FavoritesStore.copy(source.value);
        else
            return;
        root.hideIfAutoCloses();
    }

    function _copyIndex(token) {
        const position = parseInt(token);
        if (position > ClipboardState.rowCount || (root._favorites && ClipboardState.collection === ""))
            return;
        ClipboardState.select(position - 1);
        root._copySelected();
    }

    function _escape() {
        if (ClipboardState.filter !== "")
            ClipboardState.setFilter("");
        else
            root.hide();
    }

    function _activate() {
        const source = ClipboardState.selectedSource;
        if (source && root._favorites && ClipboardState.collection === "")
            ClipboardState.openCollection(source.name);
        else
            root._copySelected();
    }

    function _deleteSelected() {
        const source = ClipboardState.selectedSource;
        if (!source)
            return;
        if (!root._favorites) {
            ClipboardHistory.remove(source.rawLine);
        } else if (!root._guardWritable()) {
            return;
        } else if (ClipboardState.collection !== "") {
            FavoritesStore.deleteEntry(ClipboardState.collection, source.key);
        } else if (FavoritesStore.isProtected(source.name)) {
            root._notify(root.noticeProtected);
        } else {
            root._askConfirm(root.titleDeletePrefix + source.name + root.titleDeleteSuffix,
                             () => FavoritesStore.deleteCollection(source.name));
        }
    }

    function _wipeHistory() {
        root._askConfirm(root.titleWipeHistory, () => ClipboardHistory.wipe());
    }

    function _addToFavorites() {
        const source = ClipboardState.selectedSource;
        if (!source || !root._guardWritable())
            return;
        if (source.isImage) {
            root._notify(root.noticeTextOnly);
            return;
        }
        // The list title is a truncated preview; only the decoded preview holds the full text.
        const value = ClipboardHistory.previewText;
        if (value === "") {
            root._notify(root.noticePreviewLoading);
            return;
        }
        const names = FavoritesStore.collections.map(c => c.name);
        const askKey = name => root._askText(root.titleKeyName, "", false, root.placeholderKey, key => FavoritesStore.addToCollection(name, key, value));
        const askName = () => root._askText(root.titleNewCollection, "", false, root.placeholderCollection, name => {
            if (name.trim() === "")
                return FavoritesStore.errEmpty;
            if (FavoritesStore.isProtected(name) || names.includes(name.trim()))
                return FavoritesStore.errDuplicate;
            askKey(name);
            return "";
        });
        root._openPrompt({ kind: root.promptPick, message: root.titleAddToCollection, options: names.concat([root.optionNewCollection]) },
                         index => index < names.length ? askKey(names[index]) : askName());
    }

    function _add() {
        if (!root._guardWritable())
            return;
        if (ClipboardState.collection === "")
            root._askText(root.titleNewCollection, "", false, root.placeholderCollection, name => FavoritesStore.addCollection(name));
        else
            root._addEntry();
    }

    function _addEntry() {
        const collection = ClipboardState.collection;
        root._askText(root.titleKeyName, "", false, root.placeholderKey, key => {
            if (key.trim() === "")
                return FavoritesStore.errEmpty;
            if (root._hasEntry(collection, key))
                return FavoritesStore.errDuplicate;
            root._askText(root.titleValuePrefix + key.trim() + root.quoteSuffix, "", false, root.placeholderValue, value => FavoritesStore.addEntry(collection, key, value, false));
            return "";
        });
    }

    function _rename() {
        const source = ClipboardState.selectedSource;
        if (!source || !root._guardWritable())
            return;
        const collection = ClipboardState.collection;
        if (collection === "" && FavoritesStore.isProtected(source.name))
            root._notify(root.noticeProtected);
        else if (collection === "")
            root._askText(root.titleRenameCollection, source.name, false, root.placeholderCollection, name => FavoritesStore.renameCollection(source.name, name));
        else
            root._askText(root.titleRenameEntry, source.key, false, root.placeholderKey, key => FavoritesStore.renameEntry(collection, source.key, key));
    }

    function _changeValue() {
        const source = ClipboardState.selectedSource;
        const collection = ClipboardState.collection;
        if (!source || collection === "" || !root._guardWritable())
            return;
        root._askText(root.titleValuePrefix + source.key + root.quoteSuffix, source.secret ? "" : source.value, source.secret, root.placeholderValue, value => FavoritesStore.changeValue(collection, source.key, value));
    }

    function _markSecret() {
        const source = ClipboardState.selectedSource;
        const collection = ClipboardState.collection;
        if (!source || collection === "" || !root._guardWritable())
            return;
        if (source.secret) {
            root._notify(root.noticeAlreadySecret);
            return;
        }
        root._askConfirm(root.titleSecretPrefix + source.key + root.titleSecretSuffix, () => {
            const error = FavoritesStore.markSecret(collection, source.key);
            if (error !== "")
                root._notify(root._noticeFor(error));
        });
    }

    readonly property var _handlers: ({
        moveDown: () => ClipboardState.moveSelection(1),
        moveUp: () => ClipboardState.moveSelection(-1),
        jumpFirst: () => ClipboardState.select(0),
        jumpLast: () => ClipboardState.select(ClipboardState.rowCount - 1),
        halfPageDown: () => ClipboardState.moveSelection(root.halfPage),
        halfPageUp: () => ClipboardState.moveSelection(-root.halfPage),
        copy: () => root._copySelected(),
        copyIndex: token => root._copyIndex(token),
        deleteEntry: () => root._deleteSelected(),
        addToFavorites: () => root._addToFavorites(),
        wipeHistory: () => root._wipeHistory(),
        enterInsert: () => { keymap.mode = root.modeInsert; },
        leaveInsert: () => { keymap.mode = root.modeNormal; },
        close: () => root.hide(),
        escape: () => root._escape(),
        nextSection: () => ClipboardState.cycleSection(1),
        prevSection: () => ClipboardState.cycleSection(-1),
        togglePreview: () => ClipboardState.togglePreview(),
        previewScrollDown: () => root.contentItem.scrollPreviewDown(),
        previewScrollUp: () => root.contentItem.scrollPreviewUp(),
        activate: () => root._activate(),
        leaveCollection: () => ClipboardState.closeCollection(),
        add: () => root._add(),
        rename: () => root._rename(),
        changeValue: () => root._changeValue(),
        markSecret: () => root._markSecret(),
        pickNext: () => root._movePick(1),
        pickPrev: () => root._movePick(-1),
        pickAccept: () => root._resolvePick(),
        pickCancel: () => root._closePrompt(),
        confirmAccept: () => root._resolveConfirm(true),
        confirmDecline: () => root._resolveConfirm(false),
        confirmToggle: () => root._toggleConfirm(),
        confirmActivate: () => root._resolveConfirm(root._pickIndex === root.confirmYes)
    })

    function _dispatch(action, token) {
        const handler = root._handlers[action];
        if (handler)
            handler(token);
    }

    IpcHandler {
        target: "toggleClip"

        function handle() { root.toggle(); }
        function wipe() { ClipboardHistory.wipe(); }
    }

    ClipboardBindings { id: bindings }

    ModalKeymap {
        id: keymap
        enabled: root._promptKind !== root.promptText
        tables: ({
            [root.modeNormal]: root._favorites ? bindings.normalFavorites : bindings.normalHistory,
            [root.modeInsert]: bindings.insert,
            [root.modePick]: bindings.pick,
            [root.modeConfirm]: bindings.confirm
        })
        onActionTriggered: (action, token) => root._dispatch(action, token)
        onModeChanged: root._restoreFocus()
    }

    Connections {
        target: ClipboardState
        function onPromptChanged() {
            if (ClipboardState.prompt === null) {
                root._pending = null;
                if (keymap.mode === root.modePick || keymap.mode === root.modeConfirm)
                    keymap.mode = root.modeNormal;
                root._restoreFocus();
            } else if (root._promptKind === root.promptPick) {
                keymap.mode = root.modePick;
            } else if (root._promptKind === root.promptConfirm || root._promptKind === root.promptNotice) {
                keymap.mode = root.modeConfirm;
            }
        }
    }

    contentComponent: Component {
        Item {
            function focusFilter() {
                contentPane.focusFilter();
            }

            function focusPrompt() {
                promptPopup.focusInput();
            }

            function scrollPreviewDown() {
                contentPane.scrollPreviewDown();
            }

            function scrollPreviewUp() {
                contentPane.scrollPreviewUp();
            }

            implicitWidth: Theme.Tokens.comp.clip.contentW
            implicitHeight: layout.implicitHeight
            width: implicitWidth
            height: implicitHeight
            focusPolicy: Qt.StrongFocus

            // The FilterBar routes INSERT keys itself; routing here too would double-fire.
            // Swallowing unhandled keys during a text prompt stops Tab from moving focus off its input.
            Keys.onPressed: event => {
                if (root._promptKind === root.promptText) {
                    event.accepted = true;
                    if (event.key === Qt.Key_Escape)
                        root._closePrompt();
                    else if (event.key !== Qt.Key_Tab && event.key !== Qt.Key_Backtab) {
                        promptPopup.focusInput();
                        const commandModifiers = Qt.ControlModifier | Qt.AltModifier | Qt.MetaModifier;
                        if (event.text !== "" && !(event.modifiers & commandModifiers))
                            promptPopup.typeText(event.text);
                    }
                    return;
                }
                if (keymap.mode !== root.modeInsert)
                    keymap.handleKey(event);
            }

            ColumnLayout {
                id: layout
                anchors.fill: parent
                spacing: 0

                RowLayout {
                    Layout.fillWidth: true
                    spacing: 0

                    SectionSidebar {
                        Layout.preferredWidth: Theme.Tokens.comp.clip.sidebarW
                        Layout.fillHeight: true
                        items: [{ icon: ClipboardState.iconHistory }, { icon: ClipboardState.iconFavorites }]
                        currentIndex: root._favorites ? 1 : 0
                    }

                    QsSeparator {
                        Layout.fillHeight: true
                        Layout.preferredHeight: 0
                        lengthRatio: 1
                    }

                    ContentPane {
                        id: contentPane
                        Layout.fillWidth: true
                        keymap: root.panelKeymap
                    }
                }

                WarningBanner {
                    Layout.fillWidth: true
                    warning: root._favorites ? FavoritesStore.warning : ""
                }
            }

            PromptPopup {
                id: promptPopup
                anchors.fill: parent
                z: 1
                prompt: ClipboardState.prompt
                pickIndex: root._pickIndex
                confirmYesIndex: root.confirmYes
                onConfirmed: accepted => root._resolveConfirm(accepted)
                onAccepted: text => root._acceptText(text)
                onCancelled: root._closePrompt()
            }
        }
    }
}
