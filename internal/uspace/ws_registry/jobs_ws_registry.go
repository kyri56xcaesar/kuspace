package ws_registry

import (
	"log"
	"net/http"
	"os"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var (
	Job_log_path = "data/logs/jobs/"

	// Registry maps jobIDs to their socket servers
	registry = struct {
		sync.Mutex
		servers map[string]*JobSocketServer
	}{
		servers: make(map[string]*JobSocketServer),
	}

	upgrader = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool { return true },
	}
)

type Role string

const (
	Producer Role = "Producer"
	Consumer Role = "Consumer"
)

// client represents a WebSocket connection
type Client struct {
	Jid  string
	Conn *websocket.Conn
	Role Role
	Send chan []byte
}

// JobSocketServer manages clients for a specific job
type JobSocketServer struct {
	Producers  map[*Client]bool
	Consumers  map[*Client]bool
	Broadcast  chan []byte
	Register   chan *Client
	Unregister chan *Client
	sync.Mutex

	Jid    string
	Logger *log.Logger
}

func NewJobSocketServer(jid string) *JobSocketServer {
	// Create a new logger for the job socket server
	log_file, err := os.OpenFile(Job_log_path+"job-"+jid+".log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		log.Fatalf("failed to open log file: %v", err)
	}
	logger := log.New(log_file, "[JOB-"+jid+" WS-server] ", log.LstdFlags)

	return &JobSocketServer{
		Jid:        jid,
		Logger:     logger,
		Producers:  make(map[*Client]bool),
		Consumers:  make(map[*Client]bool),
		Broadcast:  make(chan []byte),
		Register:   make(chan *Client),
		Unregister: make(chan *Client),
	}
}

func (s *JobSocketServer) Start() {
	for {
		select {
		case client := <-s.Register:
			s.Lock()
			if client.Role == Producer {
				s.Producers[client] = true
			} else {
				s.Consumers[client] = true
			}
			s.Unlock()

		case client := <-s.Unregister:
			s.Lock()
			if client.Role == Producer {
				delete(s.Producers, client)
			} else {
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

func getOrCreateServer(jobID string) *JobSocketServer {
	registry.Lock()
	defer registry.Unlock()
	server, exists := registry.servers[jobID]
	if !exists {
		server = NewJobSocketServer(jobID)
		registry.servers[jobID] = server
		go server.Start()
	}
	return server
}

func HandleJobWS(c *gin.Context) {
	jobID := c.Query("jid")
	roleStr := c.Query("role")
	if jobID == "" || roleStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing job_id or role"})
		return
	}
	role := Role(roleStr)
	if role != Producer && role != Consumer {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid role"})
		return
	}
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	client := &Client{
		Jid:  jobID,
		Conn: conn,
		Role: role,
		Send: make(chan []byte, 256),
	}
	server := getOrCreateServer(jobID)
	server.Register <- client

	server.Logger.Printf("client registered: %v\n", client.Role)

	go writeMessages(client)
	go readMessages(client, server)

}

func HandleJobWSClose(c *gin.Context) {
	jobID := c.Query("jid")
	if jobID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing job_id"})
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
	c.JSON(http.StatusOK, gin.H{"status": "successfully deleted job socket server"})
}

func readMessages(client *Client, server *JobSocketServer) {
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
		if client.Role == Producer {
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
