import QtQuick
import Quickshell
import Quickshell.Io

/*!
    CoalescedWriter — atomic text writes to one file, one in flight at a time.

    write() calls made while a write is in flight collapse to the last text
    (FileView silently drops an overlapping setText). A write whose text equals
    the live file content is skipped: FileView emits no signal for it, which
    would leave the queue stuck. The live content is re-read from disk before
    every write, so external edits never leave a stale comparison.

    done() fires once the queue is idle after a successful or skipped write;
    failed(reason) fires for each failed save.
*/
Scope {
    id: root

    // Absolute: FileView does not expand "~".
    required property string path

    readonly property bool idle: !_busy && !_dirty

    signal done()
    signal failed(string reason)

    property bool   _busy:     false
    property bool   _dirty:    false
    property string _latest:   ""
    property string _inFlight: ""

    readonly property bool _pathValid: path.startsWith("/")

    Component.onCompleted: {
        if (!_pathValid) console.warn("CoalescedWriter: path must be absolute, writes are refused: " + path)
    }

    FileView {
        id: file
        path: root._pathValid ? root.path : ""
        // Keep the default preload: with preload false, reload() emits neither loaded nor loadFailed and the queue would hang.
        watchChanges: false
        blockLoading: true
        printErrors: false
        onLoaded: root._commit(file.text())
        onLoadFailed: error => root._onLoadFailed(error)
        onSaved: {
            root._busy = false
            // Deferred: FileView logs "dropped operation" when a new operation starts inside a handler.
            Qt.callLater(root._settle)
        }
        onSaveFailed: error => {
            root._busy = false
            root.failed("could not save " + root.path + " (" + FileViewError.toString(error) + ")")
            Qt.callLater(root._retry)
        }
    }

    // Content of the first (blocking) load; not live afterwards.
    function loadedText() {
        return file.text()
    }

    function write(text) {
        if (!_pathValid) {
            failed("path is not absolute: " + path)
            return
        }
        _latest = text
        _dirty = true
        _flush()
    }

    function _flush() {
        if (_busy || !_dirty) return
        _dirty = false
        _busy = true
        _inFlight = _latest
        // Async: text() right after reload() is still the old content, so compare in onLoaded.
        file.reload()
    }

    function _commit(liveText) {
        if (!_busy) return
        if (liveText === _inFlight) {
            _busy = false
            Qt.callLater(_settle)
            return
        }
        // Deferred: setText inside a FileView handler is dropped.
        Qt.callLater(() => file.setText(_inFlight))
    }

    // Only a missing file counts as empty; any other failure means the live content is unknown, so overwriting could destroy data.
    function _onLoadFailed(error) {
        if (error === FileViewError.FileNotFound) {
            _commit("")
            return
        }
        if (!_busy) return
        _busy = false
        _dirty = false
        failed("could not read " + path + " (" + FileViewError.toString(error) + "), write skipped")
    }

    function _settle() {
        if (_busy) return
        if (_dirty) _flush()
        else done()
    }

    function _retry() {
        if (!_busy && _dirty) _flush()
    }
}
