---
name: welcome
description: Use when the user asks how Techlog or the techlog plugin works, how to connect or set up the Techlog MCP server, which development plugin is configured, or wants to switch it. A help desk for the plugin. Takes a free-text question, or use <plugin> to switch the development plugin, or nothing for a status report.
---

# Techlog help desk

Look at what the user typed after `/techlog:welcome` and choose one mode.

## No arguments: status

Report four things:

- **Active development plugin.** Read `.claude/techlog.json` in the project, then `~/.claude/techlog.json`, and take `devPlugin` from the first that exists; otherwise the default is `superpowers`. Say which of the three it came from. If a file exists but is malformed, say so.
- **Mapped skills.** Read `${CLAUDE_PLUGIN_ROOT}/config/mappings.yaml`. For each stage of the active plugin, say whether the mapped skill is available to you.
- **Techlog connection.** Call `techlog_search_events` with `{"limit": 1}`. If the tool is missing or the call fails, say Techlog is not connected and show the connect command from `${CLAUDE_PLUGIN_ROOT}/docs/connect-mcp.md`.
- **Skills.** `/techlog:problem`, `/techlog:proposal` and `/techlog:welcome`, one line each.

## `use <plugin>`: switch the development plugin

1. Read `${CLAUDE_PLUGIN_ROOT}/config/mappings.yaml`. If it has no top-level block named exactly like `<plugin>`, list the plugins it does have and change nothing.
2. Check that every skill the block maps is available to you. If one is not, name it, say the plugin has to be installed first, and change nothing.
3. Write `{"devPlugin": "<plugin>"}` to `.claude/techlog.json` in the project. Create `.claude/` if needed. If the file already exists, keep any other keys in it. Do not write the user-level file.
4. Say which plugin is now active.

## Anything else: a question

Answer only from the help pages in `${CLAUDE_PLUGIN_ROOT}/docs/`: `connect-mcp.md`, `skills.md`, `configuration.md` and `lifecycle.md`. Read the pages that fit the question and answer in a few lines, quoting commands exactly. If the pages do not cover it, say so and point to https://techlog.dadev.nl/docs. Do not answer from memory about how Techlog works inside.

## Rules

- Never start, stop or repair the Techlog server. Show the command and let the user run it.
- Change nothing except `.claude/techlog.json`, and only in `use <plugin>` mode.
