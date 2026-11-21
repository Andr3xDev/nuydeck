import QtQuick
import Quickshell
import "../../"
import "../../../../core/services/system" as Services

/*!
    VolumeTrigger — volume OSD pair's trigger half.

    Watches the AudioService state (proxied properties on the root avoid the
    Connections signal-mismatch pitfall when the sink is transiently
    undefined) and fires an arbiter request on every volume/muted change.
    Startup and default-sink switches produce service transients (0→N sync,
    0 on sink swap); those are swallowed by a silent baseline — handlers
    only fire once _primed is set AND AudioService.available, so the first
    real user-initiated change is the first OSD.
    The view stays a Component reference — the service stores no view
    instances; the window instantiates it in its own Loader.
*/
Scope {
    id: root

    // two levels up here: module root, where OsdService lives
    property Component view: Component { VolumeView {} }

    property int volume: Services.AudioService.volume
    property bool muted: Services.AudioService.muted
    property bool _primed: false
    onVolumeChanged: if (root._primed && Services.AudioService.available) root._request()
    onMutedChanged: if (root._primed && Services.AudioService.available) root._request()

    // One event-loop tick after Component.onCompleted: initial bindings have
    // settled by then, so _primed never flips inside a startup change batch.
    Component.onCompleted: Qt.callLater(function() { root._primed = true })

    function _request() {
        OsdService.request("volume", 0, 0, root.view)
    }
}
