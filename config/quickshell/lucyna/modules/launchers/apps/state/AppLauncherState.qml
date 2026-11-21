pragma Singleton

import QtQuick
import Quickshell
import "../../../../core/persistence"

/*!
    AppLauncherState — private state for the app launcher module.

    Launch frequency/recency ranking (persisted through JsonStore) and query
    filtering of desktop entries.
*/
Singleton {
    id: root

    property string query: ""
    property alias  filteredApps: appsModel

    readonly property var launchCounts: store.content.launchCounts
    readonly property var lastUsed:     store.content.lastUsed

    // JsonStore loads synchronously: the singleton is only created on first launcher open,
    // so the sort order must be ready before the ScriptModel first reads it.
    JsonStore {
        id: store
        path: Quickshell.statePath("apps/launch-counts.json")
        version: 1
        defaults: ({ launchCounts: {}, lastUsed: {} })
        validate: parsed => root._validate(parsed)
    }

    function launch(entry) {
        entry.execute()
        _recordLaunch(_appKey(entry))
    }

    function _appKey(entry) {
        return entry.id
            || entry.desktopId
            || entry.desktopFile
            || entry.execString
            || entry.command
            || entry.name
            || ""
    }

    // Returns the clean payload, or null when a field is present but not an object.
    function _validate(parsed) {
        const isRecord = v => v !== null && typeof v === "object" && !Array.isArray(v)
        if (!isRecord(parsed.launchCounts ?? {}) || !isRecord(parsed.lastUsed ?? {})) return null

        const positive = record => {
            const result = {}
            for (const key of Object.keys(record ?? {})) {
                const v = Number(record[key])
                if (Number.isFinite(v) && v > 0) result[key] = v
            }
            return result
        }
        return { launchCounts: positive(parsed.launchCounts), lastUsed: positive(parsed.lastUsed) }
    }

    // A locked store ignores the save: the app still launches but nothing is recorded.
    function _recordLaunch(appId) {
        if (!appId) return
        store.save({
            launchCounts: Object.assign({}, root.launchCounts, { [appId]: (root.launchCounts[appId] ?? 0) + 1 }),
            lastUsed:     Object.assign({}, root.lastUsed,     { [appId]: Date.now() })
        })
    }

    ScriptModel {
        id: appsModel
        values: {
            const q   = root.query.trim().toLowerCase()
            const limit = q === "" ? 20 : 30
            const apps  = DesktopEntries.applications
                ? [...DesktopEntries.applications.values]
                : []

            const filtered = q === ""
                ? apps
                : apps.filter(e =>
                    (e.name ? e.name.toLowerCase().includes(q) : false)
                    || (e.genericName ? e.genericName.toLowerCase().includes(q) : false)
                    || (e.keywords ? e.keywords.some(k => k.toLowerCase().includes(q)) : false)
                )

            return filtered.sort((a, b) => {
                const aKey = root._appKey(a), bKey = root._appKey(b)

                const countDiff = (root.launchCounts[bKey] ?? 0) - (root.launchCounts[aKey] ?? 0)
                if (countDiff !== 0) return countDiff

                const lastDiff = (root.lastUsed[bKey] ?? 0) - (root.lastUsed[aKey] ?? 0)
                if (lastDiff !== 0) return lastDiff

                return a.name.localeCompare(b.name)
            }).slice(0, limit)
        }
    }
}
