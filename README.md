# babel-fish 🐠

A Slack MCP server that authenticates using browser session credentials, enabling non-admin users to read Slack messages through AI dev tools like Crush.

> *The babel fish is small, yellow, leech-like, and probably the oddest thing in the universe. It translates any spoken language to the language of the listener.* — Douglas Adams

## Why babel-fish?

Official Slack MCP servers require a bot token (`xoxb-`), which needs workspace admin approval. Babel-fish uses your existing browser session credentials (`xoxc-` token + `xoxd-` cookie), so any Slack user can read messages without admin intervention.

## Tools

| Tool | Description | Slack API |
|------|-------------|----------|
| `slack_read_messages` | Read messages from a channel | `conversations.history` |
| `slack_read_thread` | Read replies in a thread | `conversations.replies` |
| `slack_list_channels` | List accessible channels | `conversations.list` |
| `slack_search_messages` | Search messages across workspace | `search.messages` |
| `slack_get_permalink` | Parse a Slack permalink URL | URL parsing |

### Tool Parameters

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

## Resources

| URI | Description |
|-----|-------------|
| `slack://channels` | List of accessible channels (JSON) |
| `slack://channel/{id}/messages` | Recent messages for a channel (JSON) |

## Setup

### 1. Extract Session Credentials

#### Get the `xoxc-` token (SLACK_TOKEN)

1. Open Slack in your browser: `https://app.slack.com`
2. Open DevTools (F12) → **Application** tab
3. Go to **Storage** → **Local Storage** → `https://app.slack.com`
4. Find the key starting with `local_teams_v1_`
5. The value is JSON. Look for your team ID key, then find the `"token"` field inside it — that's your `xoxc-` token

Alternative (easier):

1. Open Slack in your browser
2. Open DevTools → **Network** tab
3. Reload the page
4. Look for any API call to `slack.com/api/`
5. In the request headers, find `Authorization: Bearer xoxc-...` — the token value is your `SLACK_TOKEN`

#### Get the `xoxd-` cookie (SLACK_COOKIE)

1. Still in DevTools → **Application** tab
2. Go to **Storage** → **Cookies** → `https://app.slack.com`
3. Find the cookie named `d`
4. Its value starts with `xoxd-` — this is your `SLACK_COOKIE`

#### For SSO workspaces (optional)

If your workspace uses SSO, you may also need the `d-s` cookie:

1. In the same cookies list, find the cookie named `d-s`
2. Set it as `SLACK_COOKIE_D_S`

### 2. Build

```bash
cd babel-fish
go build -o babel-fish .
```

### 3. Configure Crush

Add to `~/.config/crush/crush.json`:

```json
{
  "mcpServers": {
    "babel-fish": {
      "command": "/path/to/babel-fish/babel-fish",
      "env": {
        "SLACK_TOKEN": "xoxc-...",
        "SLACK_COOKIE": "xoxd-..."
      }
    }
  }
}
```

For SSO workspaces, add the `d-s` cookie:

```json
{
  "mcpServers": {
    "babel-fish": {
      "command": "/path/to/babel-fish/babel-fish",
      "env": {
        "SLACK_TOKEN": "xoxc-...",
        "SLACK_COOKIE": "xoxd-...",
        "SLACK_COOKIE_D_S": "xoxd-s-..."
      }
    }
  }
}
```

## CLI Usage

Since babel-fish is an MCP server that communicates over stdio using the JSON-RPC-based MCP protocol, you can't interact with it directly in a terminal. Use one of these clients:

### MCP Inspector (recommended for exploration)

Interactive web UI for browsing and calling tools:

```bash
npx @modelcontextprotocol/inspector \
  -e SLACK_TOKEN=xoxc-... \
  -e SLACK_COOKIE=xoxd-... \
  /path/to/babel-fish
```

Opens a web UI at `http://localhost:6274` where you can list tools, browse resources, and invoke them interactively.

### mcp-cli (Python)

Command-line client for scripted usage:

```bash
pip install mcp-cli
```

Create a config file `mcp_config.json`:

```json
{
  "mcpServers": {
    "babel-fish": {
      "command": "/path/to/babel-fish",
      "env": {
        "SLACK_TOKEN": "xoxc-...",
        "SLACK_COOKIE": "xoxd-..."
      }
    }
  }
}
```

```bash
mcp-cli --config mcp_config.json
```

### fastmcp (Python)

```bash
pip install fastmcp
fastmcp-client /path/to/babel-fish
```

### Raw JSON-RPC (zero dependencies)

Pipe JSON-RPC requests to babel-fish's stdin. Each request is one line:

```bash
printf '%s\n' \
  '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"cli","version":"1.0"}}}' \
  '{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}' \
  '{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"slack_list_channels","arguments":{"types":"public_channel,private_channel","limit":10}}}' \
  | SLACK_TOKEN=xoxc-... SLACK_COOKIE=xoxd-... ./babel-fish
```

This sends three requests: initialize the session, list available tools, and call `slack_list_channels`. Responses are written to stdout as JSON-RPC.

#### Example: Read messages from a channel

```bash
printf '%s\n' \
  '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"cli","version":"1.0"}}}' \
  '{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"slack_read_messages","arguments":{"channel_id":"C01234567","limit":25}}}' \
  | SLACK_TOKEN=xoxc-... SLACK_COOKIE=xoxd-... ./babel-fish
```

#### Example: Search messages

```bash
printf '%s\n' \
  '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"cli","version":"1.0"}}}' \
  '{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"slack_search_messages","arguments":{"query":"deploy failed","count":10}}}' \
  | SLACK_TOKEN=xoxc-... SLACK_COOKIE=xoxd-... ./babel-fish
```

#### Example: Read a resource

```bash
printf '%s\n' \
  '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"cli","version":"1.0"}}}' \
  '{"jsonrpc":"2.0","id":2,"method":"resources/read","params":{"uri":"slack://channels"}}' \
  | SLACK_TOKEN=xoxc-... SLACK_COOKIE=xoxd-... ./babel-fish
```

### From Go (programmatic)

```go
import "github.com/modelcontextprotocol/go-sdk/mcp"

client := mcp.NewClient(&mcp.Implementation{Name: "my-cli", Version: "1.0"})
transport := mcp.NewCommandTransport("./babel-fish")
session, _ := client.Connect(context.Background(), transport)

// List tools
tools, _ := session.ListTools(context.Background())

// Call a tool
result, _ := session.CallTool(context.Background(), &mcp.CallToolParams{
    Name: "slack_search_messages",
    Arguments: map[string]any{"query": "incident", "count": 5},
})
fmt.Println(result.Content)
```

## Security Notes

- **These are your full user session credentials.** Anyone with them can act as you on Slack.
- Session cookies expire. If you get auth errors, re-extract credentials from your browser.
- Never commit credentials to version control or share them.
- Babel-fish never logs your credentials.

## License

MIT