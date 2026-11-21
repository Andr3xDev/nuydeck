pragma Singleton

import QtQuick
import Quickshell
import Quickshell.Io

/*!
    ClipboardHistory — cliphist IO for the clipboard panel.

    Lists, copies, deletes and wipes history entries, decodes the selected
    text entry for the preview pane, and decodes image entries into a
    per-user runtime directory for thumbnails.
*/
Singleton {
    id: root

    // Entries as { id, title, isImage, rawLine }, most recent first.
    property var    entries:     []
    property string previewText: ""

    signal decoded(string id)

    readonly property string _runtimeDir: Quickshell.env("XDG_RUNTIME_DIR") || ""
    readonly property string _imageDir:   _runtimeDir ? _runtimeDir + "/lucyna-clipboard" : ""
    readonly property bool   thumbnailsEnabled: _imageDir !== ""

    property var    _listBuffer:      []
    property bool   _refreshQueued:   false
    property string _wantedPreviewId: ""
    property string _activePreviewId: ""
    property bool   _previewBusy:     false
    property bool   _previewExited:   false
    property bool   _previewStreamed: false
    property string _previewResult:   ""
    property var    _removeQueue:     []
    property var    _decodeQueue:     []
    property var    _decodedIds:      ({})
    property string _decodingId:      ""

    Process {
        id: _listProc
        command: ["cliphist", "list"]
        stdout: SplitParser { onRead: data => root._parseLine(data) }
        onExited: {
            root.entries = root._listBuffer
            root._listBuffer = []
            if (root._refreshQueued) {
                root._refreshQueued = false
                root.refresh()
            }
        }
    }

    Process { id: _copyProc }

    Process {
        id: _removeProc
        onExited: {
            if (root._removeQueue.length > 0)
                root._pumpRemoveQueue()
            else
                root.refresh()
        }
    }

    Process {
        id: _wipeProc
        command: ["cliphist", "wipe"]
        onExited: root.refresh()
    }

    Process {
        id: _previewProc
        stdout: StdioCollector {
            onStreamFinished: {
                root._previewResult = text
                root._previewStreamed = true
                root._finishPreview()
            }
        }
        onExited: {
            root._previewExited = true
            root._finishPreview()
        }
    }

    Process {
        id: _decodeProc
        onExited: exitCode => root._onDecodeExited(exitCode)
    }

    Process {
        id: _cleanupProc
        onExited: root._pumpDecodeQueue()
    }

    Component.onCompleted: {
        if (!thumbnailsEnabled)
            console.warn("ClipboardHistory: XDG_RUNTIME_DIR unset, image thumbnails disabled")
        cleanup()
    }

    // ── Public API ───────────────────────────────────────

    function refresh() {
        if (_listProc.running) {
            _refreshQueued = true
            return
        }
        _listBuffer = []
        _listProc.running = true
    }

    function copy(id) {
        _copyProc.exec(["sh", "-c", 'cliphist decode "$1" | wl-copy', "sh", String(id)])
    }

    // Deletes run one at a time: exec() on a running Process kills the previous run.
    function remove(rawLine) {
        // Drop it now: the list only refreshes after the delete exits, and a quick second `dd` would hit the same entry.
        entries = entries.filter(e => e.rawLine !== rawLine)
        _removeQueue.push(rawLine)
        _pumpRemoveQueue()
    }

    function wipe() {
        if (!_wipeProc.running) _wipeProc.running = true
    }

    function requestPreview(id) {
        const wanted = id ? String(id) : ""
        if (wanted !== _wantedPreviewId) previewText = ""
        _wantedPreviewId = wanted
        if (_wantedPreviewId === "") return
        if (!_previewBusy) _startPreview()
    }

    function requestDecode(id) {
        if (!thumbnailsEnabled) return
        const key = String(id)
        if (_decodedIds[key] || _decodingId === key || _decodeQueue.indexOf(key) >= 0) return
        _decodeQueue.push(key)
        _pumpDecodeQueue()
    }

    function decodedPath(id) {
        const key = String(id)
        return _decodedIds[key] ? "file://" + _imageDir + "/" + key : ""
    }

    function cleanup() {
        _decodeQueue = []
        _decodedIds = ({})
        _decodingId = ""
        if (_decodeProc.running) _decodeProc.running = false
        if (!thumbnailsEnabled) return
        _cleanupProc.exec(["rm", "-rf", "--", _imageDir])
    }

    // ── Private ──────────────────────────────────────────

    function _parseLine(line) {
        if (!line.trim()) return
        const tab = line.indexOf('\t')
        if (tab < 0) return
        const id = line.substring(0, tab)
        if (!/^\d+$/.test(id)) return
        const title = line.substring(tab + 1).trim()
        _listBuffer.push({
            id:      id,
            title:   title,
            isImage: title.startsWith("[[") && title.includes("binary"),
            rawLine: line
        })
    }

    // The raw line travels in the environment: Process has no reliable stdin EOF
    // and argv would expose the clipboard content to `ps`.
    function _pumpRemoveQueue() {
        if (_removeProc.running || _removeQueue.length === 0) return
        _removeProc.exec({
            command: ["sh", "-c", 'printf "%s\\n" "$CLIP_LINE" | cliphist delete'],
            environment: ({ CLIP_LINE: _removeQueue.shift() })
        })
    }

    function _startPreview() {
        if (_wantedPreviewId === "") return
        _activePreviewId = _wantedPreviewId
        _previewBusy = true
        _previewExited = false
        _previewStreamed = false
        _previewProc.exec(["cliphist", "decode", _activePreviewId])
    }

    // Exit and stream-finished may arrive in either order; only when both
    // have fired is the run over and its text safe to attribute to its id.
    function _finishPreview() {
        if (!_previewExited || !_previewStreamed) return
        _previewBusy = false
        if (_activePreviewId === _wantedPreviewId)
            previewText = _previewResult
        else
            _startPreview()
    }

    // A killed decode exits late; never start the next one until it and the
    // async cleanup have finished, so a stale exit cannot hit a new run.
    function _pumpDecodeQueue() {
        if (_decodingId !== "" || _decodeProc.running || _cleanupProc.running
                || _decodeQueue.length === 0) return
        _decodingId = _decodeQueue.pop()
        _decodeProc.exec({
            command: ["sh", "-c", 'mkdir -p -m 700 "$2" && cliphist decode "$1" > "$2/$1"',
                      "sh", _decodingId, _imageDir]
        })
    }

    function _onDecodeExited(exitCode) {
        const id = _decodingId
        if (id === "") {
            _pumpDecodeQueue()
            return
        }
        _decodingId = ""
        if (exitCode === 0) {
            const next = Object.assign({}, _decodedIds)
            next[id] = true
            _decodedIds = next
            decoded(id)
        } else {
            console.warn("ClipboardHistory: image decode failed for id " + id)
        }
        _pumpDecodeQueue()
    }
}
