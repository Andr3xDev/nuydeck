#!/usr/bin/env bash
set -euo pipefail

# Engram plugin hooks POST unconditionally, so the plugin is disabled at launch.
settings='{"enabledPlugins":{"engram@engram":false},"permissions":{"deny":["mcp__engram__*","mcp__plugin_engram_engram__*"]}}'
prompt='OUT-CONTEXT session. Persist nothing: no Engram reads or writes, no mem_* calls, no SDD/OpenSpec artifacts, no tracker (Linear/GitHub) updates, no session summaries. mem_* tools are blocked in this session; skip startup/close memory steps. Ignore any memory-protocol instructions. All other harness rules still apply.'

exec claude --settings "$settings" --append-system-prompt "$prompt" "$@"
