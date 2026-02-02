package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins for demo
	},
}

type Tenant struct {
	tenantID string
	server   *websocket.Conn
	client   *websocket.Conn
	mu       sync.RWMutex
}

type Relay struct {
	tenants map[string]*Tenant
	mu      sync.RWMutex
}

func NewRelay() *Relay {
	return &Relay{
		tenants: make(map[string]*Tenant),
	}
}

func (r *Relay) getTenant(tenantID string) *Tenant {
	r.mu.Lock()
	defer r.mu.Unlock()

	if tenant, exists := r.tenants[tenantID]; exists {
		return tenant
	}

	tenant := &Tenant{tenantID: tenantID}
	r.tenants[tenantID] = tenant
	return tenant
}

func (r *Relay) handleServerConnect(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	tenantID := vars["tenantID"]

	conn, err := upgrader.Upgrade(w, req, nil)
	if err != nil {
		log.Printf("Server upgrade failed for tenant %s: %v", tenantID, err)
		return
	}

	tenant := r.getTenant(tenantID)
	tenant.mu.Lock()
	tenant.server = conn
	tenant.mu.Unlock()

	log.Printf("Authz server connected for tenant: %s", tenantID)

	// Read from server and forward to client
	go r.forwardServerToClient(tenant)
}

func (r *Relay) handleClientConnect(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	tenantID := vars["tenantID"]

	conn, err := upgrader.Upgrade(w, req, nil)
	if err != nil {
		log.Printf("Client upgrade failed for tenant %s: %v", tenantID, err)
		return
	}

	tenant := r.getTenant(tenantID)
	tenant.mu.Lock()

	// Disconnect existing client if any
	if tenant.client != nil {
		tenant.client.Close()
	}
	tenant.client = conn
	tenant.mu.Unlock()

	log.Printf("Browser client connected for tenant: %s", tenantID)

	// Read from client and forward to server
	go r.forwardClientToServer(tenant)
}

func (r *Relay) forwardServerToClient(tenant *Tenant) {
	defer func() {
		tenant.mu.Lock()
		if tenant.server != nil {
			tenant.server.Close()
			tenant.server = nil
		}
		tenant.mu.Unlock()
		log.Printf("Authz server disconnected for tenant: %s", tenant.tenantID)
	}()

	for {
		tenant.mu.RLock()
		server := tenant.server
		tenant.mu.RUnlock()

		if server == nil {
			return
		}

		messageType, message, err := server.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("Server read error for tenant %s: %v", tenant.tenantID, err)
			}
			return
		}

		// Forward to client
		tenant.mu.RLock()
		client := tenant.client
		tenant.mu.RUnlock()

		if client != nil {
			if err := client.WriteMessage(messageType, message); err != nil {
				log.Printf("Failed to forward to client for tenant %s: %v", tenant.tenantID, err)
			} else {
				log.Printf("Forwarded %d bytes from server to client (tenant: %s)", len(message), tenant.tenantID)
			}
		}
	}
}

func (r *Relay) forwardClientToServer(tenant *Tenant) {
	defer func() {
		tenant.mu.Lock()
		if tenant.client != nil {
			tenant.client.Close()
			tenant.client = nil
		}
		tenant.mu.Unlock()
		log.Printf("Browser client disconnected for tenant: %s", tenant.tenantID)
	}()

	for {
		tenant.mu.RLock()
		client := tenant.client
		tenant.mu.RUnlock()

		if client == nil {
			return
		}

		messageType, message, err := client.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("Client read error for tenant %s: %v", tenant.tenantID, err)
			}
			return
		}

		// Forward to server
		tenant.mu.RLock()
		server := tenant.server
		tenant.mu.RUnlock()

		if server != nil {
			if err := server.WriteMessage(messageType, message); err != nil {
				log.Printf("Failed to forward to server for tenant %s: %v", tenant.tenantID, err)
			} else {
				log.Printf("Forwarded %d bytes from client to server (tenant: %s)", len(message), tenant.tenantID)
			}
		}
	}
}

func main() {
	relay := NewRelay()

	router := mux.NewRouter()
	router.HandleFunc("/ws/server/{tenantID}", relay.handleServerConnect)
	router.HandleFunc("/ws/client/{tenantID}", relay.handleClientConnect)

	// Serve static HTML for client
	router.HandleFunc("/s/{tenantID}", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "./web/static/index.html")
	})

	server := &http.Server{
		Addr:    ":9090",
		Handler: router,
	}

	go func() {
		log.Println("Relay server listening on :9090")
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start relay server: %v", err)
		}
	}()

	// Graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	log.Println("Shutting down relay server...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	server.Shutdown(ctx)
	log.Println("Relay server shutdown complete")
}
