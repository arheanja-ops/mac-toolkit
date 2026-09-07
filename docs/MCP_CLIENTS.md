# Mac Toolkit MCP — Client Setup (Claude, VS Code, Cursor, and more)

`mac-toolkit` ships a native MCP server over stdio. Any MCP-compatible client can
launch it with the command `mac-toolkit mcp`. This guide covers the most common
third-party clients.

For Kiro-specific setup, see [MCP_SETUP.md](MCP_SETUP.md).

---

## Prerequisite: install the binary

```bash
go install github.com/arheanja-ops/mac-toolkit@latest
```

This puts `mac-toolkit` in `$(go env GOPATH)/bin`. Make sure that directory is on your
`PATH` so clients can find it. Verify:

```bash
which mac-toolkit && mac-toolkit --version
```

> If a client can't resolve `mac-toolkit` from `PATH`, use the absolute path instead
> (`$(go env GOPATH)/bin/mac-toolkit`, `/usr/local/bin/toolkit`, or `./bin/toolkit`).

The server is **local and read-safe by default**: destructive tools require an
explicit `dry_run=false`. No tokens or network bridge are needed.

---

## Claude Desktop

Edit the config file (create it if it doesn't exist):

- macOS: `~/Library/Application Support/Claude/claude_desktop_config.json`

```json
{
  "mcpServers": {
    "mac-toolkit": {
      "command": "mac-toolkit",
      "args": ["mcp"]
    }
  }
}
```

Restart Claude Desktop. The `mac-toolkit` tools appear under the tools (🔨) menu.

---

## Claude Code (CLI)

One command registers the server:

```bash
claude mcp add mac-toolkit -- mac-toolkit mcp
```

Check it:

```bash
claude mcp list
```

Scope flags: add `--scope user` for all projects, or `--scope project` to share
via a checked-in `.mcp.json`. Default scope is local to the current project.

---

## VS Code (GitHub Copilot / Agent mode)

> VS Code uses the key **`servers`**, not `mcpServers`.

Per-workspace — create `.vscode/mcp.json`:

```json
{
  "servers": {
    "mac-toolkit": {
      "type": "stdio",
      "command": "mac-toolkit",
      "args": ["mcp"]
    }
  }
}
```

Or globally: Command Palette (⌘⇧P) → **MCP: Add Server** → *Command (stdio)* →
command `mac-toolkit`, args `mcp`. Then open Copilot Chat in **Agent** mode and the
`mac-toolkit` tools become available.

---

## Cursor

Per-project — create `.cursor/mcp.json` (or global `~/.cursor/mcp.json`):

```json
{
  "mcpServers": {
    "mac-toolkit": {
      "command": "mac-toolkit",
      "args": ["mcp"]
    }
  }
}
```

Enable it under **Cursor Settings → MCP**.

---

## Windsurf

Edit `~/.codeium/windsurf/mcp_config.json`:

```json
{
  "mcpServers": {
    "mac-toolkit": {
      "command": "mac-toolkit",
      "args": ["mcp"]
    }
  }
}
```

Then **Refresh** the MCP servers in the Cascade panel.

---

## Zed

In `settings.json` (Command Palette → *zed: open settings*):

```json
{
  "context_servers": {
    "mac-toolkit": {
      "command": {
        "path": "mac-toolkit",
        "args": ["mcp"]
      }
    }
  }
}
```

---

## Generic MCP client

Any client that speaks MCP over stdio can use:

- **command**: `mac-toolkit`
- **args**: `["mcp"]`
- **transport**: stdio
- **env**: none required

---

## Available tools

Read-only / safe (never delete): `mac_analyze`, `mac_battery`, `mac_system`,
`mac_processes`, `mac_network`, `mac_status`, `mac_clean_preview`,
`mac_docker_compact`.

Destructive (dry-run by default; require explicit `dry_run=false`):
`mac_clean_batch`, `mac_docker_cleanup`, `mac_docker_backup`.

---

## Verify the server runs

```bash
# List tools over JSON-RPC (quick smoke test)
echo '{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}' | mac-toolkit mcp
```

## Troubleshooting

- **Client can't find `mac-toolkit`** — use the absolute path in `command`, or ensure
  `$(go env GOPATH)/bin` is on your `PATH`.
- **macOS blocks the binary** — `xattr -d com.apple.quarantine "$(which mac-toolkit)"`.
- **No battery data** — desktop Macs (iMac/Mac mini/Mac Pro) have no battery, so
  `mac_battery` returns empty.
- **`mac_analyze` is slow** — the `repos` domain scans large trees; other domains
  are unaffected. Per-analyzer timeout is 180s.
