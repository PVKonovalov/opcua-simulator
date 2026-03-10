# OPC UA Simulator

`opcua-simulator` is a Go-based OPC UA client that:
- connects to an [OPC UA server](https://github.com/azure-samples/iot-edge-opc-plc),
- discovers a subtree of nodes from a configured root,
- builds simulated values for supported variable types,
- continuously writes those values back to the server.

The repository also includes a `docker-compose` setup with an `opcplc` test server.

## Features

- OPC UA secure connection support (`Basic256Sha256`, `SignAndEncrypt`)
- Endpoint discovery and connection configuration
- Recursive node tree traversal from a root node
- Variable simulation by data type:
  - `Float` -> sine-like numeric signal
  - `String` -> alternating selectable values
- Continuous write loop to push simulated points
- Optional OPC UA debug logging support in the pool

## Project Structure

- `main.go` - application entrypoint, CLI flags, discovery + simulation loop
- `pkg/opcuapool/` - OPC UA client wrapper (connect, browse, write, subscribe)
- `pkg/simulator/` - simulated point generation logic
- `docker-compose.yml` - local stack (`opcplc` + `opcua-simulator`)
- `config/` - runtime configuration/cert paths used by compose

## Requirements

- Go 1.26+ (matching project Docker build stage)
- Access to an OPC UA server endpoint
- For secure mode: valid certificate and private key files

## CLI Usage

```bash
./opcua-simulator \
  --endpoint opc.tcp://127.0.0.1:4840 \
  --node "ns=3;s=OpcPlc" \
  --cert /path/to/cert.pem \
  --key /path/to/key.pem
```

### Flags

- `--endpoint` (required): OPC UA server URL
- `--node` (optional, default `i=84`): root node to start discovery
- `--cert` (optional): TLS certificate path
- `--key` (optional): TLS private key path

## Run with Docker Compose

Create a `.env` file in project root (example):

```env
OPCUA_SERVER_PORT=4840
OPCUA_SIMULATOR_NODE=ns=3;s=OpcPlc
OPCUA_SIMULATROR_STEP_MS=1000
```

Start stack:

```bash
docker compose build
docker compose up -d
```

Stop stack:

```bash
docker compose down
```

## Simulation Flow

1. Parse CLI flags
2. Connect to OPC UA server
3. Load all child nodes under `--node`
4. Register simulation points for supported data types
5. Start async simulation generator
6. Write generated values to matching OPC UA node IDs

## Notes

- Unsupported node data types are skipped with a log message.
- Write failures are logged per node and simulation continues.
- The application handles `SIGINT` / `SIGTERM` for graceful shutdown.

## Development

Build locally:

```bash
go build -o opcua-simulator .
```

## License

MIT License. See `LICENSE`.
