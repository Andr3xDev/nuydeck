import QtQuick

/*!
    Runs a scheduled callback once the host panel's close animation has
    fully settled (`progress` reaches exactly 0; MotionAnimation ends on the
    target value). Declare it at the panel root, never inside a Loader gated
    on `progress`, or the close would destroy it before it fires.

    `settleDelay` adds a margin after progress hits 0, so a screen capture
    never grabs the panel's last frame. Scheduling while progress is already
    0 runs immediately (after `settleDelay`).
*/
Item {
    id: root

    required property real progress
    property int settleDelay: 0

    property var _pending: null

    visible: false

    function schedule(fn) {
        root._pending = fn;
        if (root.progress === 0)
            root._settle();
    }

    function cancel() {
        settleTimer.stop();
        root._pending = null;
    }

    function _settle() {
        if (!root._pending)
            return;
        if (root.settleDelay > 0)
            settleTimer.restart();
        else
            root._run();
    }

    function _run() {
        const fn = root._pending;
        root._pending = null;
        if (fn)
            fn();
    }

    onProgressChanged: {
        if (root.progress === 0)
            root._settle();
    }

    Timer {
        id: settleTimer
        interval: root.settleDelay
        onTriggered: root._run()
    }
}
