----------------------------------------------------------------------------------
--
--      ,---.
--     /  O  \ ,--.--. ,--.--.  ,---.  ,--,--.,--.--.  ,--,--.,--,--,  ,---.  ,---.
--    |  .-.  ||  .-. \|  .-. \| .-. :' ,-.  ||  .--' ' ,-.  ||      \|  .--'| .-. :
--    |  | |  ||  '-' /|  '-' /\   --.\ '-'  ||  |    \ '-'  ||  ||  |'  '--.\   --.
--    `--' `--'|  |-'  |  |-'   `----' `--`--'`--'     `--`--'`--''--' `---'  `----'
--             `--'    `--'
--             _______
--　　　　　 /  ＞　　フ     
--　　　　　| 　_　 _ l      RESUME: Visual config to make       
--　 　　　／` ミ＿xノ               beautiful components
--　　 　 /　　　 　 |
--　　　 /　 ヽ　　 ﾉ
--　 　 │　　|　|　|         AUTHOR:  Andr3xDev
--　／￣|　　 |　|　|        URL: https://github.com/Andr3xDev/nuydeck
--　| (￣ヽ＿_ヽ_)__)
--　＼二つ
----------------------------------------------------------------------------------


----- Import Colors -----
local theme = require("theme")


----------------------------------------------------------------------------------
--- Gaps, borders, layouts & configs
----------------------------------------------------------------------------------
hl.config({
    general = {
        layout           = "master",
        gaps_in          = 5,
        gaps_out         = 10,
        border_size      = 1,
        allow_tearing    = true,
        resize_on_border = false,

        col = {
            active_border   = theme.border_active,
            inactive_border = theme.border_inactive,
        },
    },

    master = {
        mfact      = 0.6,
        orientation = "left",
        new_status = "slave",
    },
})


----------------------------------------------------------------------------------
--- Decoration
----------------------------------------------------------------------------------

hl.config({
    decoration = {
        rounding         = 6,
        dim_special      = 0.5,
        active_opacity   = 1,
        inactive_opacity = 0.92,

        shadow = {
            range          = 20,
            color          = theme.shadow_active,
            enabled        = true,
            render_power   = 2,
            color_inactive = theme.shadow_inactive,
        },

        blur = {
            enabled           = true,
            size              = 4,
            passes            = 2,
            popups            = true,
            special           = false,
            ignore_opacity    = false,
            new_optimizations = true,
        },
    },
})
