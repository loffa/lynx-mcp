# Lynx-MCP

An MCP (Model Context Protocol) server for IoT Open Lynx. It lets MCP clients search
installations, devices, and functions; read current and historical values; inspect
schedules and automations; and optionally control functions.

The server exposes MCP over Streamable HTTP at `/mcp`. It uses the Lynx HTTP APIs
for queries and MQTT for control commands.

## Requirements

- Go 1.27.2 or newer, as specified in [go.mod](go.mod).
- A Lynx API key with access to the installations you want to use.
- Network access to your Lynx API and MQTT broker.
- An MCP client that supports Streamable HTTP.

The MQTT connection is required at startup, even when control is disabled.

## Quick start

From the repository root, copy the example configuration:

```sh
cp config.example.yaml lynx-mcp.yaml
```

Edit `lynx-mcp.yaml`:

- Replace `lynx.api_key` with your API key.
- Check `lynx.api_base` and `lynx.mqtt.broker` for your Lynx environment.
- Set `lynx.mqtt.client_id` to a unique identifier for this server instance.
- For local use, set `http.listen` to `"127.0.0.1:8080"`.

Start the server:

```sh
go run . -config ./lynx-mcp.yaml
```

Alternatively, build and run a binary:

```sh
go build -o lynx-mcp .
./lynx-mcp -config ./lynx-mcp.yaml
```

The MCP endpoint is now available at `http://127.0.0.1:8080/mcp`. Control is disabled
in the example configuration. Stop the server with Ctrl+C; SIGTERM also triggers
graceful shutdown.

## Configuration

[config.example.yaml](config.example.yaml) contains a complete starting configuration:

These are example values, not automatically applied defaults. The server validates
the configuration before connecting to Lynx.

| Setting | Purpose |
| --- | --- |
| `permissions.allow_control` | Allows the control tool to publish MQTT commands. Omitted or `false` disables control. |
| `http.listen` | HTTP listening address. `:8080` listens on all interfaces; `127.0.0.1:8080` limits access to the local machine. |
| `http.read_timeout` | Maximum time to read a complete request. |
| `http.read_header_timeout` | Maximum time to read request headers. |
| `http.write_timeout` | Maximum time to write a response. Must be at least `30s`, the Lynx HTTP request timeout. |
| `http.idle_timeout` | Maximum time an idle HTTP connection stays open. |
| `http.shutdown_timeout` | Time allowed for graceful HTTP shutdown. |
| `lynx.api_base` | Absolute HTTP or HTTPS base URL for the Lynx API. |
| `lynx.api_key` | API key used by the server to authenticate with Lynx. |
| `lynx.mqtt.broker` | MQTT broker URL, including scheme and port. |
| `lynx.mqtt.client_id` | Nonempty MQTT client identifier, unique to each running server instance. |

All HTTP timeout values must be positive durations, such as `5s` or `1m`.

Pass a configuration path with `-config` or its shorthand `-c`:

```sh
./lynx-mcp -c /path/to/lynx-mcp.yaml
```

Without either flag, the server searches for `lynx-mcp.yaml` in the current
directory first, then in `/etc/iot-open`. Configuration is loaded at startup; restart
the server after changing it.

## Connecting an MCP client

Add a server connection in your MCP client with these settings:

| Field | Value for local use |
| --- | --- |
| Name | `lynx-mcp` |
| Transport | Streamable HTTP |
| URL | `http://127.0.0.1:8080/mcp` |

Use the server's reachable address when connecting from another machine. The client
connects to an already running server; Lynx credentials belong in the server's YAML
configuration.

After connecting, the client can discover the tools and their input schemas. For
example, ask it to find temperature functions, retrieve their latest values, or
inspect the schedules in an installation.

## Available tools

| Tool | Purpose | Main inputs |
| --- | --- | --- |
| `iot.search_objects` | Search accessible installations, devices, and functions, or list devices and functions in one installation. | `query` or `installation_id`; optional `types`, `metadata`, `limit`, `offset`. |
| `iot.get_object` | Retrieve an object with its full installation and optional related objects. | `type`, `id`; `installation_id` required for devices and functions; optional `include_related`. |
| `iot.get_current_values` | Retrieve the latest known values for all configured topics on selected functions. | `installation_id`, nonempty `function_ids`. |
| `iot.get_history` | Retrieve historical values for selected functions or all installation topics. | `installation_id`; optional `function_ids`, `from`, `to`, `limit`, `aggregation_method`, `aggregation_interval`. |
| `iot.list_schedules` | Inspect schedule definitions and functions sharing their target topics. | `installation_id`; optional `function_ids`, `topics`, `active`, `executor`, `limit`, `offset`. |
| `iot.list_automations` | Inspect configured EdgeApp instances, saved configuration, and app details. | `installation_id`; optional `instance_id`, `app_id`, `include_config_options`, `limit`, `offset`. |
| `iot.control_function` | Submit numeric values to a function's configured writable MQTT topics. | `installation_id`, `function_id`, `writes`; requires control permission. |

### Finding and inspecting objects

Use `query: "*"` to search all accessible objects. Filter entity categories with
`types`: `installation`, `device`, or `function`. Metadata filters accept string
values and support the `*` wildcard.

When supplying `installation_id`, omit `query` or use `"*"`. This mode lists only
devices and functions in that installation. Search results distinguish the entity
category (`type`) from the DeviceX or FunctionX model type (`model_type`).

For example, these are tool names and arguments to send through an MCP client:

```json
{
  "name": "iot.search_objects",
  "arguments": {
    "installation_id": 123,
    "types": ["function"],
    "metadata": {"name": "*temperature*"},
    "limit": 20
  }
}
```

Replace example IDs with IDs from your Lynx environment. To inspect a known function:

```json
{
  "name": "iot.get_object",
  "arguments": {
    "type": "function",
    "id": 456,
    "installation_id": 123
  }
}
```

Object retrieval includes related objects by default: a function's linked device,
or a device's linked function summaries. Set `include_related: false` to skip these
lookups. Optional relationship failures appear in `warnings`.

Search, schedule, and automation lists default to `limit: 100` and `offset: 0`,
with a maximum limit of 10,000.

### Current values and history

Current values are the latest known reports. A configured topic with no reported
value has `status: null`.

History defaults to the last 24 hours and the 500 newest entries. Omit `function_ids`
or pass an empty list to retrieve all installation topics. History supports a limit
of up to 10,000 entries and RFC 3339 timestamps for `from` and `to`.

Supported aggregation methods are `count`, `first`, `last`, `avg`, `mean`, `spread`,
`max`, `min`, `sum`, `stddev`, `median`, `percentile(X)`, and `integral(X)`.
An optional `aggregation_interval` uses a positive Go duration such as `10m` or `1h`
and requires an aggregation method.

### Schedules and automations

Schedules can be filtered by functions, exact MQTT topics, active status, or executor.
Automations include saved EdgeApp configuration and, by default, the configuration
definition for the configured app version. Set `include_config_options: false` to
skip fetching that definition. Optional app enrichment failures appear in `warnings`.

These tools describe current configuration. They do not establish runtime health,
prove historical execution, or calculate schedule execution times.

## Permissions and security

The server uses one configured Lynx API key for all MCP clients. Its HTTP endpoint
does not implement client authentication or TLS. Bind to loopback for local use;
for shared access, put it behind an authenticated HTTPS reverse proxy and restrict
network access. Connected clients share the access granted by the configured key.

### Enabling control

To allow commands that can change device state, set:

```yaml
permissions:
  allow_control: true
```

The control tool remains discoverable when control is disabled, but calls are
rejected. This permission controls publishing; MQTT still connects at startup.

Before writing, inspect the function with `iot.get_object` to identify its
`topic_write` or `topic_write_*` metadata keys and its declared states and bounds.
A control request uses metadata keys rather than raw MQTT topics:

```json
{
  "name": "iot.control_function",
  "arguments": {
    "installation_id": 123,
    "function_id": 456,
    "writes": [{"topic_key": "topic_write", "value": 1}]
  }
}
```

Use a value supported by the function's metadata. The server validates the entire
batch before publishing, including numeric constraints and distinct destination
topics. It submits writes in order with MQTT QoS 1 and retain disabled, stopping on
the first failure.

Results report each write as `submitted`, `failed`, or `not_attempted`. Submission
does not confirm the resulting device state, and delivery may be uncertain after a
failure. Inspect partial results before deciding whether to retry.

## Development and troubleshooting

Run the tests and lint checks from the repository root:

```sh
go test ./...
golangci-lint run
```

Linting requires `golangci-lint` v2. If you change the Lynx client interfaces,
regenerate the test mocks with `go generate ./internal/service`.

- **Startup fails:** Check the configuration path, API key, broker URL, and network
  access. MQTT must connect before the HTTP server starts.
- **Client cannot connect:** Check the listening address, port, and `/mcp` path,
  and use Streamable HTTP in the client.
- **Control is rejected:** Check `permissions.allow_control` and the function's
  writable topic metadata and value constraints.

## License

Released under the [MIT License](LICENSE).
