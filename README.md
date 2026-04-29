# babel-fish 🐠

A multi-service MCP server for Slack and SumoLogic, enabling AI dev tools like Crush to query messages, logs, and error traces.

> *The babel fish is small, yellow, leech-like, and probably the oddest thing in the universe. It translates any spoken language to the language of the listener.* — Douglas Adams

## Services

babel-fish is modular: you configure whichever service(s) you need via CLI flags or environment variables. At least one service must be configured.

### Slack

Authenticates using browser session credentials, enabling non-admin users to read Slack messages without workspace admin approval.

### SumoLogic

Authenticates with Access ID + Access Key to query logs, search for error traces, and inspect stack dumps.

## Tools

### Slack

| Tool | Description | Slack API |
|------|-------------|----------|
| `slack_read_messages` | Read messages from a channel | `conversations.history` |
| `slack_read_thread` | Read replies in a thread | `conversations.replies` |
| `slack_list_channels` | List accessible channels | `conversations.list` |
| `slack_search_messages` | Search messages across workspace | `search.messages` |
| `slack_get_permalink` | Parse a Slack permalink URL | URL parsing |

#### Slack Tool Parameters

| Tool | Parameter | Type | Default | Description |
|------|-----------|------|---------|-------------|
| `slack_read_messages` | `channel_id` | string | — | Channel ID (e.g. `C01234567`) |
| | `limit` | int | 50 | Max messages to return |
| | `oldest` | string | — | Start timestamp (e.g. `1700000000.000000`) |
| | `latest` | string | — | End timestamp |
| `slack_read_thread` | `channel_id` | string | — | Channel ID containing the thread |
| | `thread_ts` | string | — | Parent message timestamp |
| | `limit` | int | 50 | Max replies to return |
| `slack_list_channels` | `types` | string | `public_channel,private_channel,mpim,im` | Comma-separated channel types |
| | `limit` | int | 100 | Max channels to return |
| `slack_search_messages` | `query` | string | — | Search query |
| | `count` | int | 20 | Max results to return |
| `slack_get_permalink` | `url` | string | — | Slack permalink URL |

### SumoLogic

| Tool | Description |
|------|-------------|
| `sumo_search_logs` | Search logs with a SumoLogic query and time range |
| `sumo_search_error_traces` | Search for errors, exceptions, and stack traces |

#### SumoLogic Tool Parameters

| Tool | Parameter | Type | Default | Description |
|------|-----------|------|---------|-------------|
| `sumo_search_logs` | `query` | string | — | SumoLogic query string |
| | `from` | string | — | Start time (ISO-8601, e.g. `2024-01-01T00:00:00Z`) |
| | `to` | string | — | End time (ISO-8601) |
| | `time_range` | string | — | Relative range: `15m`, `1h`, `1d`, `7d`, `1w` |
| | `time_zone` | string | — | IANA time zone (e.g. `UTC`) |
| | `limit` | int | 100 | Max messages to return (max 10,000) |
| `sumo_search_error_traces` | `service_name` | string | — | Filter by `_sourceCategory` |
| | `trace_id` | string | — | Search for a specific trace ID |
| | `from` | string | — | Start time (ISO-8601) |
| | `to` | string | — | End time (ISO-8601) |
| | `time_range` | string | — | Relative range: `15m`, `1h`, `1d`, `7d`, `1w` |
| | `limit` | int | 100 | Max messages to return (max 10,000) |

## Resources

| URI | Description |
|-----|-------------|
| `slack://channels` | List of accessible channels (JSON) |
| `slack://channel/{id}/messages` | Recent messages for a channel (JSON) |

## Setup

### 1. Build

```bash
cd babel-fish
go build -o babel-fish .
```

### 2. Configure Credentials

#### Slack Credentials

Open Slack in your browser (`https://app.slack.com`) and use DevTools to extract:

- **`xoxc-` token** — best found in DevTools **Network** tab by looking at any `slack.com/api/` request's `Authorization: Bearer xoxc-...` header.
- **`xoxd-` cookie** — from **Application → Cookies → https://app.slack.com**, look for cookie `d`.
- **(Optional) `d-s` cookie** — for SSO workspaces, also from the Cookies list.

#### SumoLogic Credentials

- **Access ID** — from SumoLogic → Administration → Security → Access Keys
- **Access Key** — generated alongside the Access ID
- **Base URL** — defaults to `https://api.sumologic.com/api`. Use your region-specific endpoint if needed (see [SumoLogic docs](https://help.sumologic.com/APIs/General-API-Information/Sumo-Logic-Endpoints-and-Firewall-Security)).

### 3. Configure Crush

Add to `~/.config/crush/crush.json`:

#### Slack only

```json
{
  "mcpServers": {
    "babel-fish": {
      "command": "/path/to/babel-fish/babel-fish",
      "args": [
        "--slack-token", "xoxc-...",
        "--slack-cookie", "xoxd-..."
      ]
    }
  }
}
```

#### SumoLogic only

```json
{
  "mcpServers": {
    "babel-fish": {
      "command": "/path/to/babel-fish/babel-fish",
      "args": [
        "--sumo-access-id", "suABCDEF123...",
        "--sumo-access-key", "XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX"
      ]
    }
  }
}
```

#### Both Slack and SumoLogic

```json
{
  "mcpServers": {
    "babel-fish": {
      "command": "/path/to/babel-fish/babel-fish",
      "args": [
        "--slack-token", "xoxc-...",
        "--slack-cookie", "xoxd-...",
        "--sumo-access-id", "suABCDEF123...",
        "--sumo-access-key", "XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX"
      ]
    }
  }
}
```

For SSO workspaces, also add `--slack-cookie-d-s` to the args.

Environment variables are accepted for backward compatibility: `SLACK_TOKEN`, `SLACK_COOKIE`, `SLACK_COOKIE_D_S`, `SUMO_ACCESS_ID`, `SUMO_ACCESS_KEY`, `SUMO_BASE_URL`.

## CLI Usage

Since babel-fish is an MCP server that communicates over stdio using the JSON-RPC-based MCP protocol, you can't interact with it directly in a terminal. Use one of these clients:

### MCP Inspector (recommended for exploration)

```bash
npx @modelcontextprotocol/inspector \
  -e SUMO_ACCESS_ID=suABCDEF123... \
  -e SUMO_ACCESS_KEY=... \
  /path/to/babel-fish
```

Opens a web UI at `http://localhost:6274` where you can list tools, browse resources, and invoke them interactively.

### mcp-cli (Python)

```bash
pip install mcp-cli
```

Create a config file `mcp_config.json`:

```json
{
  "mcpServers": {
    "babel-fish": {
      "command": "/path/to/babel-fish",
      "args": [
        "--sumo-access-id", "suABCDEF123...",
        "--sumo-access-key", "..."
      ]
    }
  }
}
```

```bash
mcp-cli --config mcp_config.json
```

### Raw JSON-RPC (zero dependencies)

Pipe JSON-RPC requests to babel-fish's stdin. Each request is one line:

```bash
printf '%s\n' \
  '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"cli","version":"1.0"}}}' \
  '{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}' \
  '{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"sumo_search_logs","arguments":{"query":"error","time_range":"1h","limit":10}}}' \
  | ./babel-fish --sumo-access-id suABCDEF123... --sumo-access-key ...
```

## Security Notes

- **Slack credentials are your full user session.** Anyone with them can act as you on Slack.
- **SumoLogic credentials grant full API access.** Treat them like passwords.
- Session cookies expire. If authentication fails, re-extract credentials.
- Never commit credentials to version control or share them.
- babel-fish never logs your credentials.

## License

MIT
