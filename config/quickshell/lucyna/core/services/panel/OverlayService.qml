pragma Singleton

import QtQuick
import Quickshell

/*!
    Global FILO registry of modal overlay consumers, one stack, per-screen
    views. claim({ name, screen, close, focus? }) pushes (re-claiming moves to
    top); release(name) removes and gives the screen's new top its focus hook;
    raise(name) moves to top and focuses. QuickshellTop calls closeTopOn(screen)
    on scrim click. PanelService reads topOn/consumersOn for focus moves.
    _stack is reassigned, never mutated, so bindings re-evaluate (same as
    PanelService._panels).
*/
Singleton {
    id: root

    // Deepest-first: index 0 is the oldest claim, last is the topmost.
    property var _stack: []

    function claim(consumer) {
        const reclaimer = root._stack.some(function (c) { return c.name === consumer.name })
        root._stack = reclaimer
            ? root._stack.filter(function (c) { return c.name !== consumer.name }).concat([consumer])
            : root._stack.concat([consumer])
    }

    function release(name) {
        const victim = root._stack.find(function (c) { return c.name === name })
        if (!victim)
            return
        const wasTopOnScreen = root.topOn(victim.screen) === victim
        root._stack = root._stack.filter(function (c) { return c.name !== name })
        if (wasTopOnScreen) {
            const newTop = root.topOn(victim.screen)
            if (newTop && newTop.focus)
                newTop.focus()
        }
    }

    // Moves the claim to the top of the stack and hands it keyboard focus.
    function raise(name) {
        const claim = root._stack.find(function (c) { return c.name === name })
        if (!claim)
            return
        root.claim(claim)
        if (claim.focus)
            claim.focus()
    }

    function consumersOn(screen) {
        return root._stack.filter(function (c) { return c.screen === screen })
    }

    // The topmost (last claimed) consumer on this screen, or null.
    function topOn(screen) {
        const onScreen = root.consumersOn(screen)
        return onScreen.length > 0 ? onScreen[onScreen.length - 1] : null
    }

    function closeTopOn(screen) {
        const top = root.topOn(screen)
        if (top)
            top.close()
    }
}
