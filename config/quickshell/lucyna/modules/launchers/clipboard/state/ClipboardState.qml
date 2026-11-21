pragma Singleton

import QtQuick
import Quickshell

/*!
    ClipboardState — view model of the clipboard panel.

    Holds the active section, per-section filter and selection, the Favorites
    collection drill-in, the preview toggle (hidden on every reset) and the
    prompt descriptor, and exposes the filtered rows as uniform
    { kind, title, subtitle, masked, imageId } objects.
    All IO is delegated to ClipboardHistory and FavoritesStore.
*/
Singleton {
    id: root

    readonly property string sectionHistory:   "history"
    readonly property string sectionFavorites: "favorites"
    readonly property string iconHistory:      "\uf0ea"
    readonly property string iconFavorites:    "\uf005"

    readonly property string kindHistory:    "history"
    readonly property string kindCollection: "collection"
    readonly property string kindEntry:      "entry"

    readonly property string maskText: "••••"
    readonly property int _rowTextMaxChars: 200

    readonly property var _sections: [sectionHistory, sectionFavorites]

    property string section:    sectionHistory
    // Name of the open Favorites collection, "" at the collection list.
    property string collection: ""
    // Active prompt descriptor set by the panel, null when none.
    property var    prompt:     null
    property bool   previewVisible: false

    // The preview never shows on the Favorites collection list, whatever previewVisible says.
    readonly property bool   previewAvailable: !(section === sectionFavorites && collection === "")
    readonly property bool   previewShown: previewVisible && previewAvailable

    readonly property string filter: _filters[section]
    // Clamped on read, so delete/refresh never leaves it out of range.
    readonly property int    selectedIndex: _clamp(_selections[section])
    readonly property int    rowCount: _items.length
    // Source object behind the selected row: a History entry, a collection or a Favorites entry.
    readonly property var    selectedSource: _items.length > 0 ? _items[selectedIndex].source : null
    readonly property string positionText: rowCount > 0 ? (selectedIndex + 1) + "/" + rowCount : ""
    readonly property alias  rows: rowsModel

    property var _filters:    ({ history: "", favorites: "" })
    property var _selections: ({ history: 0,  favorites: 0 })

    readonly property var _items: _buildItems(section, collection, filter,
                                              ClipboardHistory.entries, FavoritesStore.collections)

    ScriptModel {
        id: rowsModel
        // Default structural comparison: unchanged rows keep their delegate (thumbnails), changed rows are replaced.
        values: root._items.map(item => root._toRow(item))
    }

    // Keeps `collection` in step when the open collection is renamed or deleted.
    Connections {
        target: FavoritesStore
        function onCollectionsChanged() {
            if (root.collection !== "" && !FavoritesStore.collections.some(c => c.name === root.collection))
                root.collection = ""
        }
    }

    // ── Public API ───────────────────────────────────────

    function reset() {
        section = sectionHistory
        collection = ""
        prompt = null
        previewVisible = false
        _filters = ({ history: "", favorites: "" })
        _selections = ({ history: 0, favorites: 0 })
        _lastSyncedId = null
        _syncPreview()
    }

    function togglePreview() {
        if (previewAvailable) previewVisible = !previewVisible
    }

    function setFilter(text) {
        _filters = Object.assign({}, _filters, { [section]: text })
        _selections = Object.assign({}, _selections, { [section]: 0 })
    }

    function select(index) {
        _selections = Object.assign({}, _selections, { [section]: _clamp(index) })
    }

    function moveSelection(delta) {
        select(selectedIndex + delta)
    }

    function cycleSection(step) {
        const count = _sections.length
        section = _sections[(_sections.indexOf(section) + step % count + count) % count]
    }

    function openCollection(name) {
        if (section !== sectionFavorites) return
        collection = name
        setFilter("")
    }

    // Selects the collection just left, like vim's `h` in a file tree.
    function closeCollection() {
        if (collection === "") return
        const left = collection
        openCollection("")
        select(FavoritesStore.collections.findIndex(c => c.name === left))
    }

    // ── Private ──────────────────────────────────────────

    function _clamp(index) {
        return Math.max(0, Math.min(index, _items.length - 1))
    }

    function _matches(text, query) {
        return String(text).toLowerCase().includes(query.toLowerCase())
    }

    function _buildItems(currentSection, currentCollection, currentFilter, historyEntries, collections) {
        if (currentSection === sectionHistory)
            return historyEntries.filter(entry => _matches(entry.title, currentFilter))
                .map(entry => ({ kind: kindHistory, source: entry }))

        const open = collections.find(c => c.name === currentCollection)
        if (!open)
            return collections.filter(c => _matches(c.name, currentFilter))
                .map(c => ({ kind: kindCollection, source: c }))

        return open.entries.filter(entry => _matches(entry.key, currentFilter))
            .map(entry => ({ kind: kindEntry, source: entry }))
    }

    function _singleLine(text) {
        return String(text).slice(0, _rowTextMaxChars).replace(/\s+/g, " ")
    }

    function _toRow(item) {
        const source = item.source
        if (item.kind === kindHistory)
            return { kind: item.kind, title: _singleLine(source.title), subtitle: "", masked: false,
                     imageId: source.isImage ? source.id : "" }
        if (item.kind === kindCollection)
            return { kind: item.kind, title: _singleLine(source.name), subtitle: String(source.entries.length), masked: false, imageId: "" }
        return { kind: item.kind, title: _singleLine(source.key), subtitle: source.secret ? maskText : _singleLine(source.value),
                 masked: source.secret, imageId: "" }
    }

    // Null forces the next sync; otherwise a refresh that keeps the same selected id is skipped.
    property var _lastSyncedId: null

    onSelectedSourceChanged: _syncPreview()

    function _syncPreview() {
        const source = selectedSource && section === sectionHistory ? selectedSource : null
        const id = source ? source.id : ""
        if (id === _lastSyncedId) return
        _lastSyncedId = id
        if (source && source.isImage) {
            ClipboardHistory.requestPreview("")
            ClipboardHistory.requestDecode(id)
        } else {
            ClipboardHistory.requestPreview(id)
        }
    }
}
