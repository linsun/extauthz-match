package relay

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/yuval/extauth-match/internal/crypto"
)

// DecisionHandler is a callback for handling authorization decisions
type DecisionHandler func(requestID string, approved bool)

// Client represents a relay client that connects authz server to the relay
type Client struct {
	relayURL        string
	tenantID        string
	encryptionKey   []byte
	conn            *websocket.Conn
	decisionHandler DecisionHandler
	mu              sync.RWMutex
	maxRetries      int
	retryDelay      time.Duration
}

// NewClient creates a new relay client
func NewClient(relayURL, tenantID string, encryptionKey []byte) (*Client, error) {
	return &Client{
		relayURL:      relayURL,
		tenantID:      tenantID,
		encryptionKey: encryptionKey,
		maxRetries:    3,
		retryDelay:    time.Second,
	}, nil
}

// SetDecisionHandler sets the handler for authorization decisions
func (c *Client) SetDecisionHandler(handler DecisionHandler) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.decisionHandler = handler
}

// Connect establishes WebSocket connection to relay
func (c *Client) Connect() error {
	wsURL := fmt.Sprintf("%s/ws/server/%s", c.relayURL, c.tenantID)

	var err error
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		return fmt.Errorf("failed to connect to relay: %w", err)
	}

	c.mu.Lock()
	c.conn = conn
	c.mu.Unlock()

	slog.Info("Connected to relay as server", "tenantID", c.tenantID)

	// Start reading messages from relay
	go c.readMessages()

	return nil
}

// SendRequest sends an encrypted auth request to the browser
func (c *Client) SendRequest(requestData interface{}) error {
	// Marshal to JSON
	plaintext, err := json.Marshal(requestData)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	// Encrypt
	ciphertext, err := crypto.Encrypt(c.encryptionKey, plaintext)
	if err != nil {
		return fmt.Errorf("failed to encrypt request: %w", err)
	}

	// Try to send with retry logic
	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		c.mu.RLock()
		conn := c.conn
		c.mu.RUnlock()

		if conn == nil {
			return fmt.Errorf("not connected to relay")
		}

		if err := conn.WriteMessage(websocket.BinaryMessage, ciphertext); err != nil {
			// If connection is broken, try to reconnect
			if attempt < c.maxRetries {
				slog.Warn("Failed to send to relay, attempting reconnect", "attempt", attempt+1, "error", err)

				// Close existing connection
				c.mu.Lock()
				if c.conn != nil {
					c.conn.Close()
					c.conn = nil
				}
				c.mu.Unlock()

				// Wait before retrying
				time.Sleep(c.retryDelay)

				// Attempt to reconnect
				if reconnectErr := c.Connect(); reconnectErr != nil {
					slog.Error("Failed to reconnect to relay", "error", reconnectErr)
					continue
				}

				// Retry sending the message
				continue
			}

			return fmt.Errorf("failed to send to relay after %d attempts: %w", c.maxRetries+1, err)
		}

		// Success
		return nil
	}

	return fmt.Errorf("failed to send to relay after all retries")
}

// readMessages reads encrypted messages from relay (decisions from browser)
func (c *Client) readMessages() {
	for {
		c.mu.RLock()
		conn := c.conn
		c.mu.RUnlock()

		if conn == nil {
			return
		}

		_, message, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				slog.Error("Relay connection error", "error", err)
			}
			return
		}

		// Decrypt message
		plaintext, err := crypto.Decrypt(c.encryptionKey, message)
		if err != nil {
			slog.Error("Failed to decrypt message", "error", err)
			continue
		}

		// Parse decision
		var decision struct {
			RequestID string `json:"requestId"`
			Approved  bool   `json:"approved"`
		}

		if err := json.Unmarshal(plaintext, &decision); err != nil {
			slog.Error("Failed to unmarshal decision", "error", err)
			continue
		}

		// Call handler
		c.mu.RLock()
		handler := c.decisionHandler
		c.mu.RUnlock()

		if handler != nil {
			handler(decision.RequestID, decision.Approved)
		}
	}
}

// Close closes the relay connection
func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn != nil {
		c.conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
		time.Sleep(time.Second)
		c.conn.Close()
		c.conn = nil
	}

	return nil
}
