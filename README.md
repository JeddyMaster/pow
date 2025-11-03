# Word of Wisdom TCP Server with Proof of Work

A production-ready TCP server implementation that protects against DDoS attacks using a Proof of Work (PoW) challenge-response protocol. The server provides random wisdom quotes to clients who successfully solve the computational challenge.

## Features

- **DDoS Protection**: Hashcash-based Proof of Work algorithm prevents abuse
- **Production Ready**: Comprehensive error handling, timeouts, and graceful shutdown
- **Secure**: Protection against replay attacks with challenge TTL and active challenge tracking
- **Scalable**: Configurable connection limits and concurrent request handling
- **Observable**: Structured JSON logging with slog
- **Docker Support**: Complete Docker and Docker Compose setup for easy deployment

## Architecture

### Project Structure

```
pow/
├── cmd/
│   ├── server/          # Server entry point
│   └── client/          # Client entry point
├── internal/
│   ├── config/          # Configuration management
│   ├── pow/             # Proof of Work implementation
│   ├── quotes/          # Quote service
│   ├── server/          # TCP server logic
│   └── client/          # TCP client logic
├── pkg/
│   └── protocol/        # Network protocol definitions
├── Dockerfile.server    # Server Docker image
├── Dockerfile.client    # Client Docker image
└── docker-compose.yaml  # Orchestration configuration
```

### Protocol Design

The protocol uses a binary format with length-prefixed JSON messages:

1. **Length Prefix**: 4 bytes (Little-Endian uint32) indicating message size
2. **JSON Payload**: The actual message data

#### Message Types

```go
// Challenge sent by server
{
  "type": "challenge",
  "challenge": "1699000000:a1b2c3d4e5f6...",
  "difficulty": 2
}

// Proof sent by client
{
  "type": "proof",
  "challenge": "1699000000:a1b2c3d4e5f6...",
  "nonce": "42"
}

// Quote sent by server
{
  "type": "quote",
  "quote": "The only way to do great work is to love what you do. - Steve Jobs"
}

// Error message
{
  "type": "error",
  "message": "Invalid proof"
}
```

### Proof of Work Algorithm

**Implementation**: SHA-256 Hashcash

The algorithm works as follows:

1. **Challenge Generation**: Server generates a unique challenge containing:
   - Timestamp (Unix epoch)
   - Random 16-byte hex string
   - Format: `{timestamp}:{random_hex}`

2. **Challenge Solving**: Client must find a nonce such that:
   - `SHA256(challenge + nonce)` starts with N zero bytes
   - N is the difficulty level (configurable)

3. **Proof Verification**: Server validates:
   - Challenge exists and hasn't expired
   - Hash has required leading zeros
   - Challenge hasn't been used before (replay attack prevention)

**Example**:
- Difficulty 1: Hash must start with 1 zero byte (00...)
- Difficulty 2: Hash must start with 2 zero bytes (0000...)

## Security Features

### 1. DDoS Protection
- **Proof of Work**: SHA-256 Hashcash algorithm requiring computational effort
- **Challenge Limit**: Maximum 100,000 active challenges (configurable via `MAX_ACTIVE_CHALLENGES`)
- **Connection Limit**: Configurable max concurrent connections
- **Memory Protection**: Challenges invalidated on connection failure to prevent exhaustion

### 2. Replay Attack Prevention
- **One-time Use**: Each challenge can only be used once
- **Challenge TTL**: Automatic expiration (default: 5 minutes)
- **Active Tracking**: Per-connection challenge validation
- **Auto Cleanup**: Background goroutine removes expired challenges every TTL/2

### 3. Timeout Protection
- **Connection Timeouts**: `SetReadDeadline` and `SetWriteDeadline` on all operations
- **Dial Timeout**: Client connection establishment timeout
- **Solve Timeout**: Context-based PoW solving with cancellation
- **Graceful Shutdown**: Waits for active connections with timeout

### 4. Protocol Security
- **Size Limits**: Maximum message size of 64KB
- **Length Validation**: Validates length before reading payload
- **Complete Writes**: Ensures all bytes are written (handles partial writes)
- **Hash Verification**: Byte-level comparison of leading zeros

### 5. Code Quality
- **Interface Segregation**: Separate `ChallengeService` (server) and `SolverService` (client) interfaces
- **Thread Safety**: `sync.RWMutex` protects shared state
- **No Magic Numbers**: All constants defined and documented
- **Race-Free**: Verified with `-race` detector

## Configuration

### Server Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `SERVER_HOST` | `0.0.0.0` | Server bind address |
| `SERVER_PORT` | `8080` | Server port |
| `POW_DIFFICULTY` | `2` | Number of leading zero bytes required (1-5) |
| `CHALLENGE_TTL` | `5m` | Challenge expiration time |
| `MAX_ACTIVE_CHALLENGES` | `100000` | Maximum number of active challenges |
| `READ_TIMEOUT` | `30s` | Read operation timeout |
| `WRITE_TIMEOUT` | `10s` | Write operation timeout |
| `MAX_CONNECTIONS` | `100` | Maximum concurrent connections |
| `SHUTDOWN_TIMEOUT` | `30s` | Graceful shutdown timeout |

### Client Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `SERVER_HOST` | `localhost` | Server address |
| `SERVER_PORT` | `8080` | Server port |
| `CONNECT_TIMEOUT` | `10s` | Connection timeout |
| `READ_TIMEOUT` | `30s` | Read operation timeout |
| `WRITE_TIMEOUT` | `10s` | Write operation timeout |
| `SOLVE_TIMEOUT` | `5m` | PoW solving timeout |

## Quick Start

### Using Docker Compose (Recommended)

1. **Start the server and run client**:
```bash
docker-compose up --build
```

2. **Run only the server**:
```bash
docker-compose up server
```

3. **Run client separately**:
```bash
docker-compose run client
```

### Manual Build and Run

#### Prerequisites
- Go 1.21 or later

#### Build

```bash
# Using Makefile
make build

# Or manually
go build -o bin/server ./cmd/server
go build -o bin/client ./cmd/client
```

#### Run

```bash
# Start server (loads .env file automatically)
./bin/server

# In another terminal, run client
./bin/client
```

### Using Docker

#### Build Images

```bash
# Build server image
docker build -f Dockerfile.server -t pow-server .

# Build client image
docker build -f Dockerfile.client -t pow-client .
```

#### Run Containers

```bash
# Run server
docker run -p 8080:8080 \
  -e POW_DIFFICULTY=2 \
  pow-server

# Run client
docker run \
  -e SERVER_HOST=host.docker.internal \
  -e SERVER_PORT=8080 \
  pow-client
```

## Testing

```bash
# Run all tests
go test ./...

# Run with race detector
go test -race ./...

# Run with coverage
go test -cover ./...
```

## Performance Considerations

### Difficulty Levels

The difficulty significantly impacts solving time:

| Difficulty | Avg. Attempts | Avg. Time | Use Case |
|------------|---------------|-----------|----------|
| 1 | ~256 | ~0.2ms | Development/Testing |
| 2 | ~65,536 | ~15ms | Light protection (recommended) |
| 3 | ~16,777,216 | ~1.3s | Medium protection |
| 4 | ~4,294,967,296 | ~8 min | High protection |
| 5 | Astronomical | Hours+ | Maximum protection |

**Recommendation**: Start with difficulty 2 for production and adjust based on:
- Attack patterns
- Client device capabilities
- Acceptable UX latency

**Note**: Each increase in difficulty multiplies solving time by ~256×

### Scalability

- Server handles connections concurrently using goroutines
- Configurable `MAX_CONNECTIONS` prevents resource exhaustion
- Each connection has independent timeouts
- Minimal memory footprint per connection

## Troubleshooting

### Client can't connect

```bash
# Check if server is running
docker-compose ps

# View server logs
docker-compose logs server

# Test connectivity
telnet localhost 8080
```

### PoW solving takes too long

- Reduce `POW_DIFFICULTY` environment variable
- Increase `SOLVE_TIMEOUT` for slower clients
- Consider client hardware capabilities

### Server under heavy load

- Increase `MAX_CONNECTIONS`
- Increase `POW_DIFFICULTY` to slow down attackers
- Monitor active connections and adjust
