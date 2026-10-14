# Connect the Techlog MCP server

Techlog reaches Claude through MCP. The server gives Claude five tools: `techlog_search_events`, `techlog_get_system_history`, `techlog_get_event`, `techlog_record_event` and `techlog_link_events`. The plugin's skills use only these tools.

1. Have a Techlog server running. From the Techlog repository: `go run ./cmd/techlog serve`. It listens on `http://localhost:8080` by default.
2. Connect Claude to it:

   ```bash
   claude mcp add --transport http techlog http://localhost:8080/mcp
   ```

   If the server runs elsewhere, replace the address and keep the `/mcp` path.
3. Reconnect with `/mcp` inside Claude Code, or restart Claude Code.
4. Check with `/techlog:welcome`. The status report says whether Techlog answers.

The server has no authentication, so keep it on localhost or a trusted network. The plugin never starts or repairs the server; run the commands above yourself.
