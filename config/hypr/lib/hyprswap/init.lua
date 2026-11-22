-- ---------------------------------------------------------------------------
-- Hyprswap
-- Swaps workspace ranges between two monitors while preserving assignments
-- ---------------------------------------------------------------------------

local hs = require("lib.hyprsplit")

---@class Hyprswap
local hyprswap = {
    dsp = {},
}

---@param formatString string
local function log(formatString, ...)
    print(string.format("[hyprswap] " .. formatString, ...))
end

---@param formatString string
local function notify_error(formatString, ...)
    local err = string.format("[hyprswap] error: " .. formatString, ...)
    log(err)
    hl.notification.create({
        text     = err,
        duration = "10000",
        icon     = 3,
        color    = "rgb(ff0000)",
    })
end

---Reads the complete set of window moves needed for the swap without writing
---anything. Separating read from write prevents live-reference issues.
---@param range1 Hyprsplit.MonitorRange
---@param range2 Hyprsplit.MonitorRange
---@param num_ws integer
---@return { windows: HL.Window[], master: HL.Window|nil, target_id: integer }[] plans
---@return integer window_count
local function build_move_plan(range1, range2, num_ws)
    local plans        = {}
    local window_count = 0

    ---@param ws HL.Workspace|nil
    ---@param target_id integer
    local function plan_workspace(ws, target_id)
        if not ws then
            return
        end
        local windows = hs.windows_for_move(ws)
        window_count = window_count + #windows
        table.insert(plans, { windows = windows, master = hs.get_master(ws), target_id = target_id })
    end

    for i = 0, num_ws - 1 do
        local ws1_id = range1.min + i
        local ws2_id = range2.min + i
        plan_workspace(hl.get_workspace(ws1_id), ws2_id)
        plan_workspace(hl.get_workspace(ws2_id), ws1_id)
    end

    return plans, window_count
end

---@param monitor1 HL.Monitor
---@param monitor2 HL.Monitor
local function swap_decks(monitor1, monitor2)
    local range1 = hs.MonitorRange:new(monitor1)
    local range2 = hs.MonitorRange:new(monitor2)
    local num_ws = hs.get_config("num_workspaces")

    -- Emptied workspaces are destroyed; pinned ids are recreated on their monitor.
    hs.pin_workspaces(monitor1)
    hs.pin_workspaces(monitor2)

    local plans, window_count = build_move_plan(range1, range2, num_ws)

    log("swap: %s <-> %s (%d windows)", monitor1.name, monitor2.name, window_count)

    -- Promoting focuses windows on hidden workspaces; the caller restores the visible workspaces.
    hs.move_and_promote(plans)
end

---@param args table  { monitor1: HL.MonitorSelector, monitor2: HL.MonitorSelector }
---@return function
function hyprswap.dsp.swap(args)
    if args == nil or args.monitor1 == nil or args.monitor2 == nil then
        notify_error("dsp.swap: monitor1 and monitor2 are required")
        return function() end
    end

    return function()
        local monitor1, monitor2 = hs.resolve_swap_monitors(args.monitor1, args.monitor2)
        if not monitor1 or not monitor2 then
            return
        end

        -- The focused monitor reveals the slot the other monitor had visible (same
        -- slot offset, own range base); the other monitor keeps its visible workspace.
        local reveal_ids = nil
        local orig_focused = hl.get_active_monitor()
        local other_monitor = orig_focused and (
            (orig_focused.id == monitor1.id and monitor2) or
            (orig_focused.id == monitor2.id and monitor1)
        ) or nil
        if other_monitor then
            local range_orig  = hs.MonitorRange:new(orig_focused)
            local range_other = hs.MonitorRange:new(other_monitor)
            reveal_ids = {
                [orig_focused.id] = range_orig.min + (other_monitor.active_workspace.id - range_other.min),
            }
        end

        hs.with_workspace_transition({ monitor1, monitor2 }, function()
            swap_decks(monitor1, monitor2)
        end, reveal_ids)
    end
end

return hyprswap
