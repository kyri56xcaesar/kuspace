// Package ws defines a simple websocket server logic
package ws

import (
	"fmt"
	"log"
	"net/http"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

// WebSocket Upgrader
var upgrader = websocket.Upgrader{
	CheckOrigin: func(_ *http.Request) bool { return true },
}

// Client Connection Structure
type Client struct {
	Conn *websocket.Conn
	Send chan []byte
}

// WebSocketServer struct defines a set of channels and a map of its clients that register on the channels
type WebSocketServer struct {
	Clients    map[*Client]bool
	Broadcast  chan []byte
	Register   chan *Client
	Unregister chan *Client
	sync.Mutex
}

// Server main variable
var Server = WebSocketServer{
	Clients:    make(map[*Client]bool),
	Broadcast:  make(chan []byte),
	Register:   make(chan *Client),
	Unregister: make(chan *Client),
}

// Start listening
func (s *WebSocketServer) Start() {
	for {
		select {
		case client := <-s.Register:
			s.Lock()
			s.Clients[client] = true

			clientAddr := client.Conn.RemoteAddr().String()
			connectMsg := fmt.Sprintf("Client %s connected.", clientAddr)
			fmt.Println(connectMsg) // Log the connection

			for client := range s.Clients {
				select {
				case client.Send <- []byte(connectMsg):
				default:
					close(client.Send)
					delete(s.Clients, client)
				}
			}

			s.Unlock()
			fmt.Println("Client connected")

		case client := <-s.Unregister:
			s.Lock()

			clientAddr := client.Conn.RemoteAddr().String()
			disconnectMsg := fmt.Sprintf("Client %s disconnected.", clientAddr)
			fmt.Println(disconnectMsg) // Log the connection

			if _, ok := s.Clients[client]; ok {
				delete(s.Clients, client)
				close(client.Send)
				for client := range s.Clients {
					select {
					case client.Send <- []byte(disconnectMsg):
					default:
						close(client.Send)
						delete(s.Clients, client)
					}
				}
			}
			s.Unlock()
			fmt.Println("Client disconnected")

		case message := <-s.Broadcast:
			s.Lock()
			for client := range s.Clients {
				select {
				case client.Send <- message:
				default:
					close(client.Send)
					delete(s.Clients, client)
				}
			}
			s.Unlock()
		}
	}
}

func handleGShellWs(c *gin.Context) {
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		fmt.Println("Failed to upgrade:", err)
		return
	}

	client := &Client{Conn: conn, Send: make(chan []byte)}
	Server.Register <- client

	go readMessages(client)
	go writeMessages(client)
}

func readMessages(client *Client) {
	defer func() {
		Server.Unregister <- client
		err := client.Conn.Close()
		if err != nil {
			log.Printf("failed to close the connection: %v", err)
		}
	}()

	for {
		_, message, err := client.Conn.ReadMessage()
		if err != nil {
			break
		}
		clientAddr := client.Conn.RemoteAddr().String()
		fullMessage := fmt.Sprintf("[%s]: %s", clientAddr, string(message))

		Server.Broadcast <- []byte(fullMessage)
	}
}

func writeMessages(client *Client) {
	defer func() {
		err := client.Conn.Close()
		if err != nil {
			log.Printf("failed to close the connection: %v", err)
		}
	}()
	for message := range client.Send {
		err := client.Conn.WriteMessage(websocket.TextMessage, message)
		if err != nil {
			break
		}
	}
}

// Serve http + websocket conns
func Serve() error {
	r := gin.Default()
	r.GET("/gshell", handleGShellWs)

	go Server.Start()

	log.Println("WebSocket Server started on :8081")
	log.Fatal(r.Run(":8081"))

	return nil
}
