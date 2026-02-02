# AGENTS.md - Project Context for AI Assistants

This document provides comprehensive context about the ExtAuth Match project for AI assistants working on future sessions.

## Project Overview

**ExtAuth Match** is a demo implementation of Envoy's external authorization (ext_authz) gRPC interface with a unique Tinder-like swipeable web UI. Users can swipe right to approve or left to deny authorization requests in real-time.

### Core Purpose
- Demonstrate Envoy ext_authz integration patterns
- Provide a mobile-friendly authorization approval interface
- Support secure cloud deployment with end-to-end encryption
- Enable multi-tenant relay architecture for public access

## Architecture Evolution

### Phase 1: Local WebSocket (Initial Implementation)
```
Browser (localhost:8080) ←→ WebSocket ←→ AuthZ Server (9000, 8080)
                                              ↑
                                              | ext_authz gRPC
                                              |
                                          Envoy (10000)
                                              ↑
                                              |
                                          Client
```

### Phase 2: Relay with E2E Encryption (Current)
```
Browser ←--encrypted-ws--→ Relay Server ←--encrypted-ws--→ AuthZ Server
                            (9090)                            (9000)
                                                                ↑
                                                                | gRPC
                                                                |
                                                            Envoy (10000)
```

**Key Change**: Transitioned from local WebSocket to relay-based architecture to enable:
- Cloud deployment of relay server
- Multi-tenant support (multiple authz servers can use one relay)
- End-to-end encryption (relay never sees plaintext)
- Mobile access without VPN/tunneling

## Technical Stack

### Languages & Frameworks
- **Go 1.22+**: Backend services (authz-server, relay-server)
- **JavaScript**: Browser-side encryption and UI
- **Envoy v1.29**: Proxy with ext_authz filter
- **Docker Compose**: Service orchestration

### Key Dependencies
- `github.com/envoyproxy/go-control-plane`: Envoy ext_authz v3 gRPC API
- `github.com/gorilla/websocket v1.5.3`: WebSocket communication
- `github.com/gorilla/mux v1.8.1`: HTTP routing (relay server)
- `google.golang.org/grpc`: gRPC server implementation

### Crypto Stack
- **AES-256-GCM**: Symmetric encryption for all messages
- **SHA256**: Tenant ID derivation from encryption keys
- **Web Crypto API**: Browser-side encryption/decryption
- **crypto/rand**: Secure key generation

## File Structure & Component Details

### Core Services

#### `cmd/server/main.go` - AuthZ Server
**Purpose**: Main ext_authz gRPC server with relay connectivity
**Key Functions**:
- Generates 256-bit AES encryption key on startup
- Derives tenant ID via SHA256(key)[:12] (24 hex chars)
- Connects to relay server as "server" role
- Displays ASCII QR code with URL containing tenant ID and key
- Implements gRPC ext_authz v3 API
- Encrypts authorization requests before sending to relay
- Decrypts responses from browser

**Important Details**:
- Key is base64-URL encoded for URL safety
- QR code displays: `http://relay:9090/s/{tenantID}#key={base64Key}`
- URL fragment (#key=...) is client-side only, never sent to server
- Graceful shutdown closes relay connection

#### `cmd/relay/main.go` - Relay Server
**Purpose**: Multi-tenant WebSocket relay for cloud deployment
**Key Functions**:
- Routes: `/ws/server/{tenantID}` (authz servers), `/ws/client/{tenantID}` (browsers), `/s/{tenantID}` (serves HTML)
- Maintains `Tenant` structs with server/client connections per tenant ID
- Forwards encrypted messages bidirectionally without decryption
- Handles connection lifecycle (upgrades, disconnects, cleanup)

**Tenant Structure**:
```go
type Tenant struct {
    ID            string
    ServerConn    *websocket.Conn  // AuthZ server connection
    ClientConn    *websocket.Conn  // Browser connection
    mu            sync.RWMutex     // Thread-safe access
}
```

**Important Details**:
- Uses gorilla/mux for routing
- Stores static HTML inline (web/static/index.html embedded as string)
- Cleans up tenant on disconnect
- No authentication (relay trusts first-come-first-served per tenant ID)
- CORS enabled for browser WebSocket upgrades

#### `internal/auth/service.go` - AuthZ Service Logic
**Purpose**: Implements ext_authz gRPC service interface
**Key Interface**: 
```go
type RelayClient interface {
    SendRequest(ctx context.Context, req *Request) (*Response, error)
}
```

**Flow**:
1. Receives gRPC `Check()` call from Envoy
2. Creates authorization request with method, path, headers
3. Calls `relayClient.SendRequest()` with 30s timeout
4. Returns `CheckResponse` (OK/DENIED) to Envoy

**Important Details**:
- 30-second timeout for user approval
- Auto-denies on timeout or error
- Extracts request metadata (method, path, headers)
- Single-request-at-a-time (no queueing in current implementation)

#### `internal/relay/client.go` - Relay Client
**Purpose**: AuthZ server's client for relay communication
**Key Functions**:
- `Connect()`: Establishes WebSocket to relay `/ws/server/{tenantID}`
- `SendRequest()`: Encrypts request JSON, sends to relay, waits for encrypted response
- `readMessages()`: Background goroutine reading responses from relay

**Message Flow**:
1. Encrypt request with AES-256-GCM
2. Base64-encode ciphertext
3. Send JSON: `{"type": "request", "data": "base64..."}`
4. Wait for response with request ID
5. Decode base64, decrypt with AES-256-GCM
6. Return to auth service

**Important Details**:
- Uses `pendingRequests map[string]chan *Response` for request-response matching
- Request ID generated via timestamp-based approach
- Handles relay disconnects gracefully
- Thread-safe with mutex protection

#### `internal/crypto/aes.go` - Encryption Utilities
**Purpose**: AES-256-GCM encryption/decryption functions

**Key Functions**:
```go
GenerateKey() ([]byte, error)                              // 32-byte random key
DeriveTenantID(key []byte) string                          // SHA256(key)[:12] as hex
Encrypt(key []byte, plaintext []byte) ([]byte, error)      // AES-GCM encrypt
Decrypt(key []byte, ciphertext []byte) ([]byte, error)     // AES-GCM decrypt
EncryptString(key []byte, plaintext string) (string, error) // Base64 output
DecryptString(key []byte, ciphertext string) (string, error) // Base64 input
EncodeKey(key []byte) string                               // Base64-URL encode
DecodeKey(encoded string) ([]byte, error)                  // Base64-URL decode
```

**Important Details**:
- Nonce prepended to ciphertext (first N bytes where N = gcm.NonceSize())
- Random nonce per encryption (never reuse)
- Base64 standard encoding for message data
- Base64-URL encoding for keys in URLs (no +/= issues)

#### `web/static/index.html` - Browser UI
**Purpose**: Swipe interface with client-side encryption

**Key Features**:
- Web Crypto API for AES-256-GCM
- Extracts key from URL fragment: `window.location.hash.substring(5)` (skip "#key=")
- WebSocket connection to `/ws/client/{tenantID}`
- Swipe gestures (touch) and button clicks
- Card animation and state management

**Crypto Flow**:
1. Extract base64-URL key from fragment
2. Convert to ArrayBuffer via base64 decode
3. Import as CryptoKey for AES-GCM
4. Decrypt incoming messages: base64 → decrypt → JSON parse
5. Encrypt outgoing: JSON stringify → encrypt → base64

**Important Details**:
- Key never sent to server (URL fragment not transmitted)
- Derives tenant ID client-side: SHA256(key).slice(0,12) as hex
- Queue system for multiple pending requests
- Auto-updates button states based on queue
- Mobile-responsive design with touch events

## Environment Variables

### AuthZ Server
- `RELAY_URL`: WebSocket URL of relay server (e.g., `ws://relay-server:9090`)
  - Defaults to `ws://localhost:9090` if not set
  - Use `wss://` for production TLS

### Relay Server
- `PORT`: HTTP listen port (default: `9090`)

## Docker Architecture

### docker-compose.yml Services

1. **relay-server**
   - Image: Built from `Dockerfile.relay`
   - Port: 9090 (host:container)
   - Role: Multi-tenant WebSocket relay

2. **authz-server**
   - Image: Built from `Dockerfile`
   - Port: 9000 (gRPC only)
   - Environment: `RELAY_URL=ws://relay-server:9090`
   - Depends on: relay-server

3. **envoy**
   - Image: `envoyproxy/envoy:v1.29-latest`
   - Ports: 10000 (proxy), 9901 (admin)
   - Config: `./envoy/envoy.yaml`
   - Depends on: authz-server

4. **backend**
   - Image: `nginx:alpine`
   - Port: 8081
   - Role: Protected upstream service

### Dockerfiles

#### `Dockerfile` (AuthZ Server)
- Multi-stage build
- Stage 1: `golang:1.24-alpine` - Build binary
- Stage 2: `alpine:latest` - Runtime with ca-certificates
- Binary: `/app/server`
- Web assets: `/app/web/static/`

#### `Dockerfile.relay` (Relay Server)
- Similar multi-stage build
- Binary: `/app/relay`

## Key Design Decisions

### 1. Why Relay Architecture?
**Problem**: Direct WebSocket from authz to browser requires:
- Public IP for authz server
- Firewall configuration
- VPN for mobile access

**Solution**: Relay server acts as rendezvous point:
- Only relay needs public access
- AuthZ server can be anywhere with outbound connectivity
- Browser connects to stable public endpoint

### 2. Why End-to-End Encryption?
**Problem**: Relay server could inspect authorization requests (privacy concern)

**Solution**: Encrypt before sending to relay:
- Relay only forwards opaque encrypted blobs
- Key distributed via URL fragment (client-side only)
- Zero-trust architecture: don't trust relay server

### 3. Why URL Fragment for Key?
**Problem**: How to securely share encryption key with browser?

**Solution**: URL fragment (#key=...) properties:
- Never sent in HTTP requests (browser-only)
- Can be scanned via QR code
- No server-side storage needed
- Ephemeral (changes on each authz restart)

### 4. Why Tenant ID from Key Hash?
**Problem**: Need unique tenant ID but can't use key directly (security)

**Solution**: SHA256(key)[:12]:
- Deterministic (browser can compute same ID)
- One-way (can't reverse to get key)
- Collision-resistant for demo purposes
- Short enough for URLs (24 hex chars)

### 5. Why No Persistent Queue?
**Decision**: Single in-flight request per tenant

**Rationale**:
- Demo simplicity
- Mobile UX: focus on one decision at a time
- Timeout prevents request buildup
- Production would add Redis/database queue

## Common Pitfalls & Solutions

### Issue: "undefined: qrcode.Generate"
**Cause**: Missing or corrupted qrcode package
**Fix**: Recreate `internal/qrcode/qrcode.go` - it was getting corrupted during edits

### Issue: "syntax error: non-declaration statement outside function body"
**Cause**: Duplicate `package` declaration or reversed file content
**Fix**: Delete and recreate file - text editor corrupted the file

### Issue: Context timeout in auth service
**Cause**: No browser connected or browser not responding
**Fix**: Ensure browser WebSocket connected, check console for errors

### Issue: Relay connection refused
**Cause**: Relay server not started before authz server
**Fix**: Ensure relay starts first via `depends_on` in docker-compose

## Testing the System

### End-to-End Test
```bash
# 1. Start all services
docker compose up

# 2. Get browser URL from logs
docker compose logs authz-server | grep "Browser URL"

# 3. Open URL on phone (scan QR or copy URL)

# 4. Make test request
curl http://localhost:10000/

# 5. Swipe right on phone to approve

# 6. Observe curl receives response
```

### Component Testing
```bash
# Test relay server
curl http://localhost:9090/s/test-tenant-id

# Test Envoy admin
curl http://localhost:9901/stats

# Test gRPC directly (requires grpcurl)
grpcurl -plaintext -d '{"attributes": {...}}' localhost:9000 envoy.service.auth.v3.Authorization/Check
```

## Future Enhancement Ideas

1. **Persistent Queue**: Redis-backed request queue for multiple pending requests
2. **Request History**: Log of approved/denied requests with timestamps
3. **Real QR Codes**: Use proper QR code library instead of ASCII art
4. **Authentication**: Add JWT or API keys for relay access
5. **Rate Limiting**: Prevent abuse of relay server
6. **Metrics**: Prometheus metrics for authorization latency
7. **UI Improvements**: Request details (headers, body preview), dark mode
8. **Mobile App**: Native iOS/Android apps with push notifications

## Debugging Tips

### View All Logs
```bash
docker compose logs -f
```

### View Specific Service
```bash
docker compose logs -f authz-server
docker compose logs -f relay-server
```

### Check WebSocket Connection
Browser console should show:
```
Connected to relay!
Tenant ID: 516100ad100e649c3edf0ee9
```

### Inspect Envoy ext_authz Calls
```bash
# Check ext_authz cluster stats
curl -s http://localhost:9901/stats | grep ext_authz

# Check if authz server is healthy
curl http://localhost:9901/clusters
```

### Test Encryption Locally
```go
// In Go
key, _ := crypto.GenerateKey()
encrypted, _ := crypto.EncryptString(key, "test")
decrypted, _ := crypto.DecryptString(key, encrypted)
fmt.Println(decrypted) // "test"
```

## Port Reference

- **9000**: AuthZ gRPC server (internal)
- **9090**: Relay WebSocket server (public)
- **10000**: Envoy proxy (client-facing)
- **9901**: Envoy admin interface
- **8081**: Backend nginx (internal)

## Important Files for Modification

### To Change Authorization Logic
Edit: `internal/auth/service.go` - `Check()` method

### To Change Encryption Algorithm
Edit: `internal/crypto/aes.go` - Use different cipher

### To Change UI
Edit: `web/static/index.html` - HTML/CSS/JS in single file

### To Change Relay Behavior
Edit: `cmd/relay/main.go` - Message forwarding logic

### To Change Envoy Config
Edit: `envoy/envoy.yaml` - Proxy rules, ext_authz cluster

## Project Status

**Current State**: Fully functional demo
- ✅ Relay architecture implemented
- ✅ End-to-end encryption working
- ✅ Multi-tenant support functional
- ✅ Mobile-friendly UI complete
- ✅ Docker Compose setup working
- ✅ QR code generation (ASCII)

**Known Limitations**:
- ASCII QR codes (not scannable by phone cameras)
- No persistent storage
- Single request at a time
- No authentication on relay
- localhost URLs (need to update for cloud)

**Production Readiness**: 🔴 Demo only
- Missing: Authentication, rate limiting, monitoring, proper QR codes
- Security: Relay has no access control
- Scalability: In-memory state only
- For production: Add Redis, proper QR library, TLS, auth, monitoring

## Context for Future Sessions

When working on this project in future sessions:

1. **Architecture is relay-based**: Don't suggest going back to direct WebSocket
2. **Encryption is mandatory**: All relay messages must be encrypted
3. **URL fragment is sacred**: Never move key to query params or headers
4. **Tenant ID derivation**: Always use SHA256(key)[:12] for consistency
5. **Go module path**: `github.com/yuval/extauth-match` (update if forking)
6. **Docker first**: Prefer containerized testing over local Go builds
7. **Single HTML file**: UI is intentionally one file for simplicity

## Quick Reference Commands

```bash
# Full rebuild and start
docker compose up --build

# Start in background
docker compose up -d

# View logs
docker compose logs -f authz-server

# Stop all
docker compose down

# Test request
curl http://localhost:10000/

# Check Envoy stats
curl http://localhost:9901/stats | grep ext_authz

# Rebuild specific service
docker compose build authz-server

# Shell into container
docker compose exec authz-server sh
```

## Summary

ExtAuth Match is a production-ready **demo** of Envoy ext_authz with unique UX and security features. The relay architecture enables cloud deployment while E2E encryption ensures privacy. The codebase is well-structured for both learning and extension.

For questions or modifications, refer to the component details above and remember: **encryption key never touches the relay server**.
