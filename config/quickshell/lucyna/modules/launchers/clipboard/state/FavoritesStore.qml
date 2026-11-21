pragma Singleton

import QtQuick
import Quickshell
import "../../../../core/persistence"

/*!
    FavoritesStore — persistent clipboard favorites. Owns the schema validation
    and the mutations; persistence and corruption lock are JsonStore's.
*/
Singleton {
    id: root

    readonly property string defaultCollection: "General"

    // [{ name, entries: [{ key, value, secret }] }] in user order. Starts with the default collection; it reaches disk with the first real mutation.
    readonly property var    collections: store.content.collections
    readonly property bool   readOnly:    store.readOnly
    readonly property string warning:     store.warning

    readonly property string errReadOnly:  "read-only"
    readonly property string errNotReady:  "not-ready"
    readonly property string errEmpty:     "empty"
    readonly property string errDuplicate: "duplicate"
    readonly property string errNotFound:  "not-found"
    readonly property string errProtected: "protected"

    JsonStore {
        id: store
        path: Quickshell.dataPath("clipboard/favorites.json")
        version: 1
        secure: true
        defaults: ({ collections: [{ name: root.defaultCollection, entries: [] }] })
        validate: parsed => root._validate(parsed)
    }

    // ── Public API ───────────────────────────────────────
    // Mutations return "" when applied, otherwise one of the err* constants.
    // Names and keys are trimmed before every lookup and comparison.

    function addEntry(collection, key, value, secret) {
        return _mutate(next => _addTo(next, collection, key, value, secret, false))
    }

    // Used by add-from-History: creates the collection when it is new.
    function addToCollection(collection, key, value) {
        return _mutate(next => _addTo(next, collection, key, value, false, true))
    }

    function isProtected(name) {
        return _clean(name) === defaultCollection
    }

    function renameEntry(collection, oldKey, newKey) {
        const key = _clean(newKey)
        return _mutate(next => {
            const entry = _entryOf(next, collection, oldKey)
            if (!entry) return errNotFound
            if (key === "") return errEmpty
            if (key !== entry.key && _entryOf(next, collection, key)) return errDuplicate
            entry.key = key
            return ""
        })
    }

    function markSecret(collection, key) {
        return _mutate(next => {
            const entry = _entryOf(next, collection, key)
            if (!entry) return errNotFound
            entry.secret = true
            return ""
        })
    }

    function addCollection(name) {
        const clean = _clean(name)
        return _mutate(next => {
            if (clean === "") return errEmpty
            if (_byName(next, clean)) return errDuplicate
            next.push({ name: clean, entries: [] })
            return ""
        })
    }

    function renameCollection(oldName, newName) {
        if (isProtected(oldName)) return errProtected
        const name = _clean(newName)
        return _mutate(next => {
            const target = _byName(next, oldName)
            if (!target) return errNotFound
            if (name === "") return errEmpty
            if (name !== target.name && _byName(next, name)) return errDuplicate
            target.name = name
            return ""
        })
    }

    function changeValue(collection, key, value) {
        return _mutate(next => {
            const entry = _entryOf(next, collection, key)
            if (!entry) return errNotFound
            entry.value = String(value)
            return ""
        })
    }

    function deleteEntry(collection, key) {
        return _mutate(next => {
            const entry = _entryOf(next, collection, key)
            if (!entry) return errNotFound
            const entries = _byName(next, collection).entries
            entries.splice(entries.indexOf(entry), 1)
            return ""
        })
    }

    function deleteCollection(name) {
        if (isProtected(name)) return errProtected
        return _mutate(next => {
            const target = _byName(next, name)
            if (!target) return errNotFound
            next.splice(next.indexOf(target), 1)
            return ""
        })
    }

    // Call BEFORE hiding the panel: on Wayland the clipboard is empty unless a Quickshell window has focus.
    function copy(value) {
        Quickshell.clipboardText = value
    }

    // ── Load ─────────────────────────────────────────────

    function _withDefaultCollection(parsed) {
        const stored = parsed.find(c => c.name === defaultCollection)
        const rest = parsed.filter(c => c.name !== defaultCollection)
        return [stored || { name: defaultCollection, entries: [] }].concat(rest)
    }

    // JsonStore validate: returns { collections } or null when the file content is unusable.
    function _validate(parsed) {
        if (!Array.isArray(parsed.collections)) return null

        // Unknown fields are dropped on rebuild; only the schema is preserved.
        const result = []
        const names = new Set()
        for (const c of parsed.collections) {
            if (!c || typeof c.name !== "string" || !Array.isArray(c.entries)) return null
            const name = _clean(c.name)
            if (names.has(name)) return null
            names.add(name)

            const entries = []
            const keys = new Set()
            for (const e of c.entries) {
                if (!e || typeof e.key !== "string" || typeof e.value !== "string") return null
                if (e.secret !== undefined && typeof e.secret !== "boolean") return null
                const key = _clean(e.key)
                if (keys.has(key)) return null
                keys.add(key)
                entries.push({ key: key, value: e.value, secret: e.secret === true })
            }
            result.push({ name: name, entries: entries })
        }
        return { collections: _withDefaultCollection(result) }
    }

    // ── Mutation helpers ─────────────────────────────────

    // apply(next) returns "" on success or an err* constant. A no-op result skips the save.
    function _mutate(apply) {
        if (readOnly) return errReadOnly
        if (!store.ready) return errNotReady
        const next = JSON.parse(JSON.stringify(collections))
        const error = apply(next)
        if (error !== "") return error
        if (JSON.stringify(next) === JSON.stringify(collections)) return ""
        store.save({ collections: next })
        return ""
    }

    function _clean(text) {
        return String(text).trim()
    }

    function _byName(list, name) {
        const wanted = _clean(name)
        return list.find(c => c.name === wanted) || null
    }

    function _entryOf(list, collection, key) {
        const target = _byName(list, collection)
        const wanted = _clean(key)
        return target ? target.entries.find(e => e.key === wanted) || null : null
    }

    function _addTo(list, collection, key, value, secret, createCollection) {
        const entryKey = _clean(key)
        if (entryKey === "") return errEmpty

        let target = _byName(list, collection)
        if (!target) {
            if (!createCollection) return errNotFound
            const name = _clean(collection)
            if (name === "") return errEmpty
            target = { name: name, entries: [] }
            list.push(target)
        }
        if (target.entries.some(e => e.key === entryKey)) return errDuplicate

        target.entries.push({ key: entryKey, value: String(value), secret: secret === true })
        return ""
    }
}
