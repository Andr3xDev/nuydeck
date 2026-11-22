----------------------------------------------------------------------------------
--
--            ,---.
--           /  O  \  ,---.  ,---.  ,---.
--          |  .-.  || .-. || .-. |(  .-'
--          |  | |  || '-' '| '-' '.-'  `)
--          `--' `--'|  |-' |  |-' `----'
--                   `--'   `--'
--             _______
--　　　　　 /  ＞　　フ
--　　　　　| 　_　 _ l      RESUME: Here you select the default apps and which
--　 　　　／` ミ＿xノ               ones are started at autostart.
--　　 　 /　　　 　 |
--　　　 /　 ヽ　　 ﾉ
--　 　 │　　|　|　|         AUTHOR:  Andr3xDev
--　／￣|　　 |　|　|        URL: https://github.com/Andr3xDev/nuydeck
--　| (￣ヽ＿_ヽ_)__)
--　＼二つ
-----------------------------------------------------------------------------------


---------------------------------------------------------------------------------
--- Autostart
---------------------------------------------------------------------------------
hl.on("hyprland.start", function()
	----- apps -----
	hl.exec_cmd("awww-daemon & dunst & hypridle")

	----- shell -----
	hl.exec_cmd("quickshell --config lucyna &")

	----- services -----
	hl.exec_cmd("/usr/lib/polkit-gnome/polkit-gnome-authentication-agent-1 &")
	hl.exec_cmd("wl-paste --type text --watch cliphist store &")
	hl.exec_cmd("wl-paste --type image --watch cliphist store &")

	----- ui -----
	hl.exec_cmd("hyprctl setcursor phinger-cursors-light 25 &")
end)


---------------------------------------------------------------------------------
--- Exported commands — consumed by keyblinds and other modules
---------------------------------------------------------------------------------
return {
	-- programs
	terminal       = "ghostty -e zsh --no-rcs -c 'kotofetch; exec zsh'",
	fileManager    = "ghostty --class=yazi -e yazi",
	top            = "ghostty --class=btop -e btop",
	bluethoot      = "ghostty --class=bluetui -e bluetui",
	network        = "ghostty --class=nmtui -e nmtui",
	fetch          = "ghostty --class=fastfetch -e zsh -c 'fastfetch; read '''",

	-- panels
    screenshot_panel   = "quickshell --config lucyna ipc call panel toggle screenshot",
    cursor_tools_panel = "quickshell --config lucyna ipc call panel toggle cursor-tools",

    -- launchers
	powermenu      = "quickshell ipc --config lucyna call togglePower handle",

	-- shell basics
	menu           = "quickshell --config lucyna ipc call panel toggle launcher",
	toggle_bar     = "quickshell --config lucyna ipc call panel toggle bar-$(hyprctl monitors -j | jq -r '.[] | select(.focused) | .name')",
	test           = "qs -c lucyna ipc call panel toggle testPanel",
	clipboard      = "quickshell ipc --config lucyna call toggleClip handle",
	calendar       = "quickshell --config lucyna ipc call panel toggle calendar",

    -- general
	reload         = "killall quickshell && quickshell --config lucyna && hyprctl reload",
}
