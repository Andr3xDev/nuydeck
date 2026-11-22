import QtQuick
import QtQuick.Layouts
import "../../../core/theme" as Theme
import "../../../core/services/system" as Services
import "../layout"
import "../../../core/components/atomic"
import "../../../core/animations" as Animations

/*!
    CPU and GPU temperature readouts, colored by severity.
*/
ExpandableRow {
    id: root

    function getTempColor(temp) {
        if (temp >= 80) return Theme.Tokens.severity("critical")
        if (temp >= 70) return Theme.Tokens.severity("warn")
        return Theme.Tokens.color.aquamarine
    }
    
    RowLayout {
        id: tempsRow
        anchors.centerIn: parent
        spacing: 10  // raw literal: a spacing token here caused a layout shift
        opacity: root.expanded ? 1 : 0
        
        Behavior on opacity {
            Animations.MotionAnimation { role: "stateStandard" }
        }
        
        RowLayout {
            visible: Services.TemperatureService.cpuTemp > 0
            spacing: Theme.Tokens.space.xs
            
            QsText {
                text: "󰍛"
                role: "icon"
                color: root.getTempColor(Services.TemperatureService.cpuTemp)
            }

            QsText {
                text: `${Math.round(Services.TemperatureService.cpuTemp)}°`
                role: "mono"
                color: Theme.Tokens.color.textPrimary
            }
        }
        
        RowLayout {
            visible: Services.TemperatureService.hasGPU
            spacing: Theme.Tokens.space.xs
            
            QsText {
                text: "󰾲"
                role: "icon"
                color: root.getTempColor(Services.TemperatureService.gpuTemp)
            }

            QsText {
                text: `${Math.round(Services.TemperatureService.gpuTemp)}°`
                role: "mono"
                color: Theme.Tokens.color.textPrimary
            }
        }
    }
}