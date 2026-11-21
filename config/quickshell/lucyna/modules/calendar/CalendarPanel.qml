import "../../core/bridge" as Ipc
import "../../core/components/panel"
import "../../core/services/panel" as PanelServices
import QtQuick
import "../../core/components/input"
import "components"

/*!
    CalendarPanel — calendar panel hanging from the bar.
    Toggle: click on clock | qs -c lucyna ipc call panel toggle calendar
*/
PanelBase {
    id: root

    // Measured from the content's text-dependent layout; ZoneSlot warns when it drifts.
    readonly property size measuredSize: Qt.size(286, 318)

    name: "calendar"
    placement: PanelPlacement {
        type: PanelServices.PanelRules.Type.BarAttached
        shape: PanelServices.PanelRules.Shape.Wide
        size: root.measuredSize
    }
    focusTarget: root.contentItem
    contentComponent: Component {
        CalendarView {}
    }
    // Also covers reopening mid-close-animation, when the Loader item is still alive.
    onShown: {
        if (root.contentItem)
            root.contentItem.goToday();
    }
    onOpenChanged: Ipc.EventBus.calendarVisibilityChanged(root.open)

    // CalendarView doesn't handle keys, so they bubble up to here.
    Keys.onPressed: event => keymap.handleKey(event)

    ModalKeymap {
        id: keymap
        enabled: root.open
        useBase: true
        axis: "horizontal"
        modifierPolicy: "exactArrows"
        readonly property var handlers: ({
            moveLeft: () => root.contentItem?.prevMonth(),
            moveRight: () => root.contentItem?.nextMonth(),
            cancel: () => root.hide()
        })
        onActionTriggered: action => keymap.handlerFor(handlers, action)?.()
    }

    Connections {
        target: Ipc.EventBus
        function onCalendarToggleRequested() { root.toggle() }
    }
}
