----------------------------------------------------------------------------------
--
--      ,---.          ,--.                    ,--.  ,--.
--     /  O  \ ,--,--, `--',--,--,--. ,--,--.,-'  '-.`--' ,---. ,--,--,  ,---.
--    |  .-.  ||      \,--.|        |' ,-.  |'-.  .-',--.| .-. ||      \(  .-'
--    |  | |  ||  ||  ||  ||  |  |  |\ '-'  |  |  |  |  |' '-' '|  ||  |.-'  `)
--    `--' `--'`--''--'`--'`--`--`--' `--`--'  `--'  `--' `---' `--''--'`----'
--    
--             _______
--　　　　　 /  ＞　　フ     
--　　　　　| 　_　 _ l      RESUME: Compositor curves & animations to make
--　 　　　／` ミ＿xノ               beautiful components
--　　 　 /　　　 　 |
--　　　 /　 ヽ　　 ﾉ
--　 　 │　　|　|　|         AUTHOR:  Andr3xDev
--　／￣|　　 |　|　|        URL: https://github.com/Andr3xDev/nuydeck
--　| (￣ヽ＿_ヽ_)__)
--　＼二つ
----------------------------------------------------------------------------------

----------------------------------------------------------------------------------
--- Curves
----------------------------------------------------------------------------------
hl.curve("expressiveEffectsDefault", { type = "bezier", points = { {0.34, 0.80}, {0.34, 1.00} } })
hl.curve("emphasizedDecel",          { type = "bezier", points = { {0.05, 0.7}, {0.1, 1} } })
hl.curve("standard",                 { type = "bezier", points = { {0.2, 0}, {0, 1} } })
hl.curve("fastOutSlowIn",            { type = "bezier", points = { {0.4, 0}, {0.2, 1} } })
hl.curve("expressiveDefaultSpatial", { type = "bezier", points = { {0.38, 1.21}, {0.22, 1.00} } })
hl.curve("springOpen",              { type = "spring", mass = 1, stiffness = 220, dampening = 26 })


----------------------------------------------------------------------------------
--- Animations
----------------------------------------------------------------------------------
-- Windows
hl.animation({ leaf = "windows",             enabled = true,  speed = 3,   bezier = "emphasizedDecel", style = "popin 85%" })
hl.animation({ leaf = "windowsIn",           enabled = true,  speed = 3,   bezier = "emphasizedDecel", style = "popin 85%" })
hl.animation({ leaf = "windowsOut",          enabled = true,  speed = 2,   bezier = "emphasizedDecel", style = "popin 90%" })
hl.animation({ leaf = "windowsMove",         enabled = true,  speed = 4,   bezier = "emphasizedDecel", style = "slide" })

-- Layers: mirror the Quickshell motion tokens; in practice only dunst animates (other layers have no_anim)
hl.animation({ leaf = "layers",              enabled = true,  speed = 3,   bezier = "expressiveEffectsDefault", style = "fade" })
hl.animation({ leaf = "layersIn",            enabled = true,  speed = 5,   bezier = "expressiveDefaultSpatial", style = "fade" })
hl.animation({ leaf = "layersOut",           enabled = true,  speed = 3,   bezier = "fastOutSlowIn",   style = "fade" })

-- Fade
hl.animation({ leaf = "fade",                enabled = true,  speed = 3.5, bezier = "expressiveEffectsDefault" })
hl.animation({ leaf = "fadeIn",              enabled = true,  speed = 3,   bezier = "emphasizedDecel" })
hl.animation({ leaf = "fadeOut",             enabled = true,  speed = 2,   bezier = "emphasizedDecel" })
hl.animation({ leaf = "fadeSwitch",          enabled = true,  speed = 3,   bezier = "expressiveEffectsDefault" })
hl.animation({ leaf = "fadeShadow",          enabled = true,  speed = 3,   bezier = "expressiveEffectsDefault" })
hl.animation({ leaf = "fadeDim",             enabled = true,  speed = 3,   bezier = "expressiveEffectsDefault" })
hl.animation({ leaf = "fadeLayers",          enabled = true,  speed = 3,   bezier = "expressiveEffectsDefault" })
hl.animation({ leaf = "fadeLayersIn",        enabled = true,  speed = 2,   bezier = "expressiveEffectsDefault" })
hl.animation({ leaf = "fadeLayersOut",       enabled = true,  speed = 3,   bezier = "expressiveEffectsDefault" })
hl.animation({ leaf = "fadePopups",          enabled = true,  speed = 4, bezier = "expressiveEffectsDefault" })
hl.animation({ leaf = "fadePopupsIn",        enabled = true,  speed = 4, bezier = "expressiveEffectsDefault" })
hl.animation({ leaf = "fadePopupsOut",       enabled = true,  speed = 2.5, bezier = "expressiveEffectsDefault" })
hl.animation({ leaf = "fadeDpms",            enabled = true,  speed = 4.5, bezier = "expressiveEffectsDefault" })

-- Border
hl.animation({ leaf = "border",              enabled = true,  speed = 3,   bezier = "standard" })
hl.animation({ leaf = "borderangle",         enabled = false })

-- Workspaces
hl.animation({ leaf = "workspaces",          enabled = true,  speed = 3.5, bezier = "fastOutSlowIn",  style = "slide" })
hl.animation({ leaf = "specialWorkspaceIn",  enabled = true,  speed = 3.5,   bezier = "emphasizedDecel", style = "slidevert" })
hl.animation({ leaf = "specialWorkspaceOut", enabled = true,  speed = 3.5,   bezier = "fastOutSlowIn",   style = "slidevert" })

-- Misc
-- zoomFactor is a continuous scale factor, so it stays on a spring curve.
hl.animation({ leaf = "zoomFactor",          enabled = true,  speed = 4.5, spring = "springOpen" })
hl.animation({ leaf = "monitorAdded",        enabled = true,  speed = 4.5, bezier = "expressiveEffectsDefault" })
