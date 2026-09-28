// Package wss defines logic of multiple websocket streamers
package wss

import (
	"context"
	"crypto/subtle"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	ut "kyri56xcaesar/kuspace/internal/utils"
)

var (
	address    = "0.0.0.0:8082"
	jobLogPath = "data/logs/jobs/"
	// serviceSecret authenticates producers (uspace) and admin calls, and
	// keys the consumer tickets frontapp hands out (see utils.SignWSTicket)
	serviceSecret []byte

	// Registry maps jobIDs to their socket servers
	registry = struct {
		sync.Mutex
		servers map[string]*SocketServer
	}{
		servers: make(map[string]*SocketServer),
	}

	upgrader = websocket.Upgrader{CheckOrigin: sameHostOrigin}
)

// sameHostOrigin allows non-browser clients (no Origin) and browser pages
// served from the same host name (frontapp and wss differ only in port).
func sameHostOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}
	u, err := url.Parse(origin)
	if err != nil {
		return false
	}
	host, _, err := net.SplitHostPort(r.Host)
	if err != nil {
		host = r.Host
	}

	return strings.EqualFold(u.Hostname(), host)
}

// Role as in a string describing type of user
type Role string

const (
	// Producer the one who sends the broadcasted messages
	Producer Role = "producer"
	// Consumer the one who receives
	Consumer Role = "consumer"
	// JackOfAllTrades both
	JackOfAllTrades Role = "jack"
)

// Client represents a WebSocket connection
type Client struct {
	Jid       string
	Conn      *websocket.Conn
	Role      Role
	Send      chan []byte
	closeOnce sync.Once
}

// closeSend closes the client's outgoing queue exactly once: several paths
// (slow consumer, unregister, session delete) used to close it and the
// second close panicked, taking wss down.
func (c *Client) closeSend() {
	c.closeOnce.Do(func() { close(c.Send) })
}

// SocketServer manages clients for a specific job
type SocketServer struct {
	Producers  map[*Client]bool
	Consumers  map[*Client]bool
	Broadcast  chan []byte
	Register   chan *Client
	Unregister chan *Client
	done       chan struct{}
	sync.Mutex

	Jid     string
	Logger  *log.Logger
	logFile *os.File
}

// NewSocketServer the constructor for a SocketServer
func NewSocketServer(jid string) (*SocketServer, error) {
	if err := os.MkdirAll(jobLogPath, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create path to logs: %w", err)
	}
	logFile, err := os.OpenFile(filepath.Join(jobLogPath, "ws-server-"+jid+".log"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o640)
	if err != nil {
		return nil, fmt.Errorf("failed to open log file: %w", err)
	}

	return &SocketServer{
		Jid:        jid,
		Logger:     log.New(logFile, "[WS-"+jid+" WS-server] ", log.LstdFlags),
		logFile:    logFile,
		Producers:  make(map[*Client]bool),
		Consumers:  make(map[*Client]bool),
		Broadcast:  make(chan []byte),
		Register:   make(chan *Client),
		Unregister: make(chan *Client),
		done:       make(chan struct{}),
	}, nil
}

func getOrCreateServer(jobID string) (*SocketServer, error) {
	registry.Lock()
	defer registry.Unlock()
	server, exists := registry.servers[jobID]
	if !exists {
		var err error
		if server, err = NewSocketServer(jobID); err != nil {
			return nil, err
		}
		registry.servers[jobID] = server
		go server.Start()
	}

	return server, nil
}

// shutdown stops the server, disconnects everyone and releases its log file.
// Every job used to keep its goroutine and open log file forever.
func (s *SocketServer) shutdown() {
	registry.Lock()
	if registry.servers[s.Jid] == s {
		delete(registry.servers, s.Jid)
	}
	registry.Unlock()

	s.Lock()
	defer s.Unlock()
	select {
	case <-s.done:
		return // already shut down
	default:
		close(s.done)
	}
	for c := range s.Producers {
		_ = c.Conn.Close()
		c.closeSend()
	}
	for c := range s.Consumers {
		_ = c.Conn.Close()
		c.closeSend()
	}
	s.Producers, s.Consumers = map[*Client]bool{}, map[*Client]bool{}
	_ = s.logFile.Close()
}

// join registers a client, retrying on a fresh server if the one found was
// shutting down.
func join(jid string, client *Client) (*SocketServer, error) {
	for range 3 {
		server, err := getOrCreateServer(jid)
		if err != nil {
			return nil, err
		}
		select {
		case server.Register <- client:
			return server, nil
		case <-server.done:
		}
	}

	return nil, errors.New("session is shutting down")
}

// Start as in begin listening
func (s *SocketServer) Start() {
	for {
		select {
		case <-s.done:
			return
		case client := <-s.Register:
			s.Lock()
			switch client.Role {
			case Producer:
				s.Producers[client] = true
			case Consumer:
				s.Consumers[client] = true
			default:
				s.Producers[client] = true
				s.Consumers[client] = true
			}
			s.Unlock()
		case client := <-s.Unregister:
			s.Lock()
			delete(s.Producers, client)
			delete(s.Consumers, client)
			client.closeSend()
			empty := len(s.Producers) == 0 && len(s.Consumers) == 0
			s.Unlock()
			if empty {
				go s.shutdown() // not from inside Start's select
			}
		case msg := <-s.Broadcast:
			s.Lock()
			for consumer := range s.Consumers {
				select {
				case consumer.Send <- msg:
				default: // too slow: drop it
					consumer.closeSend()
					delete(s.Consumers, consumer)
				}
			}
			s.Unlock()
		}
	}
}

func hasServiceSecret(c *gin.Context) bool {
	got := c.GetHeader("X-Service-Secret")

	return got != "" && len(serviceSecret) > 0 && subtle.ConstantTimeCompare([]byte(got), serviceSecret) == 1
}

// HandleWSsession a handler for the endpoint
//
// Consumers need a ticket from frontapp (?ticket=...) for this job; producers
// (uspace) authenticate with the service secret.
func HandleWSsession(c *gin.Context) {
	id := c.Query("jid")
	roleStr := strings.ToLower(c.Query("role"))
	if id == "" || roleStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing jid or role"})

		return
	}
	role := Role(roleStr)
	switch role {
	case Producer:
		if !hasServiceSecret(c) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "producers must authenticate as a service"})

			return
		}
	case Consumer, JackOfAllTrades:
		if _, err := ut.VerifyWSTicket(serviceSecret, c.Query("ticket"), id, roleStr, time.Now()); err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "a valid ticket for this job is required"})

			return
		}
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid role"})

		return
	}
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	client := &Client{
		Jid:  id,
		Conn: conn,
		Role: role,
		Send: make(chan []byte, 256),
	}
	server, err := join(id, client)
	if err != nil {
		log.Printf("failed to join session %s: %v", id, err)
		_ = conn.Close()

		return
	}
	server.Logger.Printf("client registered: %v\n", client.Role)

	go writeMessages(client)
	go broadcastMessages(client, server)
}

// HandleWSsessionClose ends a job's session (services only).
func HandleWSsessionClose(c *gin.Context) {
	if !hasServiceSecret(c) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "service secret required"})

		return
	}
	jobID := c.Query("jid")
	if jobID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing jid"})

		return
	}
	registry.Lock()
	server, exists := registry.servers[jobID]
	registry.Unlock()
	if exists {
		server.shutdown()
	}
	c.JSON(http.StatusOK, gin.H{"status": "successfully deleted socket server"})
}

func broadcastMessages(client *Client, server *SocketServer) {
	defer func() {
		select {
		case server.Unregister <- client:
		case <-server.done:
		}
		if err := client.Conn.Close(); err != nil {
			log.Printf("failed to close connection: %v", err)
		}
	}()

	for {
		_, msg, err := client.Conn.ReadMessage()
		if err != nil {
			break
		}
		server.Logger.Printf("message read: %s", msg)
		if client.Role == JackOfAllTrades {
			msg = []byte(fmt.Sprintf("[%s]: %s", client.Conn.RemoteAddr().String(), string(msg)))
		}

		if client.Role == Producer || client.Role == JackOfAllTrades {
			select {
			case server.Broadcast <- msg:
			case <-server.done:
				return
			}
		}
	}
}

func writeMessages(client *Client) {
	defer func() {
		err := client.Conn.Close()
		if err != nil {
			log.Printf("failed to close the connection: %v", err)
		}
	}()
	for msg := range client.Send {
		if err := client.Conn.WriteMessage(websocket.TextMessage, msg); err != nil {
			break
		}
	}
}

// newEngine applies the configuration and returns the wss routes.
func newEngine(cfg ut.EnvConfig) *gin.Engine {
	address = cfg.WssAddress
	jobLogPath = cfg.WssLogsPath
	serviceSecret = cfg.ServiceSecretKey

	engine := gin.Default()
	engine.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "alive"})
	})
	engine.GET("/readyz", func(c *gin.Context) { // no dependencies: ready when alive
		c.JSON(http.StatusOK, gin.H{"status": "ready"})
	})
	engine.GET("/system-conf", func(c *gin.Context) {
		if !hasServiceSecret(c) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "service secret required"})

			return
		}
		wss, err := ut.ReadConfig("configs/"+cfg.ConfigPath, false)
		if err != nil {
			log.Printf("[API_sysConf] failed to read wss config: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})

			return
		}
		c.JSON(http.StatusOK, wss)
	})
	engine.GET("/get-session", HandleWSsession)
	engine.DELETE("/delete-session", HandleWSsessionClose)

	return engine
}

// Serve launches the main service listener
func Serve(cfg ut.EnvConfig) {
	engine := newEngine(cfg)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	server := ut.ServerTimeouts(&http.Server{
		Addr:    address,
		Handler: engine,
	})

	go func() {
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("listen: %s\n", err)
		}
	}()
	<-ctx.Done()

	stop()
	log.Println("shutting down gracefully, press Ctrl+C again to force")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		log.Fatal("Server forced to shutdown: ", err)
	}

	log.Println("Server exiting")
}
