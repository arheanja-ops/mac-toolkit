# Mac Toolkit MCP — Client Setup (Claude, VS Code, Cursor, and more)

`mac-toolkit` ships a native MCP server over stdio. Any MCP-compatible client can
launch it with the command `mac-toolkit mcp`. This guide covers the most common
third-party clients.

For Kiro-specific setup, see [MCP_SETUP.md](MCP_SETUP.md).

---

## Prerequisite: install the binary

Install with `go install github.com/arheanja-ops/mac-toolkit@latest` (or `@vX.Y.Z`).
To get the binary with real commit and date, download the release with `gh release download
vX.Y.Z -p '*darwin_arm64.tar.gz' -p checksums.txt` and verify it with `shasum -a 256 -c`
(full steps: [README](../README.md#instalación)). Alternatively, from a clone, run `make install`.

Put `mac-toolkit` in `$(go env GOPATH)/bin`. Make sure that directory is on your
`PATH` so clients can find it. Verify:

```bash
which mac-toolkit && mac-toolkit --version
```

> If a client can't resolve `mac-toolkit` from `PATH`, use the absolute path instead
> (`$(go env GOPATH)/bin/mac-toolkit`; or `./bin/toolkit` / `/usr/local/bin/toolkit` if you built with `make build` / `make install`).

The server is **local and read-safe by default**: the destructive tools
(`mac_clean_batch`, `mac_docker_cleanup`) are `dry_run=true` unless the call passes an
explicit `dry_run=false`, and even then they are **refused** unless you opt in (see
[Enabling deletion](#enabling-deletion-human-opt-in)). No tokens or network bridge are needed.

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

The server exposes **12 tools** (source: `mcp.AddTool` in `cmd/mcp.go`).

Read-only (9, never delete or write): `mac_analyze`, `mac_battery`, `mac_system`,
`mac_processes`, `mac_network`, `mac_status`, `mac_clean_preview`,
`mac_docker_compact`, `mac_doctor`.

Destructive (2, `dry_run=true` by default): `mac_clean_batch`, `mac_docker_cleanup`.
A real deletion (`dry_run=false`) needs the server env `MAC_TOOLKIT_MCP_ALLOW_DELETE=1`
**and** `confirm_plan` equal to the `plan_digest` of a reviewed preview.

Writes a file, deletes nothing (1, no `dry_run` parameter): `mac_docker_backup`
(dumps a running container's database to `~/docker-backups/`).

Tip: auto-approve only the 9 read-only tools in your client and keep manual approval
for the other three.

### Enabling deletion (human opt-in)

Destructive calls with `dry_run=false` are rejected by default. To allow them, add the
variable to the server's `env` in your client config (only you can; the agent cannot):

```json
{
  "mcpServers": {
    "mac-toolkit": {
      "command": "mac-toolkit",
      "args": ["mcp"],
      "env": { "MAC_TOOLKIT_MCP_ALLOW_DELETE": "1" }
    }
  }
}
```

Flow: preview (`dry_run=true`) returns `plan_digest` → you review the plan →
`dry_run=false` with `confirm_plan=<plan_digest>`. If the state changed since the
preview, the digest no longer matches and nothing is deleted. Every real run writes an
audit log (`approval_mode: "mcp"`). Without the opt-in, use the terminal:
`mac-toolkit docker cleanup --keep X --execute` / `mac-toolkit clean --execute`.

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
