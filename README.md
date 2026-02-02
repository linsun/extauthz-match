# ExtAuth Match

A demo project that implements Envoy's external authorization (ext_authz) gRPC interface with a Tinder-like swipeable web UI. Swipe right to approve requests, swipe left to deny them!

## Features

- 🔐 **Envoy ext_authz gRPC server** - Implements the official Envoy authorization API
- 📱 **Mobile-friendly swipe UI** - Approve/deny requests with swipe gestures or buttons
- ⚡ **Real-time WebSocket** - Instant delivery of authorization requests to your browser
- 🐳 **One-command setup** - Complete Docker Compose stack
- ⏱️ **30s timeout** - Auto-deny for requests awaiting approval too long

## Quick Start

1. **Start everything:**
   ```bash
   docker compose up --build
   ```

2. **Open the swipe UI on your phone/browser:**
   ```
   http://localhost:8080
   ```
   Or from your phone: `http://<your-ip>:8080`

3. **Make a request to the protected backend:**
   ```bash
   curl http://localhost:10000/
   ```

4. **Swipe right (✓) to approve or left (✗) to deny!**

## Architecture

```
┌─────────────┐
│   Browser   │ ←─────── WebSocket ────────┐
│  (Port 8080)│                             │
└─────────────┘                             │
                                            │
┌─────────────┐         ┌──────────────────▼──┐
│   Client    │────────▶│      Envoy Proxy    │
│             │         │     (Port 10000)     │
└─────────────┘         └──────────┬───────────┘
                                   │
                          ext_authz│gRPC
                                   │
                        ┌──────────▼──────────┐
                        │  AuthZ Server        │
                        │  - gRPC: 9000        │
                        │  - HTTP: 8080        │
                        └──────────┬───────────┘
                                   │
                                   ▼
                        ┌──────────────────────┐
                        │  Backend (nginx)     │
                        └──────────────────────┘
```

## Services

- **authz-server** (ports 8080, 9000) - Go service providing ext_authz gRPC API and web UI
- **envoy** (ports 10000, 9901) - Envoy proxy with ext_authz filter
- **backend** (internal) - Simple nginx serving a protected page

## Endpoints

- `http://localhost:8080` - Swipe UI for approving/denying requests
- `http://localhost:10000` - Envoy proxy (protected by ext_authz)
- `http://localhost:9901` - Envoy admin interface

## How It Works

1. Client makes request to Envoy (`:10000`)
2. Envoy calls ext_authz gRPC service (`:9000`)
3. Auth service queues request and broadcasts via WebSocket
4. User sees request card in browser and swipes
5. Decision sent back via WebSocket
6. Auth service responds to Envoy
7. Envoy allows/denies the original request

## Development

**Run locally without Docker:**

```bash
# Terminal 1: Start auth server
go run cmd/server/main.go

# Terminal 2: Start Envoy
envoy -c envoy.yaml

# Terminal 3: Start backend
cd backend && python3 -m http.server 80
```

## Testing

```bash
# Approved request (swipe right in UI)
curl -v http://localhost:10000/

# Denied request (swipe left in UI)
curl -v http://localhost:10000/api/test

# Multiple rapid requests
for i in {1..5}; do curl http://localhost:10000/test$i & done
```

## Configuration

Edit `envoy.yaml` to customize:
- Timeout (default: 35s)
- Routes and clusters
- Access logging

## Notes

- This is a **demo/proof-of-concept** - not production-ready
- Requests timeout after 30 seconds if not approved
- FIFO queue - first request in, first request shown
- Single user only (no multi-user approval)

## License

MIT
