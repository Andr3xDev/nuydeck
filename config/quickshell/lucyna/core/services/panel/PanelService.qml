pragma Singleton

import QtQuick
import Quickshell
import Quickshell.Io
import "../../bridge" as Ipc

/*!
    PanelService — single entry point for opening, closing, and toggling
    registered panels by name.

    Holds a name -> panel registry, populated by each panel's own
    Component.onCompleted/onDestruction. In-shell callers use
    PanelService.open()/.close()/.toggle() directly;
    an external caller (Hyprland keybind, `qs ipc call`) reaches the same
    functions through the single embedded IpcHandler, target "panel".

    Mutual exclusion is placement-driven (see makeRoom). close(name) closes the
    panel's whole descendant subtree, deepest-first. The registry is reassigned
    (Object.assign), never mutated in place, so bindings on it re-evaluate.

    A registered panel must expose: name, open, present, progress, parentName,
    show()/hide()/toggle(), plus (when placed) placement, host, zoneSettled,
    inputRect, scrimEnabled. Unplaced objects (the bar) omit placement.

    Usage:
        Services.PanelService.toggle("launcher")
        qs ipc call <configName> panel toggle launcher
*/
Singleton {
    id: root

    property var _panels: ({})

    function register(name, instance) {
        if (root._panels[name] !== undefined) {
            console.warn("PanelService: panel name '" + name + "' already registered, overwriting")
            root.unregister(name, root._panels[name].instance)
        }

        // No placement block means an unplaced object (bar, legacy panels): silent and valid.
        let valid = true
        if (instance.placement === null) {
            console.error("PanelService: " + name + ".placement: required but null")
            valid = false
        } else if (instance.placement !== undefined) {
            const result = PanelRules.validate(name, instance.placement)
            // makeRoom and the drain wait on zoneSettled; a zone panel without it would hang them. Floating panels are exempt.
            if (PanelRules.zoneOf(instance.placement) !== "" && instance.zoneSettled === undefined)
                result.errors.push("PanelService: " + name + ".zoneSettled: a panel with a zone must expose zoneSettled")
            result.errors.forEach(message => console.error(message))
            result.warnings.forEach(message => console.warn(message))
            valid = result.errors.length === 0
        }

        const onSettled = instance.zoneSettled === undefined ? null : function () { root._onZoneSettled(instance) }
        if (onSettled)
            instance.zoneSettledChanged.connect(onSettled)

        root._panels = Object.assign({}, root._panels, { [name]: { instance: instance, valid: valid, onSettled: onSettled } })

        // parentName may reference a panel that registers later in the same completion
        // burst; defer the existence check one event-loop tick so ordering settles.
        if (instance.parentName !== "") {
            const parentName = instance.parentName
            Qt.callLater(function () {
                if (root._panels[parentName] === undefined)
                    console.warn("PanelService: panel '" + name + "' references unknown parent '" + parentName + "'")
            })
        }
    }

    // Removes the registration only when `instance` still owns it: a destroyed old instance must not
    // remove the newer one registered under the same name.
    function unregister(name, instance) {
        const entry = root._panels[name]
        if (!entry || entry.instance !== instance)
            return
        if (entry.onSettled)
            instance.zoneSettledChanged.disconnect(entry.onSettled)
        root.cancelPending(name)
        if (root._openStamps[name] !== undefined) {
            const remainingStamps = Object.assign({}, root._openStamps)
            delete remainingStamps[name]
            root._openStamps = remainingStamps
        }

        const remainingPanels = Object.assign({}, root._panels)
        delete remainingPanels[name]
        root._panels = remainingPanels

        // A queued panel may wait on this one's zoneSettled, which will never fire again.
        if (instance.host && instance.host.screen)
            root._drainScreen(instance.host.screen.name)
    }

    function _getPanel(name, action) {
        const entry = root._panels[name]
        if (!entry)
            console.warn("PanelService: " + action + "() called for unregistered panel '" + name + "'")
        return entry ? entry.instance : undefined
    }

    function open(name) {
        const panel = root._getPanel(name, "open")
        if (!panel)
            return

        panel.show()
    }

    function close(name) {
        const panel = root._getPanel(name, "close")
        if (!panel)
            return

        root._closeSubtree(name)
    }

    function getInstance(name) {
        return root._panels[name] ? root._panels[name].instance : undefined
    }

    function barNameFor(screen) {
        return "bar-" + screen.name
    }

    // How far the bar's border centerline reaches down from the top of `screen`, following the bar's slide
    // (visualExtent is the resting value). 0 when the bar is missing or hidden. Reactive through _panels.
    function barOffset(screen) {
        const bar = root.getInstance(root.barNameFor(screen))
        return bar && bar.visible ? Math.max(0, bar.visualExtent - (1 - bar.progress) * bar.height) : 0
    }

    // The bar's extent once its slide has settled: side zones lay out against this, so they move once
    // when the bar opens or closes instead of retargeting every frame of its slide.
    function barRestingOffset(screen) {
        const bar = root.getInstance(root.barNameFor(screen))
        return bar && bar.open && bar.visible ? bar.visualExtent : 0
    }

    // Open-order stamps, name -> { order, seq }. order picks the side (lower first); seq is the age (lower is older).
    // Reassigned, never mutated, so zoneOffset bindings re-evaluate.
    property var _openStamps: ({})
    property int _stampCounter: 0

    // Queued open requests, zoneKey ("<screen name>|<zone>") -> { name, screen (name), zone, order, reserved, scrimHold }.
    property var _pending: ({})

    function _zoneAvailable(host, zone) {
        return PanelRules.isVerticalZone(zone) ? host.screen.height - root.barRestingOffset(host.screen) : host.screen.width
    }

    // Valid placed panels attached to `host`. Reads are tracked by the calling binding.
    function _placedOn(host) {
        return Object.values(root._panels).filter(function (registered) {
            return registered.valid && registered.instance.placement !== undefined && registered.instance.host === host
        }).map(function (registered) { return registered.instance })
    }

    // Centre offset along the zone axis for `panel`, from the open order of its zone peers.
    // Reads are tracked by the calling binding.
    function zoneOffset(panel) {
        const zone = PanelRules.zoneOf(panel.placement)
        const host = panel.host
        if (zone === "" || !host)
            return 0

        const reservedOrders = {}
        Object.values(root._pending).forEach(function (entry) {
            if (entry.reserved && entry.zone === zone && entry.screen === host.screen.name)
                reservedOrders[entry.name] = entry.order
        })

        const placed = root._placedOn(host)
        // A reserved peer counts before its host is set (its first open on this screen is still queued),
        // so the partner already makes room for it.
        const peers = Object.values(root._panels).map(function (registered) {
            return registered.instance
        }).filter(function (peer) {
            const reserved = reservedOrders[peer.name] !== undefined
            return peer === panel || ((placed.includes(peer) || reserved) && PanelRules.zoneOf(peer.placement) === zone
                && (peer.present || reserved))
        }).map(function (peer) {
            const stamp = root._openStamps[peer.name]
            const order = reservedOrders[peer.name] !== undefined ? reservedOrders[peer.name] : (stamp ? stamp.order : Infinity)
            return { peer: peer, order: order, extent: PanelRules.extentOf(peer.placement, zone) }
        }).sort(function (a, b) { return a.order < b.order ? -1 : (a.order > b.order ? 1 : 0) })

        const base = PanelRules.isVerticalZone(zone) ? root.barRestingOffset(host.screen) / 2 : 0
        if (peers.length === 1)
            return base

        const total = peers.reduce(function (sum, slot) { return sum + slot.extent }, 0) + PanelRules.zoneGap * (peers.length - 1)
        if (total > root._zoneAvailable(host, zone))
            return base

        let before = 0
        for (let i = 0; i < peers.length; i++) {
            if (peers[i].peer === panel)
                return base - total / 2 + before + peers[i].extent / 2
            before += peers[i].extent + PanelRules.zoneGap
        }
        return base
    }

    function isPending(name) {
        return Object.values(root._pending).some(function (entry) { return entry.name === name })
    }

    // Copy of _pending without the entries `shouldDrop(key, entry)` selects.
    function _pendingExcept(shouldDrop) {
        const remaining = {}
        Object.keys(root._pending).forEach(function (key) {
            if (!shouldDrop(key, root._pending[key]))
                remaining[key] = root._pending[key]
        })
        return remaining
    }

    function cancelPending(name) {
        if (!root.isPending(name))
            return
        root._pending = root._pendingExcept(function (key, entry) { return entry.name === name })
    }

    // Drain: a settled panel ends its own pending entry, then every queued panel on its screen retries show().
    // show() runs synchronously so a partner never renders an intermediate "alone" frame.
    function _onZoneSettled(panel) {
        if (!panel.zoneSettled)
            return
        if (panel.open && root.isPending(panel.name))
            root.cancelPending(panel.name)

        const host = panel.host
        if (!host)
            return
        root._drainScreen(host.screen.name)
    }

    function _drainScreen(screenName) {
        Object.values(root._pending).filter(function (entry) {
            return entry.screen === screenName
        }).forEach(function (entry) {
            const queued = root.getInstance(entry.name)
            if (queued && !queued.open)
                queued.show()
        })
    }

    function keyboardHeld(screen) {
        // Queued panels are modal but report needsKeyboard=false until open.
        return Object.values(root._pending).some(function (entry) {
            return entry.screen === screen.name
        })
    }

    function scrimHeld(screen) {
        return Object.values(root._pending).some(function (entry) {
            const incoming = root.getInstance(entry.name)
            return entry.screen === screen.name && entry.scrimHold && incoming && (!incoming.open || incoming.scrimEnabled)
        })
    }

    // Two ranking values within this distance count as equal; tolerates fractional rects mid-glide.
    readonly property real focusTieEpsilonPx: 0.5

    // Moves keyboard focus to the nearest open zone panel in `direction` ("left" | "down" | "up" | "right")
    // from the focused panel of `screen`. A missing target or an exact tie does nothing.
    function focusDirection(screen, direction) {
        const top = OverlayService.topOn(screen)
        const current = top ? root.getInstance(top.name) : undefined
        if (!current || current.placement === undefined || PanelRules.zoneOf(current.placement) === "")
            return

        const from = current.inputRect
        const horizontal = direction === "left" || direction === "right"
        const candidates = OverlayService.consumersOn(screen).filter(function (consumer) {
            return consumer !== top
        }).map(function (consumer) {
            return root.getInstance(consumer.name)
        }).filter(function (peer) {
            return peer && peer.placement !== undefined && peer.open && PanelRules.zoneOf(peer.placement) !== ""
        }).map(function (peer) {
            const to = peer.inputRect
            const gaps = {
                right: to.x - (from.x + from.width),
                left: from.x - (to.x + to.width),
                down: to.y - (from.y + from.height),
                up: from.y - (to.y + to.height)
            }
            const across = horizontal
                ? Math.abs((to.y + to.height / 2) - (from.y + from.height / 2))
                : Math.abs((to.x + to.width / 2) - (from.x + from.width / 2))
            return { name: peer.name, gap: gaps[direction], across: across }
        }).filter(function (candidate) { return candidate.gap >= 0 })

        const same = function (a, b) { return Math.abs(a - b) <= root.focusTieEpsilonPx }
        const ranksBefore = function (a, b) {
            return same(a.gap, b.gap) ? (!same(a.across, b.across) && a.across < b.across) : a.gap < b.gap
        }
        let best
        candidates.forEach(function (candidate) {
            if (best === undefined || ranksBefore(candidate, best))
                best = candidate
        })
        if (best === undefined)
            return
        const tied = candidates.some(function (candidate) {
            return candidate !== best && same(candidate.gap, best.gap) && same(candidate.across, best.across)
        })
        if (!tied)
            OverlayService.raise(best.name)
    }

    // Ray tone for `name`: "dim" when it is the only placed open panel on the keyboard-owning screen,
    // "bright" when it owns focus among several there, "none" otherwise.
    function focusRayTone(name) {
        const instance = root.getInstance(name)
        const host = instance ? instance.host : undefined
        if (!instance || !instance.open || !host || !host.screen || !Ipc.HyprlandIpc.isFocused(host.screen))
            return "none"

        const activeCount = root._placedOn(host).filter(function (peer) { return peer.open }).length
        if (activeCount < 2)
            return activeCount === 1 ? "dim" : "none"

        const top = OverlayService.topOn(host.screen)
        return top && top.name === name ? "bright" : "none"
    }

    function _childrenOf(name) {
        return Object.keys(root._panels).filter(function (candidateName) {
            return root._panels[candidateName].instance.parentName === name
        })
    }

    function _closeSubtree(name, visited = new Set()) {
        if (visited.has(name))
            return
        visited.add(name)
        root._childrenOf(name).forEach(function (childName) { root._closeSubtree(childName, visited) })

        const entry = root._panels[name]
        if (entry)
            entry.instance.hide()
    }

    function toggle(name) {
        const panel = root._getPanel(name, "toggle")
        if (!panel)
            return

        panel.toggle()
    }

    // Called by a panel's show() once its host is resolved. Returns false when the panel must not open
    // now: refused (invalid) or queued behind zone panels that are still moving. The drain re-runs show().
    function makeRoom(panel, host) {
        const entry = root._panels[panel.name]
        if (entry && entry.valid === false)
            return false

        // An exclusive panel (Power) and the zone panels close each other on a screen. The bar reports open but has no placement, so it is never evicted.
        const exclusive = PanelRules.isExclusive(panel.placement)
        root._placedOn(host).filter(function (peer) {
            return peer !== panel && peer.open && (exclusive || PanelRules.isExclusive(peer.placement))
        }).forEach(function (peer) { peer.hide() })

        if (exclusive) {
            root._pending = root._pendingExcept(function (key, pendingEntry) { return pendingEntry.screen === host.screen.name })
            return true
        }

        const zone = PanelRules.zoneOf(panel.placement)
        if (zone === "")
            return true
        const zoneKey = root._zoneKey(host, zone)

        // Reopen during own exit: nothing to evict, the exit reverses through the progress Behavior.
        if (panel.present && !panel.open) {
            if (root._pending[zoneKey] !== undefined)
                root._pending = root._pendingExcept(function (key) { return key === zoneKey })
            return true
        }

        // Read before _evict so its hides cannot change the entry _admitOrQueue carries.
        const queued = root._pending[zoneKey]
        const eviction = root._evict(panel, host, zone)
        const admission = root._admitOrQueue(panel, host, zone, eviction, queued)
        if (!admission.ready)
            return false

        root._recordOpenOrder(panel, admission.order)
        return true
    }

    function _zoneKey(host, zone) {
        return host.screen.name + "|" + zone
    }

    // Hides the panels `panel` displaces on its zone: its exclusion group, then the oldest peers until it fits.
    // Returns the surviving zone peers and what was hidden.
    function _evict(panel, host, zone) {
        const active = root._placedOn(host).filter(function (peer) { return peer !== panel && peer.present })
        const peers = active.filter(function (peer) { return PanelRules.zoneOf(peer.placement) === zone })
        const group = panel.placement.exclusionGroup
        const excluded = group === PanelRules.Group.None ? [] : active.filter(function (peer) {
            return PanelRules.zoneOf(peer.placement) !== "" && peer.placement.exclusionGroup === group
        })

        const orderOf = function (peer) { return root._openStamps[peer.name] ? root._openStamps[peer.name].order : undefined }
        const seqOf = function (peer) { return root._openStamps[peer.name] ? root._openStamps[peer.name].seq : 0 }
        const extentOf = function (peer) { return PanelRules.extentOf(peer.placement, zone) }

        const hidden = []
        let focusLeft = false
        const leave = function (victim) {
            const top = OverlayService.topOn(host.screen)
            if (top && top.name === victim.name)
                focusLeft = true
            victim.hide()
            hidden.push(victim)
        }

        let inheritedOrder
        excluded.forEach(function (peer) {
            if (peer.open)
                leave(peer)
            if (inheritedOrder === undefined && peers.includes(peer))
                inheritedOrder = orderOf(peer)
        })

        const keep = peers.filter(function (peer) { return peer.open && !excluded.includes(peer) })
            .sort(function (a, b) { return seqOf(a) - seqOf(b) })
        const available = root._zoneAvailable(host, zone)
        const fits = function () {
            const used = keep.reduce(function (sum, peer) { return sum + extentOf(peer) }, extentOf(panel))
            return used + PanelRules.zoneGap * keep.length <= available
        }
        while (keep.length > 0 && (keep.length >= PanelRules.zoneCap || !fits())) {
            const victim = keep.shift()
            if (inheritedOrder === undefined)
                inheritedOrder = orderOf(victim)
            leave(victim)
        }

        // The leaving panel's hide() released its claim; a newer consumer (Power, another zone) could take focus
        // from the surviving partner, so the partner is raised explicitly. keep is sorted oldest first.
        if (focusLeft && keep.length > 0)
            OverlayService.raise(keep[keep.length - 1].name)

        return { peers: peers, excluded: excluded, keep: keep, hidden: hidden, inheritedOrder: inheritedOrder }
    }

    // Decides whether `panel` opens now or waits in _pending, and the order it takes in the zone.
    // `queued` is the zone's pending entry as read before the eviction.
    function _admitOrQueue(panel, host, zone, eviction, queued) {
        const zoneKey = root._zoneKey(host, zone)
        const carried = queued !== undefined && queued.name === panel.name ? queued : undefined
        const peers = eviction.peers
        const excluded = eviction.excluded
        const keep = eviction.keep

        const closing = peers.concat(eviction.hidden).some(function (peer) { return peer.present && !peer.open })
        const reserved = keep.length >= 1 && !closing
        let order = carried !== undefined ? carried.order : undefined
        if (order === undefined)
            order = eviction.inheritedOrder
        if (order === undefined && reserved)
            order = ++root._stampCounter
        const request = {
            name: panel.name,
            screen: host.screen.name,
            zone: zone,
            order: order,
            reserved: reserved,
            scrimHold: (carried !== undefined && carried.scrimHold) || peers.concat(excluded).some(function (peer) { return peer.scrimEnabled })
        }
        const store = function () {
            root._pending = Object.assign(root._pendingExcept(function (key, other) {
                return key === zoneKey || other.name === panel.name
            }), { [zoneKey]: request })
        }

        // Stored before the wait check: a reservation moves the partner, which makes it unsettled.
        const stored = reserved || carried !== undefined
        if (stored)
            store()

        const waiting = peers.concat(excluded).some(function (peer) { return !peer.zoneSettled })
        if (waiting) {
            if (!stored)
                store()
            return { ready: false }
        }

        if (!stored && (queued !== undefined || root.isPending(panel.name)))
            root._pending = root._pendingExcept(function (key, other) { return key === zoneKey || other.name === panel.name })
        return { ready: true, order: order }
    }

    function _recordOpenOrder(panel, order) {
        const kept = root._openStamps[panel.name]
        const stampOrder = order !== undefined ? order : (kept ? kept.order : ++root._stampCounter)
        root._openStamps = Object.assign({}, root._openStamps, { [panel.name]: { order: stampOrder, seq: ++root._stampCounter } })
    }

    IpcHandler {
        target: "panel"

        function toggle(name: string): void { root.toggle(name) }
        function open(name: string): void { root.open(name) }
        function close(name: string): void { root.close(name) }
    }
}
