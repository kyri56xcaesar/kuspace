package ws_registry

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"

	ut "kyri56xcaesar/kuspace/internal/utils"
)

var (
	Address      = "0.0.0.0:8082"
	Job_log_path = "data/logs/jobs/"

	// Registry maps jobIDs to their socket servers
	registry = struct {
		sync.Mutex
		servers map[string]*SocketServer
	}{
		servers: make(map[string]*SocketServer),
	}

	upgrader = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool { return true },
	}
)

type Role string

const (
	Producer        Role = "producer"
	Consumer        Role = "consumer"
	JackOfAllTrades Role = "jack"
)

// client represents a WebSocket connection
type Client struct {
	Jid  string
	Conn *websocket.Conn
	Role Role
	Send chan []byte
}

// SocketServer manages clients for a specific job
type SocketServer struct {
	Producers  map[*Client]bool
	Consumers  map[*Client]bool
	Broadcast  chan []byte
	Register   chan *Client
	Unregister chan *Client
	sync.Mutex

	Jid    string
	Logger *log.Logger
}

func NewSocketServer(jid string) *SocketServer {
	// Create a new logger for the job socket server
	err := os.MkdirAll(Job_log_path, 0o644)
	if err != nil {
		log.Fatalf("failed to create path to logs: %v", err)
	}

	log_file, err := os.OpenFile(Job_log_path+"ws-server-"+jid+".log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		log.Fatalf("failed to open log file: %v", err)
	}
	logger := log.New(log_file, "[WS-"+jid+" WS-server] ", log.LstdFlags)

	return &SocketServer{
		Jid:        jid,
		Logger:     logger,
		Producers:  make(map[*Client]bool),
		Consumers:  make(map[*Client]bool),
		Broadcast:  make(chan []byte),
		Register:   make(chan *Client),
		Unregister: make(chan *Client),
	}
}

func getOrCreateServer(jobID string) *SocketServer {
	registry.Lock()
	defer registry.Unlock()
	server, exists := registry.servers[jobID]
	if !exists {
		server = NewSocketServer(jobID)
		registry.servers[jobID] = server
		go server.Start()
	}
	return server
}

func (s *SocketServer) Start() {
	for {
		select {
		case client := <-s.Register:
			s.Lock()
			if client.Role == Producer {
				s.Producers[client] = true
			} else if client.Role == Consumer {
				s.Consumers[client] = true
			} else {
				s.Producers[client] = true
				s.Consumers[client] = true
			}
			s.Unlock()
		case client := <-s.Unregister:
			s.Lock()
			if client.Role == Producer {
				delete(s.Producers, client)
			} else if client.Role == Consumer {
				delete(s.Consumers, client)
			} else {
				delete(s.Producers, client)
				delete(s.Consumers, client)
			}
			close(client.Send)
			s.Unlock()
		case msg := <-s.Broadcast:
			s.Lock()
			for Consumer := range s.Consumers {
				select {
				case Consumer.Send <- msg:
					// log.Printf("message incoming: %s\n", msg)
				default:
					close(Consumer.Send)
					delete(s.Consumers, Consumer)
				}
			}
			s.Unlock()
		}
	}
}

func HandleWSsession(c *gin.Context) {
	id := c.Query("jid")
	roleStr := strings.ToLower(c.Query("role"))
	if id == "" || roleStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing jid or role"})
		return
	}
	role := Role(roleStr)
	if role != Producer && role != Consumer && role != JackOfAllTrades {
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
	server := getOrCreateServer(id)
	server.Register <- client

	server.Logger.Printf("client registered: %v\n", client.Role)

	go writeMessages(client)
	go broadcastMessages(client, server)

}

func HandleWSsessionClose(c *gin.Context) {
	jobID := c.Query("jid")
	if jobID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing jid"})
		return
	}
	registry.Lock()
	defer registry.Unlock()
	if server, exists := registry.servers[jobID]; exists {
		for client := range server.Producers {
			client.Conn.Close()
			close(client.Send)
			delete(server.Producers, client)
		}
		for client := range server.Consumers {
			client.Conn.Close()
			close(client.Send)
			delete(server.Consumers, client)
		}
		delete(registry.servers, jobID)
	}
	c.JSON(http.StatusOK, gin.H{"status": "successfully deleted socket server"})
}

func broadcastMessages(client *Client, server *SocketServer) {
	defer func() {
		server.Unregister <- client
		client.Conn.Close()
	}()

	for {
		_, msg, err := client.Conn.ReadMessage()
		server.Logger.Printf("message read: %s", msg)
		if err != nil {
			break
		}
		if client.Role == JackOfAllTrades {
			msg = []byte(fmt.Sprintf("[%s]: %s", client.Conn.RemoteAddr().String(), string(msg)))
		}

		if client.Role == Producer || client.Role == JackOfAllTrades {
			server.Logger.Printf("producer broadcasting: %s", msg)
			server.Broadcast <- msg
		}
	}
}

func writeMessages(client *Client) {
	defer client.Conn.Close()
	for msg := range client.Send {
		if err := client.Conn.WriteMessage(websocket.TextMessage, msg); err != nil {
			break
		}
	}
}

func Serve(cfg ut.EnvConfig) {
	Job_log_path = cfg.J_WS_LOGS_PATH
	Address = cfg.J_WS_ADDRESS

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	engine := gin.Default()
	engine.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "alive"})
	})
	engine.GET("/system-conf", func(c *gin.Context) {
		wss, err := ut.ReadConfig("configs/"+cfg.ConfigPath, false)
		if err != nil {
			log.Printf("[API_sysConf] failed to read frontapp config: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, wss)
	})
	engine.GET("/get-session", HandleWSsession)
	engine.DELETE("/delete-session", HandleWSsessionClose)

	server := &http.Server{
		Addr:              Address,
		Handler:           engine,
		ReadHeaderTimeout: time.Second * 5,
	}

	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
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
