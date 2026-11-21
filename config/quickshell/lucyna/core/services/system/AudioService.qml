pragma Singleton

import QtQuick
import Quickshell
import Quickshell.Io
import Quickshell.Services.Pipewire

/*!
    Audio service manages volume state from the default system device to output sound

    Reads state instantaneously via Pipewire bindings; writes go through wpctl
*/
Singleton {
    id: root

    // Readiness: covers startup transient AND sink-switch transient null
    readonly property bool available: Pipewire.ready && Pipewire.defaultAudioSink != null

    PwObjectTracker {
        objects: [Pipewire.defaultAudioSink]  // officially tolerates nulls
    }

    // Public contract preserved — same names, same units, same semantics.
    // Real 0.0–~1.5 → int 0–150 (overdrive preserved, same as wpctl scale).
    readonly property int volume: Math.round(((Pipewire.defaultAudioSink ? (Pipewire.defaultAudioSink.audio ? Pipewire.defaultAudioSink.audio.volume : undefined) : undefined) ?? 0) * 100)
    readonly property bool muted: (Pipewire.defaultAudioSink ? (Pipewire.defaultAudioSink.audio ? Pipewire.defaultAudioSink.audio.muted : undefined) : undefined) ?? false

    // Fire-and-forget process template for one-shot commands
    property Component _fireAndForget: Component {
        Process { onExited: destroy() }
    }

    /*!
        Allow mute default device from a component
    */
    function toggleMute() {
        _fireAndForget.createObject(root, {
            running: true,
            command: ["wpctl", "set-mute", "@DEFAULT_AUDIO_SINK@", "toggle"]
        })
    }

    /*!
        Allow change volume level from 0 to 150 or the default value of the device
    */
    function changeVolume(step) {
        _fireAndForget.createObject(root, {
            running: true,
            command: ["wpctl", "set-volume", "@DEFAULT_AUDIO_SINK@", step]
        })
    }
}
