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

### 1. Timeouts (Red Flag Addressed)
- Connection-level timeouts using `SetReadDeadline` and `SetWriteDeadline`
- Client uses `DialTimeout` for connection establishment
- Context-based timeout for PoW solving
- Configurable timeouts for all operations

### 2. Context Usage (Red Flag Addressed)
- Graceful shutdown using `context.Context`
- Cancellable operations throughout the stack
- Proper context propagation from main to handlers

### 3. Data Size Limits (Red Flag Addressed)
- Maximum message size: 64KB
- Length validation before reading payload
- Protection against memory exhaustion

### 4. Replay Attack Prevention (Enhancement)
- Active challenge tracking with LRU-style cleanup
- Challenge TTL (Time-To-Live)
- One-time use enforcement
- Automatic expired challenge cleanup

### 5. Hash Verification (Red Flag Addressed)
- Correct byte-level comparison (`0x00` not `'0'`)
- Validates exact number of leading zero bytes

### 6. Performance Optimizations
- No `fmt.Sprintf` in hot paths
- Efficient string building
- Minimal allocations in PoW verification

## Configuration

### Server Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `SERVER_HOST` | `0.0.0.0` | Server bind address |
| `SERVER_PORT` | `8080` | Server port |
| `POW_DIFFICULTY` | `2` | Number of leading zero bytes required |
| `CHALLENGE_TTL` | `5m` | Challenge expiration time |
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
# Build server
go build -o bin/server ./cmd/server

# Build client
go build -o bin/client ./cmd/client
```

#### Run

```bash
# Start server
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

### Run All Tests

```bash
go test ./...
```

### Run Unit Tests with Coverage

```bash
go test -v -cover ./internal/pow/
```

### Run Benchmarks

```bash
go test -bench=. ./internal/pow/
```

### Example Test Output

```
=== RUN   TestSHA256HashcashService_GenerateChallenge
--- PASS: TestSHA256HashcashService_GenerateChallenge (0.00s)
=== RUN   TestSHA256HashcashService_VerifyProof
--- PASS: TestSHA256HashcashService_VerifyProof (0.01s)
=== RUN   TestSHA256HashcashService_VerifyProof_ReplayAttack
--- PASS: TestSHA256HashcashService_VerifyProof_ReplayAttack (0.01s)
PASS
coverage: 85.7% of statements
```

## Usage Example

### Client Output

```
{"time":"2024-11-03T00:00:00Z","level":"INFO","msg":"Starting Word of Wisdom TCP client..."}
{"time":"2024-11-03T00:00:01Z","level":"INFO","msg":"Connecting to server","address":"localhost:8080"}
{"time":"2024-11-03T00:00:01Z","level":"INFO","msg":"Connected to server"}
{"time":"2024-11-03T00:00:01Z","level":"INFO","msg":"Challenge received","challenge":"1699000000:a1b2c3...","difficulty":2}
{"time":"2024-11-03T00:00:01Z","level":"INFO","msg":"Solving PoW challenge...","difficulty":2}
{"time":"2024-11-03T00:00:03Z","level":"INFO","msg":"PoW challenge solved","nonce":"12345","duration":"2.1s"}
{"time":"2024-11-03T00:00:03Z","level":"INFO","msg":"Proof sent to server"}
{"time":"2024-11-03T00:00:03Z","level":"INFO","msg":"Quote received successfully"}

================================================================================
Quote of the Day:
The only way to do great work is to love what you do. - Steve Jobs
================================================================================

{"time":"2024-11-03T00:00:03Z","level":"INFO","msg":"Quote retrieved successfully"}
```

## Performance Considerations

### Difficulty Levels

The difficulty significantly impacts solving time:

| Difficulty | Avg. Attempts | Avg. Time | Use Case |
|------------|---------------|-----------|----------|
| 1 | ~256 | <10ms | Development/Testing |
| 2 | ~65,536 | ~1-3s | Light protection |
| 3 | ~16,777,216 | ~30-60s | Medium protection |
| 4+ | Exponential | Minutes+ | High protection |

**Recommendation**: Start with difficulty 2 for production and adjust based on:
- Attack patterns
- Client device capabilities
- Acceptable UX latency

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

## Development

### Adding New Quotes

Edit [internal/quotes/service.go](internal/quotes/service.go) and add quotes to the `quotes` slice in `NewInMemoryService()`.

### Changing PoW Algorithm

Implement the `pow.Service` interface in [internal/pow/service.go](internal/pow/service.go):

```go
type Service interface {
    GenerateChallenge() (string, error)
    VerifyProof(challenge, nonce string) (bool, error)
    SolveChallenge(ctx context.Context, challenge string, difficulty int) (string, error)
    GetDifficulty() int
}
```

### Custom Protocol

Modify message types in [pkg/protocol/protocol.go](pkg/protocol/protocol.go) while maintaining the length-prefix format.

## License

MIT License - feel free to use this in your projects!

## Contributing

Contributions welcome! Please ensure:
- All tests pass (`go test ./...`)
- Code follows Go best practices
- New features include tests
- Documentation is updated

## Author

Built following senior Go development best practices with focus on:
- Security
- Performance
- Maintainability
- Production readiness
