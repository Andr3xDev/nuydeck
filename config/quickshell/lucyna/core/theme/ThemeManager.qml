pragma Singleton
import QtQuick
import Quickshell
import Quickshell.Io
import "../persistence"
import "./primitives/nuytek" as Primitives

/*!
    ThemeManager — Singleton facade for the design token system.

    Single import point for all consumers. Manages the active theme,
    persists name and variant as plain-text files in the nuydeck theme
    hub, and exposes the active palette's primitives via `p` for
    Tokens.qml to build the public token API on.
*/
QtObject {
    id: themeManager

    property string currentTheme: "nuytek"
    property string currentVariant: "nadir"

    readonly property var _themes: ({
        nuytek: {
            displayName: "Nuytek",
            defaultVariant: "nadir",
            variants: {
                nadir: Primitives.Nadir,
                zenit: Primitives.Zenit
            }
        }
    })

    readonly property var availableThemes: Object.keys(_themes)
    readonly property var availableVariants: Object.keys((_themes[currentTheme] ? _themes[currentTheme].variants : undefined) ?? {})

    // Emitted when every outstanding theme file write has finished (success, skipped or failure).
    signal persisted()

    // Theme hub shared with other tools (nuydeck scripts read these as plain text).
    property string _stateDir: Quickshell.env("HOME") + "/.config/nuydeck/theme"

    property bool _persistPending: false

    property CoalescedWriter _nameWriter: CoalescedWriter {
        path: themeManager._stateDir + "/name"
        onDone: themeManager._onWriterSettled()
        onFailed: reason => themeManager._onWriterFailed(reason)
    }

    property CoalescedWriter _variantWriter: CoalescedWriter {
        path: themeManager._stateDir + "/variant"
        onDone: themeManager._onWriterSettled()
        onFailed: reason => themeManager._onWriterFailed(reason)
    }

    Component.onCompleted: _loadFromDisk()

    // Own-property lookup: ids come from user-editable files, and plain indexing would match prototype keys ("constructor").
    function _own(object, key) {
        return Object.prototype.hasOwnProperty.call(object, key) ? object[key] : undefined
    }

    function _loadFromDisk() {
        const storedTheme = _nameWriter.loadedText().trim()
        const themeId = _own(_themes, storedTheme) ? storedTheme : "nuytek"
        const variantId = _variantWriter.loadedText().trim()
        currentTheme = themeId
        currentVariant = _own(_themes[themeId].variants, variantId)
            ? variantId
            : _themes[themeId].defaultVariant
    }

    function _writeValue(writer, value) {
        _persistPending = true
        writer.write(value + "\n")
    }

    function _onWriterFailed(reason) {
        console.warn("ThemeManager: " + reason)
        _onWriterSettled()
    }

    function _onWriterSettled() {
        if (!_persistPending || !_nameWriter.idle || !_variantWriter.idle) return
        _persistPending = false
        persisted()
    }

    readonly property QtObject p:
        (_themes[currentTheme] ? _themes[currentTheme].variants[currentVariant] : undefined) ?? Primitives.Nadir

    function setTheme(themeName) {
        if (!_own(_themes, themeName)) return false
        currentTheme = themeName
        currentVariant = _themes[themeName].defaultVariant
        _writeValue(_nameWriter, currentTheme)
        _writeValue(_variantWriter, currentVariant)
        return true
    }

    function setVariant(variantId) {
        const variants = (_themes[currentTheme] ? _themes[currentTheme].variants : undefined) ?? {}
        if (!_own(variants, variantId)) return false
        currentVariant = variantId
        _writeValue(_variantWriter, currentVariant)
        return true
    }

    function getThemeDisplayName(themeName) {
        const theme = _themes[themeName]
        return theme ? theme.displayName : themeName
    }

    function getVariantDisplayName(variantId) {
        const primitive = _themes[currentTheme] ? _themes[currentTheme].variants[variantId] : undefined
        return primitive ? primitive.name : variantId
    }

    // Swatch colors for any variant id, not only the active one.
    function previewColors(variantId) {
        const primitive = (_themes[currentTheme] ? _themes[currentTheme].variants[variantId] : undefined) ?? Primitives.Nadir
        return [primitive.neutral.bg, primitive.hue.turquoise.fill, primitive.hue.aquamarine.fill]
    }
}
