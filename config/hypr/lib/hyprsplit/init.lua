-- ---------------------------------------------------------------------------
-- Hyprsplit
-- Assigns per-monitor workspace ranges so each monitor gets its own set
-- ---------------------------------------------------------------------------

---@class Hyprsplit
---@field protected _config Hyprsplit.Config
---@field protected monitor_priority_list string[]
local hyprsplit = {
    ---@class Hyprsplit.Config
    ---@field reveal_animation? { speed: number, bezier: string } fade used while revealing swapped workspaces; needs restore_animations
    ---@field restore_animations? fun() re-applies the user's animations once the reveal fade is done
    ---@field num_workspaces? integer
    ---@field force_monitor_priority? boolean
    _config = {
        num_workspaces = 10,
        force_monitor_priority = false,
    },
    monitor_priority_list = {},
    dsp = {
        window = {},
        workspace = {},
    },
}

---@param formatString string
local function log(formatString, ...)
    print(string.format("[hyprsplit] " .. formatString, ...))
end

---@param formatString string
local function notify_error(formatString, ...)
    local error = string.format("[hyprsplit] error: " .. formatString, ...)
    log(error)
    hl.notification.create({
        text = error,
        duration = "10000",
        icon = 3,
        color = "rgb(ff0000)",
    })
end

---@class Hyprsplit.MonitorRange
---@field base integer
---@field min integer
---@field max integer
local MonitorRange = {}
---@param monitor HL.Monitor
---@return Hyprsplit.MonitorRange
function MonitorRange:new(monitor)
    local base = -1
    if #hyprsplit.monitor_priority_list == 0 and not hyprsplit._config.force_monitor_priority then
        base = monitor.id
    else
        for i, monitor_selector in ipairs(hyprsplit.monitor_priority_list) do
            if monitor.name == monitor_selector or monitor.description == monitor_selector then
                base = i - 1
            end
        end
        if base == -1 then
            local unmappedMonitors = {}
            for _, m in ipairs(hl.get_monitors()) do
                if m ~= nil and not m.is_mirror and m.id ~= -1 then
                    local mapped = false
                    for _, monitor_selector in ipairs(hyprsplit.monitor_priority_list) do
                        if
                            monitor.name == monitor_selector
                            or monitor.description == monitor_selector
                        then
                            mapped = true
                            break
                        end
                    end
                    if not mapped then
                        table.insert(unmappedMonitors, m)
                    end
                end
            end

            table.sort(unmappedMonitors, function(a, b)
                return a.name < b.name
            end)

            for i, m in ipairs(unmappedMonitors) do
                if monitor == m then
                    base = i - 1 + #hyprsplit.monitor_priority_list
                    log("auto assigning base %d to monitor %s", base, monitor.name)
                end
            end
        end
    end
    local min = (base * hyprsplit._config.num_workspaces) + 1
    local max = (base + 1) * hyprsplit._config.num_workspaces

    return setmetatable({ base = base, min = min, max = max }, {
        __index = self,
    })
end
---@param num integer
---@return boolean
function MonitorRange:contains(num)
    if type(num) ~= "number" then
        return false
    end
    return num >= self.min and num <= self.max
end
hyprsplit.MonitorRange = MonitorRange

---@param workspace_str string
---@return string
function hyprsplit.get_workspace_string(workspace_str)
    local ws_id = 1
    local num_workspaces = hyprsplit._config.num_workspaces
    local active_monitor = hl.get_active_monitor()

    if active_monitor == nil or active_monitor.active_workspace == nil then
        log("missing monitor/active_active workspace in get_workspace?")
        return "0"
    end
    local range = MonitorRange:new(active_monitor)

    local workspace_int = math.tointeger(workspace_str)
    if workspace_str:sub(1, 1) == "+" or workspace_str:sub(1, 1) == "-" then
        if workspace_int == nil then
            return tostring(active_monitor.active_workspace.id)
        end

        local local_current = active_monitor.active_workspace.id - range.min + 1
        ws_id = local_current + workspace_int

        ws_id = math.max(ws_id, 1)
        ws_id = math.min(ws_id, num_workspaces)
    elseif workspace_int ~= nil then
        ws_id = math.max(workspace_int, 1)
    elseif
        workspace_str:sub(1, 1) == "r"
        and (workspace_str:sub(2, 2) == "-" or workspace_str:sub(2, 2) == "+")
    then
        local plusminus_num = math.tointeger(workspace_str:sub(2))
        if plusminus_num == nil then
            return tostring(active_monitor.active_workspace.id)
        end

        ws_id = active_monitor.active_workspace.id + plusminus_num

        if ws_id <= 0 then
            ws_id = ((((ws_id - 1) % num_workspaces) + num_workspaces) % num_workspaces) + 1
        end
    elseif
        workspace_str:sub(1, 1) == "e"
        and (workspace_str:sub(2, 2) == "-" or workspace_str:sub(2, 2) == "+")
    then
        local plusminus_num = math.tointeger(workspace_str:sub(2))
        if plusminus_num == nil then
            return tostring(active_monitor.active_workspace.id)
        end

        local valid_workspaces = {}
        for _, ws in ipairs(hl.get_workspaces()) do
            if range:contains(ws.id) then
                table.insert(valid_workspaces, ws)
            end
        end

        local active_ws_index = -1
        for index, ws in ipairs(valid_workspaces) do
            if ws.active then
                active_ws_index = index
            end
        end
        if active_ws_index == -1 then
            return tostring(active_monitor.active_workspace.id)
        end

        local result_index = active_ws_index + plusminus_num
        if result_index < 1 then
            result_index = 1
        elseif result_index > #valid_workspaces then
            result_index = #valid_workspaces
        end

        return tostring(valid_workspaces[result_index].id)
    elseif workspace_str == "empty" then
        for i = range.min, range.max do
            local ws = hl.get_workspace(i)
            if ws == nil or ws.windows == 0 then
                return tostring(i)
            end
        end

        log("no empty workspace on monitor")
        return tostring(active_monitor.active_workspace.id)
    else
        return workspace_str
    end

    if ws_id > num_workspaces then
        ws_id = ((ws_id - 1) % num_workspaces) + 1
    end

    return tostring(range.min + ws_id - 1)
end

---@param args table args.workspace is required, it will be converted to string using tostring()
---@return function
function hyprsplit.dsp.focus(args)
    if args.workspace == nil then
        notify_error("dsp.focus: args.workspace is nil")
        return function() end
    end
    local workspace_arg = tostring(args.workspace)
    if workspace_arg then
        return function()
            local ws_string = hyprsplit.get_workspace_string(workspace_arg)
            local ws = hl.get_workspace(ws_string)
            -- A workspace in this monitor's range that lives on another monitor means assignments drifted; resync before focusing.
            if ws then
                local active_monitor = hl.get_active_monitor()
                if active_monitor then
                    local range = MonitorRange:new(active_monitor)
                    if ws.monitor.id ~= active_monitor.id and range:contains(ws.id) then
                        log("workspace exists but is on the wrong monitor")
                        hyprsplit.ensure_good_workspaces()
                    end
                end
            end

            hl.dispatch(hl.dsp.focus({
                workspace = ws_string,
                on_current_monitor = true,
            }))
        end
    else
        notify_error("dsp.focus: failed to convert args.workspace to string")
        return function() end
    end
end

---@param args table args.workspace is required, it will be converted to string using tostring()
---@return function
function hyprsplit.dsp.window.move(args)
    if args.workspace == nil then
        notify_error("dsp.window.move: args.workspace is nil")
        return function() end
    end
    local workspace = tostring(args.workspace)
    if workspace then
        return function()
            args.workspace = hyprsplit.get_workspace_string(workspace)
            hl.dispatch(hl.dsp.window.move(args))
        end
    else
        notify_error("dsp.window.move: failed to convert args.workspace to string")
        return function() end
    end
end

---@param window HL.Window
---@return boolean
local function is_master(window)
    return window.layout ~= nil and window.layout.is_master == true
end

-- Mapped windows ordered for moving: tiled stack top to bottom (assumes master orientation left),
-- then the master, then floating. Empirically the only order that keeps the stack intact;
-- moves lose the master itself, so promote_master restores it.
---@param workspace HL.Workspace
---@return HL.Window[]
function hyprsplit.windows_for_move(workspace)
    local sorted = {}
    for _, window in ipairs(hl.get_workspace_windows(workspace)) do
        if window.mapped then
            table.insert(sorted, window)
        end
    end
    table.sort(sorted, function(a, b)
        if a.floating ~= b.floating then
            return not a.floating
        end
        if is_master(a) ~= is_master(b) then
            return is_master(b)
        end
        if a.at.x ~= b.at.x then
            return a.at.x < b.at.x
        end
        return a.at.y < b.at.y
    end)
    return sorted
end

---@param workspace HL.Workspace
---@return HL.Window|nil
function hyprsplit.get_master(workspace)
    for _, window in ipairs(hl.get_workspace_windows(workspace)) do
        if window.mapped and not window.floating and is_master(window) then
            return window
        end
    end
    return nil
end

-- Focuses the window, so callers must restore focus/visible workspaces afterwards.
---@param window HL.Window
function hyprsplit.promote_master(window)
    if not window.mapped or window.floating or window.fullscreen ~= 0 or is_master(window) then
        return
    end
    hl.dispatch(hl.dsp.focus({ window = window }))
    hl.dispatch(hl.dsp.layout("swapwithmaster ignoremaster"))
end

-- Nearest empty workspace above the visible one (else the highest empty below), so the
-- reveal slides the same way every time.
---@param monitor HL.Monitor
---@param visible_id integer
---@param excluded_id integer|nil
---@return integer|nil
local function find_transition_workspace(monitor, visible_id, excluded_id)
    local range = MonitorRange:new(monitor)
    local below = nil
    for id = range.min, range.max do
        local ws = hl.get_workspace(id)
        if id ~= visible_id and id ~= excluded_id and (ws == nil or ws.windows == 0) then
            if id > visible_id then
                return id
            end
            below = id
        end
    end
    return below
end

-- Hyprland animation speed unit is 100ms; the margin covers frame and timer jitter.
local ANIMATION_SPEED_UNIT_MS = 100
local REVEAL_RESTORE_MARGIN_MS = 400
local animation_restore_timer = nil

-- Swaps the workspace slide for the configured reveal fade. hl.get_config has no getter
-- for animation leaves, so the original is restored through the restore_animations
-- callback, delayed so the in-flight fade keeps its style.
local function apply_reveal_animation()
    local reveal = hyprsplit._config.reveal_animation
    local restore = hyprsplit._config.restore_animations
    if not (reveal and restore) then
        return
    end
    if animation_restore_timer then
        animation_restore_timer:set_enabled(false)
    end
    hl.animation({ leaf = "workspaces", enabled = true, speed = reveal.speed, bezier = reveal.bezier, style = "fade" })
    animation_restore_timer = hl.timer(function()
        local ok, err = pcall(restore)
        if not ok then
            notify_error("restore_animations: %s", tostring(err))
        end
    end, {
        timeout = reveal.speed * ANIMATION_SPEED_UNIT_MS + REVEAL_RESTORE_MARGIN_MS,
        type = "oneshot",
    })
end

---@param label string
---@param fn function
local function attempt(label, fn)
    local ok, err = pcall(fn)
    if not ok then
        notify_error("with_workspace_transition %s: %s", label, tostring(err))
    end
end

-- Runs fn with animations off while each monitor is parked on an empty workspace (if it
-- has one), then refocuses each monitor's reveal workspace with a fade so the swap reads
-- as a switch. Focus, cursor warping and animations are restored even if fn throws.
---@param monitors HL.Monitor[]
---@param fn function
---@param reveal_ids table<integer, integer>|nil monitor.id -> workspace to reveal; defaults to the originally visible one
---@param excluded_id integer|nil workspace never used as the parking workspace
function hyprsplit.with_workspace_transition(monitors, fn, reveal_ids, excluded_id)
    local cursor_no_warps_orig = hl.get_config("cursor.no_warps")
    local animations_enabled_orig = hl.get_config("animations.enabled")
    local orig_focused_mon = hl.get_active_monitor()

    local visible_ids = {}
    for _, monitor in ipairs(monitors) do
        visible_ids[monitor.id] = monitor.active_workspace.id
    end

    attempt("run", function()
        hl.config({ cursor = { no_warps = true }, animations = { enabled = false } })
        for _, monitor in ipairs(monitors) do
            local temp_id = find_transition_workspace(monitor, visible_ids[monitor.id], excluded_id)
            if temp_id then
                hl.dispatch(hl.dsp.focus({ monitor = monitor }))
                hl.dispatch(hl.dsp.focus({ workspace = temp_id, on_current_monitor = true }))
            end
        end
        fn()
    end)

    attempt("animations", function()
        hl.config({ animations = { enabled = animations_enabled_orig } })
        if animations_enabled_orig then
            apply_reveal_animation()
        end
    end)
    attempt("reveal", function()
        for _, monitor in ipairs(monitors) do
            hl.dispatch(hl.dsp.focus({ monitor = monitor }))
            hl.dispatch(hl.dsp.focus({
                workspace = (reveal_ids and reveal_ids[monitor.id]) or visible_ids[monitor.id],
                on_current_monitor = true,
            }))
        end
        if orig_focused_mon then
            hl.dispatch(hl.dsp.focus({ monitor = orig_focused_mon }))
        end
    end)
    attempt("cursor", function()
        hl.config({ cursor = { no_warps = cursor_no_warps_orig } })
    end)
end

-- Pins the monitor's workspace ids to it so workspaces recreated after being emptied land
-- on the owning monitor.
---@param monitor HL.Monitor
function hyprsplit.pin_workspaces(monitor)
    local range = MonitorRange:new(monitor)
    if range.base < 0 then
        return
    end
    for id = range.min, range.max do
        hl.workspace_rule({
            workspace = tostring(id),
            monitor = monitor.name,
        })
    end
end

---Moves every plan's windows to its target workspace, then restores each plan's master.
---@param plans { windows: HL.Window[], master: HL.Window|nil, target_id: integer }[]
function hyprsplit.move_and_promote(plans)
    for _, plan in ipairs(plans) do
        for _, window in ipairs(plan.windows) do
            hl.dispatch(hl.dsp.window.move({
                workspace = plan.target_id,
                window = window,
                follow = false,
            }))
        end
    end
    for _, plan in ipairs(plans) do
        if plan.master then
            hyprsplit.promote_master(plan.master)
        end
    end
end

---@param selector1 HL.MonitorSelector
---@param selector2 HL.MonitorSelector
---@return HL.Monitor|nil
---@return HL.Monitor|nil
function hyprsplit.resolve_swap_monitors(selector1, selector2)
    local monitor1 = hl.get_monitor(selector1)
    local monitor2 = hl.get_monitor(selector2)
    if
        not monitor1 or monitor1.is_mirror or monitor1.id == -1
        or not monitor2 or monitor2.is_mirror or monitor2.id == -1
    then
        notify_error("could not resolve swap monitors")
        return nil, nil
    end
    if monitor1.id == monitor2.id then
        log("swap monitors are the same, skipping")
        return nil, nil
    end
    return monitor1, monitor2
end

---@param args table args.monitor1 and args.monitor2 are required as strings
---@return function
function hyprsplit.dsp.workspace.swap_monitors(args)
    if type(args.monitor1) ~= "string" or type(args.monitor2) ~= "string" then
        return function()
            notify_error("swap_monitors: monitor1 and monitor2 of type string are both required")
        end
    end
    return function()
        local monitor1, monitor2 = hyprsplit.resolve_swap_monitors(args.monitor1, args.monitor2)
        if not monitor1 or not monitor2 then
            return
        end

        -- Emptied hidden workspaces are destroyed (stale objects), so move by id and
        -- pin the ids for when the move recreates them.
        hyprsplit.pin_workspaces(monitor1)
        hyprsplit.pin_workspaces(monitor2)

        local plans = {}
        for _, pair in ipairs({ { monitor1, monitor2 }, { monitor2, monitor1 } }) do
            local source, destination = pair[1], pair[2]
            table.insert(plans, {
                windows = hyprsplit.windows_for_move(source.active_workspace),
                master = hyprsplit.get_master(source.active_workspace),
                target_id = destination.active_workspace.id,
            })
        end

        hyprsplit.with_workspace_transition({ monitor1, monitor2 }, function()
            hyprsplit.move_and_promote(plans)
        end)
    end
end

---@param args table args.workspace is required
---@return function
function hyprsplit.dsp.workspace.swap_with(args)
    if args.workspace == nil then
        notify_error("dsp.workspace.swap_with: args.workspace is nil")
        return function() end
    end
    return function()
        local monitor = hl.get_active_monitor()
        local target_id = tonumber(hyprsplit.get_workspace_string(tostring(args.workspace)))
        if not monitor or not monitor.active_workspace or math.type(target_id) ~= "integer" then
            notify_error("swap_with: could not resolve monitor or workspace")
            return
        end
        local visible = monitor.active_workspace
        if target_id == visible.id then
            return
        end

        hyprsplit.pin_workspaces(monitor)

        -- The target may not exist yet: nil yields an empty plan.
        local target = hl.get_workspace(target_id)
        local plans = {
            {
                windows = hyprsplit.windows_for_move(visible),
                master = hyprsplit.get_master(visible),
                target_id = target_id,
            },
            {
                windows = target and hyprsplit.windows_for_move(target) or {},
                master = target and hyprsplit.get_master(target) or nil,
                target_id = visible.id,
            },
        }

        -- Never park on the target: an empty target used as parking stays alive and breaks the empty-N swap.
        hyprsplit.with_workspace_transition({ monitor }, function()
            hyprsplit.move_and_promote(plans)
        end, nil, target_id)
    end
end

function hyprsplit.dsp.grab_rogue_windows()
    return function()
        hyprsplit.ensure_good_workspaces()
        local active_workspace = hl.get_active_workspace()
        if active_workspace == nil then
            return
        end

        for _, window in ipairs(hl.get_windows()) do
            if window.mapped and not window.workspace.special then
                local in_good_workspace = false

                for _, monitor in ipairs(hl.get_monitors()) do
                    local range = MonitorRange:new(monitor)

                    if range:contains(window.workspace.id) then
                        in_good_workspace = true
                    end
                end

                if not in_good_workspace then
                    log(
                        'moving window "%s" to active workspace %d',
                        window.title,
                        active_workspace.id
                    )
                    hl.dispatch(hl.dsp.window.move({
                        workspace = active_workspace,
                        window = window,
                        follow = false,
                    }))
                end
            end
        end
    end
end

function hyprsplit.ensure_good_workspaces()
    local cursor_no_warps_orig = hl.get_config("cursor.no_warps")
    hl.config({ cursor = { no_warps = true } })

    -- Active workspaces drift during the moves below (root cause not found); snapshot them first and refocus explicitly afterwards.
    local orig_focused_mon = hl.get_active_monitor()
    local monitors_active_workspaces = {}

    local monitors = hl.get_monitors()
    for _, m in ipairs(monitors) do
        if m ~= nil and not m.is_mirror and m.id ~= -1 then
            local range = MonitorRange:new(m)
            monitors_active_workspaces[m.name] = m.active_workspace.id

            if not range:contains(m.active_workspace.id) then
                log(
                    "%s base %d active workspace %d out of bounds, changing workspace to %d",
                    m.name,
                    range.base,
                    m.active_workspace.id,
                    range.min
                )
                monitors_active_workspaces[m.name] = range.min
                hl.dispatch(hl.dsp.focus({ workspace = range.min, on_current_monitor = true }))
            end
        end
    end

    for _, m in ipairs(monitors) do
        if m ~= nil and not m.is_mirror and m.id ~= -1 then
            local range = MonitorRange:new(m)

            for _, ws in ipairs(hl.get_workspaces()) do
                if ws.monitor and ws.monitor.id ~= m.id and range:contains(ws.id) then
                    log(
                        "workspace %d on monitor %d move to %s %d",
                        ws.id,
                        ws.monitor.id,
                        m.name,
                        range.base
                    )
                    hl.dispatch(hl.dsp.workspace.move({ workspace = ws, monitor = m }))
                    hl.dispatch(hl.dsp.focus({ monitor = m }))
                    hl.dispatch(hl.dsp.focus({
                        workspace = monitors_active_workspaces[m.name],
                        on_current_monitor = true,
                    }))
                end
            end

            hyprsplit.pin_workspaces(m)
        end
    end

    if orig_focused_mon then
        hl.dispatch(hl.dsp.focus({ monitor = orig_focused_mon }))
    end

    hl.config({ cursor = { no_warps = cursor_no_warps_orig } })
end

---@param k string
---@param v any
---@param valid boolean|nil
local function load_config_value(k, v, valid)
    if v ~= nil then
        if (valid == nil and type(v) == type(hyprsplit._config[k])) or valid then
            hyprsplit._config[k] = v
        else
            notify_error('config key "%s" invalid value "%s"', tostring(k), tostring(v))
        end
    end
end

---@param config Hyprsplit.Config
function hyprsplit.config(config)
    load_config_value(
        "num_workspaces",
        config.num_workspaces,
        math.type(config.num_workspaces) == "integer" and config.num_workspaces >= 1
    )
    load_config_value("force_monitor_priority", config.force_monitor_priority)

    local reveal = config.reveal_animation
    load_config_value(
        "reveal_animation",
        reveal,
        type(reveal) == "table"
            and type(reveal.speed) == "number"
            and reveal.speed > 0
            and type(reveal.bezier) == "string"
    )
    load_config_value(
        "restore_animations",
        config.restore_animations,
        type(config.restore_animations) == "function"
    )
end

---@param key string
---@return any
function hyprsplit.get_config(key)
    return hyprsplit._config[key]
end

---@param priority string[]
function hyprsplit.monitor_priority(priority)
    for _, monitor in ipairs(priority) do
        if type(monitor) == "string" then
            log("assigning monitor priority: %s -> %d", monitor, #hyprsplit.monitor_priority_list)
            table.insert(hyprsplit.monitor_priority_list, monitor)
        end
    end
end

hl.on("config.reloaded", function()
    hyprsplit.ensure_good_workspaces()
end)

hl.on("monitor.added", function(monitor)
    local cursor_no_warps_orig = hl.get_config("cursor.no_warps")
    hl.config({ cursor = { no_warps = true } })

    local orig_focused_mon = hl.get_active_monitor()
    hl.dispatch(hl.dsp.focus({ monitor = monitor }))
    local range = MonitorRange:new(monitor)
    hl.dispatch(hl.dsp.focus({ workspace = range.min, on_current_monitor = true }))
    hl.dispatch(hl.dsp.focus({ monitor = orig_focused_mon }))

    hl.config({ cursor = { no_warps = cursor_no_warps_orig } })

    hyprsplit.ensure_good_workspaces()
end)

return hyprsplit
