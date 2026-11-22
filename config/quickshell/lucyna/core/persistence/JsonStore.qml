import QtQuick
import Quickshell
import Quickshell.Io

/*!
    JsonStore — generic versioned JSON file store.

    The consumer supplies path, version, defaults, validate and secure; the store
    adds the version envelope, atomic coalesced writes and corruption protection.
    A file that exists but cannot be read, parsed, version-matched or validated is
    never overwritten: the store turns read-only and exposes a warning instead.
    A secure store whose directory could not be prepared is locked the same way.
*/
Scope {
    id: root

    // Absolute: FileView does not expand "~"; build it from statePath()/dataPath().
    required property string path
    required property int    version
    property var             defaults: ({})
    // (parsed) -> clean payload | null
    property var             validate: parsed => parsed
    property bool            secure:   false

    // Payload without the version field.
    // Named content, not data: "data" is Scope's default property.
    // Deep copy so defaults are never mutated; the first assignment replaces this binding.
    property var    content:  JSON.parse(JSON.stringify(defaults))
    property bool   ready:    false
    property bool   readOnly: false
    property string warning:  ""

    readonly property string _dirMode:  "700"
    readonly property string _fileMode: "600"
    readonly property string _dir:      path.substring(0, path.lastIndexOf("/"))

    // FileView creates missing parent directories itself; only secure stores need the init step (mode 700).
    property bool   _dirReady: !secure
    property var    _pending:  null

    CoalescedWriter {
        id: writer
        path: root.path
        onDone: root._onWritten()
        onFailed: reason => root._warn(reason)
    }

    FileView {
        id: reader
        watchChanges: false
        blockLoading: true
        printErrors: false
        onLoadFailed: error => root._onLoadFailed(error)
    }

    Process {
        id: initProc
        property bool started: false
        onStarted: started = true
        // A failed spawn emits neither started nor exited, only running -> false.
        onRunningChanged: if (!running && !started) root._initFailed("could not start")
        onExited: exitCode => {
            if (exitCode !== 0) {
                root._initFailed("exit " + exitCode)
                return
            }
            root._dirReady = true
            if (root._pending !== null) writer.write(root._pending)
            root._pending = null
        }
    }

    Process {
        id: chmodProc
        onExited: exitCode => {
            if (exitCode === 0) root._clearWarning()
            else root._warn("could not restrict permissions of " + root.path + " (exit " + exitCode + ")")
        }
    }

    // Readiness comes only from a known load outcome (loaded text or FileNotFound), never from an empty read: an unreadable file may report its error after construction.
    Component.onCompleted: {
        if (!path.startsWith("/")) {
            _lock("path must be absolute: " + path)
            return
        }
        if (secure) {
            // umask first so the directory is never group/world accessible, even briefly.
            initProc.exec({ command: ["sh", "-c", 'umask 077; mkdir -p "$1"; chmod "$2" "$1"', "sh", _dir, _dirMode] })
        }
        reader.path = path
        const text = reader.text()
        if (!readOnly && reader.loaded) _onLoaded(text)
    }

    // Returns false (no write, no content change) when locked.
    function save(payload) {
        if (readOnly) return false
        content = payload
        const text = JSON.stringify(Object.assign({ version: version }, payload), null, 2)
        if (_dirReady) writer.write(text)
        else _pending = text
        return true
    }

    function _onWritten() {
        if (!secure) {
            _clearWarning()
            return
        }
        chmodProc.exec({ command: ["chmod", _fileMode, path] })
    }

    // Sticky: readOnly blocks every later save, so no successful save can clear it.
    function _initFailed(detail) {
        _pending = null
        _lock("could not prepare directory " + _dir + " (" + detail + "), store is read-only")
    }

    // Only save-failure warnings are clearable; a lock keeps its message.
    function _clearWarning() {
        if (!readOnly) warning = ""
    }

    function _onLoaded(text) {
        let parsed
        try {
            parsed = JSON.parse(text)
        } catch (e) {
            _lock(path + " is not valid JSON, edit or remove it to continue")
            return
        }
        if (!parsed || typeof parsed !== "object" || Array.isArray(parsed)) {
            _lock(path + " does not hold a JSON object, edit or remove it to continue")
            return
        }
        if (parsed.version !== version) {
            _lock(path + " has version " + parsed.version + ", expected " + version)
            return
        }
        const clean = validate(parsed)
        if (clean === null || clean === undefined) {
            _lock(path + " failed validation, edit or remove it to continue")
            return
        }
        content = clean
        ready = true
    }

    function _onLoadFailed(error) {
        if (error === FileViewError.FileNotFound) {
            ready = true
            return
        }
        _lock(path + " could not be read (" + FileViewError.toString(error) + ")")
    }

    function _lock(message) {
        readOnly = true
        ready = false
        _warn(message)
    }

    function _warn(message) {
        warning = message
        console.warn("JsonStore: " + message)
    }
}
