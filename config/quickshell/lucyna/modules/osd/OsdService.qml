pragma Singleton

import QtQuick

/*!
    OsdService — singleton arbiter for the OSD module.

    Holds at most one active request record:
        { id: string, priority: int, timeout: int, view: Component }
      id       — stable identity of the OSD kind ("volume", "capslock")
      priority — int, higher wins, default 0
      timeout  — auto-hide ms; <= 0 → window default (1500 ms)
      view     — Component rendered by the window's Loader

    The service only arbitrates and notifies; it never draws, animates,
    positions anything, or holds a view instance. Signals are the contract:
    `activated` (first show or replace) and `refreshed` (same-id re-request).
    The window calls `hidden()` after its hide animation completes, which
    clears the active record.
*/
QtObject {
    id: root

    /*!
        Emitted when a request became active — first show OR replace.
        The window mounts/swaps the view and starts the fade in.
    */
    signal activated(var request)

    /*!
        Emitted when a request with the same id as the active one re-requests.
        The window restarts its auto-hide timer ONLY; the view instance is
        untouched and keeps updating through its own binding.
    */
    signal refreshed(var request)

    /*!
        The active request record, or null. Exposed for
        debuggability/introspection only — nothing in the framework binds to
        it; the signals are the contract.
    */
    readonly property var activeRequest: _active

    property var _active: null

    /*!
        Submit an OSD request. Same id → refreshed only, no view recreation;
        different id, lower priority → silently discarded.
    */
    function request(id, priority, timeout, view) {
        const req = {
            id: id,
            priority: (priority ?? 0),
            timeout: (timeout ?? 0),
            view: view
        };
        if (_active === null) {
            _active = req;
            activated(req);
            return;
        }
        if (id === _active.id) {
            _active = req;
            refreshed(req);
            return;
        }
        if (req.priority >= _active.priority) {
            _active = req;
            activated(req);
            return;
        }
        // else: discard — no signal, no state change
    }

    /*!
        Called by the window after its hide animation completed.
        Clears the active record; the next request of any id then activates.
    */
    function hidden() {
        _active = null;
    }
}
