package ws_registry

import (
	"log"
	"net/http"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var Job_log_path = "data/logs/jobs/job.log"

type Role string

const (
	Producer Role = "Producer"
	Consumer Role = "Consumer"
)

// client represents a WebSocket connection
type Client struct {
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
}

func NewJobSocketServer() *JobSocketServer {
	return &JobSocketServer{
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
					log.Printf("message incoming: %s\n", msg)
				default:
					close(Consumer.Send)
					delete(s.Consumers, Consumer)
				}
			}
			s.Unlock()
		}
	}
}

// Registry maps jobIDs to their socket servers
var registry = struct {
	sync.Mutex
	servers map[string]*JobSocketServer
}{
	servers: make(map[string]*JobSocketServer),
}

func getOrCreateServer(jobID string) *JobSocketServer {
	registry.Lock()
	defer registry.Unlock()
	server, exists := registry.servers[jobID]
	if !exists {
		server = NewJobSocketServer()
		registry.servers[jobID] = server
		go server.Start()
	}
	return server
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
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
		Conn: conn,
		Role: role,
		Send: make(chan []byte, 256),
	}
	server := getOrCreateServer(jobID)
	server.Register <- client

	log.Printf("client registered: %v\n", client.Role)

	go writeMessages(client)
	go readMessages(client, server)

}

func readMessages(client *Client, server *JobSocketServer) {
	defer func() {
		server.Unregister <- client
		client.Conn.Close()
	}()

	for {
		_, msg, err := client.Conn.ReadMessage()
		log.Printf("message read: %s", msg)
		if err != nil {
			break
		}
		if client.Role == Producer {
			log.Printf("broadcasting message by the producer")
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
		log.Printf("message sent: %s\n", msg)
	}
}
