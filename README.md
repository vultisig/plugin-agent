# Vultisig Plugin Agent

A Go-based server system that manages TSS (Threshold Signature Scheme) operations for crypto vaults through a plugin architecture. The system enables secure multi-party computation for cryptocurrency transaction signing without exposing private keys.

## Architecture

The system uses a **worker-queue pattern** with two main components:

### Agent Server (`cmd/agent/server.go`)
- HTTP REST API and WebSocket server for vault operations
- Handles plugin policy management and keysigning requests
- Enqueues TSS tasks to Redis-backed Asynq queues
- Provides real-time event streaming via WebSockets

### Worker (`cmd/worker/main.go`)
- Background task processor consuming from Redis queues
- Executes DKLS TSS operations (keygen, keysign, reshare)
- Communicates with relay servers to coordinate TSS protocol
- Stores results in Redis/PostgreSQL

### Data Flow
```
Client → Agent Server → Redis Queue → Worker → TSS Operations
                ↓                         ↓
           PostgreSQL              Relay Server
                ↓                         ↓
           S3 Storage ←──────────── Results
```

## Core Components

### Storage Layer
- **PostgreSQL**: Persistent storage for plugin policies and system events (via pgx/v5 + sqlc)
- **Redis**: Ephemeral storage for session deduplication and task results
- **S3 (MinIO)**: Encrypted vault backups with AES encryption

### API Layer
- **Framework**: Echo v4 with WebSocket support
- **Endpoints**:
  - Plugin policy CRUD (create, update, delete)
  - Vault operations (get, exist, delete, reshare)
  - Keysigning flow (submit request, poll for result)
  - Address derivation
  - Real-time event streaming

### Policy System
- Plugin policies define transaction rules and constraints
- Policies signed by vault owners using ECDSA signatures
- Policy verification uses derived public keys from vault chain codes
- Recipe specifications define allowed transaction patterns

### Event System
- System events tracked in PostgreSQL (vault reshares, deletions, policy changes)
- WebSocket streaming for real-time notifications
- Events include public key, policy ID, event type, and JSON data

## Key Dependencies

- `github.com/vultisig/verifier` - Core TSS/DKLS implementation
- `github.com/vultisig/mobile-tss-lib` - TSS key operations
- `github.com/vultisig/recipes` - Policy recipe specifications
- `github.com/vultisig/vultisig-go` - Common utilities, relay client
- `github.com/hibiken/asynq` - Redis-backed task queue
- `github.com/labstack/echo/v4` - HTTP server framework

## Prerequisites

- Go 1.21+
- PostgreSQL 14+
- Redis 6+
- MinIO or S3-compatible storage
- (Optional) Nix with flakes enabled

## Development Setup

### Using Nix Flakes (Recommended)

The project includes a Nix flake that automatically sets up the development environment:

```bash
nix develop
```

This automatically starts:
- PostgreSQL on `$PG_PORT` (databases: `vs-plugins-server`, `vs-plugins-plugin`)
- Redis on port 6379
- MinIO on port 9000

### Manual Setup

1. Start PostgreSQL:
```bash
createdb vultisig-agent
```

2. Start Redis:
```bash
redis-server
```

3. Start MinIO:
```bash
docker-compose up -d minio
./create_buckets.sh
```

4. Run migrations:
```bash
goose -dir internal/storage/postgres/migrations/plugin postgres "postgresql://..." up
```

## Configuration

Copy and edit the example configuration:

```bash
cp agent.example.json agent.json
```

Configuration fields:

```json
{
  "vault_service": {
    "relay": {"server": "https://api.vultisig.com/router"},
    "local_party_prefix": "agent",
    "encryption_secret": "your-secret-key",
    "do_setup_msg": false
  },
  "redis": {"host": "localhost", "port": "6379"},
  "block_storage": {
    "host": "http://localhost:9000",
    "region": "us-east-1",
    "access_key": "minioadmin",
    "secret": "minioadmin",
    "bucket": "vultisig-agent"
  },
  "database": {
    "dsn": "postgres://user:pass@localhost:5432/vultisig-agent?sslmode=disable"
  },
  "plugin": {
    "plugin_id": "your-plugin-id",
    "file_path": "./recipe-specification.json"
  },
  "server": {"port": 8081},
  "verifier": {
    "url": "http://localhost:8080",
    "token": "",
    "party_prefix": "agent"
  }
}
```

Both agent and worker use the same `agent.json` configuration file.

## Building and Running

### Build

```bash
go build -o bin/agent ./cmd/agent
go build -o bin/worker ./cmd/worker
```

### Run

```bash
./bin/agent
```

In a separate terminal:

```bash
./bin/worker
```

## API Endpoints

### Vault Operations
- `GET /vault/:hex_chain_code` - Get vault by chain code
- `GET /vault/:hex_chain_code/exist` - Check vault existence
- `DELETE /vault/:hex_chain_code` - Delete vault
- `POST /vault/reshare` - Reshare vault (threshold change)

### Keysigning
- `POST /vault/sign` - Submit keysign request
- `GET /vault/sign/response/:taskId` - Poll for keysign result

### Policy Operations
- `POST /plugin/policy` - Create plugin policy
- `PUT /plugin/policy/:policyId` - Update plugin policy
- `DELETE /plugin/policy/:policyId` - Delete plugin policy
- `GET /plugin/policies/:hex_chain_code` - Get policies for vault

### Other
- `GET /propose` - Submit transaction proposal and get signature
- `POST /address/derive` - Derive address from vault
- `GET /events` - WebSocket endpoint for real-time event streaming
- `GET /health` - Health check endpoint

## Database Management

### Generate Go code from SQL queries

```bash
sqlc generate
```

### Run migrations

```bash
goose -dir internal/storage/postgres/migrations/plugin postgres "postgresql://..." up
```

### Adding new queries

1. Add SQL queries to `internal/storage/postgres/queries/*.sql`
2. Run `sqlc generate`
3. Update interface in `internal/storage/interfaces/storage.go`
4. Implement in `internal/storage/postgres/storage.go`

## Project Structure

```
.
├── cmd/
│   ├── agent/          # Agent server entrypoint
│   └── worker/         # Worker entrypoint
├── internal/
│   ├── api/            # HTTP handlers and WebSocket logic
│   ├── config/         # Configuration loading
│   ├── health/         # Health check server
│   ├── logging/        # Logging setup
│   ├── policy/         # Policy validation and recipe specs
│   ├── storage/        # Storage layer
│   │   ├── interfaces/ # Storage interfaces
│   │   └── postgres/   # PostgreSQL implementation
│   │       ├── migrations/  # SQL migrations
│   │       └── queries/     # sqlc-generated code
│   └── types/          # Common types
├── testdata/           # Test fixtures
├── agent.example.json  # Example configuration
├── docker-compose.yaml # Docker services setup
├── flake.nix          # Nix development environment
└── sqlc.yml           # sqlc configuration
```

## TSS Operations Flow

### Keysigning Example

1. Client POSTs to `/vault/sign` with `KeysignRequest` (session ID, messages to sign)
2. Agent server checks Redis for session deduplication
3. Agent verifies vault exists and is decryptable from S3
4. Task enqueued to Asynq with `tasks.TypeKeySignDKLS`
5. Worker picks up task, calls `vaultMgmService.HandleKeySignDKLS`
6. Worker communicates with relay server to coordinate TSS protocol
7. Result stored in Asynq, client polls `/vault/sign/response/:taskId`

### Propose Endpoint (Synchronous Signing)

1. Client calls `/propose?policy_id=...&network=...&tx_hex=...`
2. Server loads plugin policy from database
3. Creates `PluginKeysignRequestEvm` from transaction hex
4. Uses verifier or plugin emitter to sign immediately
5. Returns signature synchronously

## Event Streaming

WebSocket endpoint at `/events`:

```javascript
// Subscribe to system events
{
  "type": "subscribe",
  "data": {
    "channel": "system_events",
    "last_seen": 1234567890000  // Unix timestamp in ms
  }
}
```

Server behavior:
- Replays missed events from PostgreSQL based on `last_seen`
- Streams new events in real-time (1-second polling)
- Event types: vault reshares, deletions, policy creations/deletions

## Policy Signature Verification

All policy operations verify ECDSA signatures:

1. Policy converted to message hex via `common.PolicyToMessageHex`
2. Vault loaded from S3 storage and decrypted
3. Public key derived using `tss.GetDerivedPubKey` with Ethereum derivation path
4. Signature verified via `common.VerifyPolicySignature`

## Important Notes

- **Vault Encryption**: All vaults in S3 are encrypted using `encryption_secret` from config
- **Session Deduplication**: Redis prevents duplicate TSS sessions (keysign/reshare)
- **DKLS vs ECDSA**: System uses DKLS for TSS but supports ECDSA and EdDSA addresses
- **Task Retention**: Asynq tasks retained for 5-10 minutes after completion
- **Chain Codes**: Vault hex chain codes enable deterministic key derivation
- **Plugin ID**: Each deployment has unique plugin ID for vault/policy namespacing

## License

Copyright © 2025 Vultisig