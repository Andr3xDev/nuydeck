local palette = require("theme.palette")

local color = palette({
    bg = "#F3F4F4",
    bg_elevated = "#FAFAFA",
    border_muted = "#E6E7E7",
    border = "#DADCDC",
    border_strong = "#B9BCBC",
    text_muted = "#5E6D67",
    text_secondary = "#4B5A54",
    text_primary = "#16211E",

    turquoise = "#0B726D",
    turquoise_dark = "#DFF3F1",
    aquamarine = "#0E7A4C",
    aquamarine_dark = "#DFF3E7",
    blue = "#146496",
    blue_dark = "#DFEBF7",

    red = "#C23B46",
    red_dark = "#FBE4E5",
    green = "#1E8A3B",
    green_dark = "#E1F5E5",
    orange = "#B85F1D",
    orange_dark = "#FBEADC",

    yellow = "#8A7C12",
    yellow_dark = "#F5F0D6",
    purple = "#6E4FA0",
    purple_dark = "#EDE5F7",
    pink = "#B23E76",
    pink_dark = "#F7E3ED",
})

color.border_active = color.border
color.border_inactive = color.border_muted
color.wallpaper = "~/.config/wallpapers/vogabond.png"
color.lock_background_brightness = 0.9

return color
