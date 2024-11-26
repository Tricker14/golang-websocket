package main

import (
	"fmt"
	"net/http"

	"github.com/gorilla/websocket"
)

// upgrade HTTP connection to WS connection
var upgrader = websocket.Upgrader{
	ReadBufferSize: 1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true		
	},
}

type Message struct {
	Username string `json:"username"`
	Message  string `json:"message"`
}

type Client struct {
	conn *websocket.Conn
	channel chan Message	// channel that send msg to client
	username string
}

var clients = make(map[*Client] bool)	// connected clients
var broadcast = make(chan Message)	// this channel use to broadcast msg to all clients

const (
	// Maximum message size allowed from peer.
	maxMessageSize = 512
)

func homePage(w http.ResponseWriter, r *http.Request){
	http.ServeFile(w, r, "home.html")
}

func handleConnections(w http.ResponseWriter, r *http.Request){
	conn, err := upgrader.Upgrade(w, r, nil)	
	if err != nil {
		fmt.Println(err)
  		return
	}
	defer conn.Close()

	// set read limit for incomming msg
	conn.SetReadLimit(maxMessageSize)

	client := &Client {
		conn: conn,
		channel: make(chan Message, maxMessageSize),
	}

	var username string
	err = conn.ReadJSON(&username) // Read the username first
	if err != nil {
		fmt.Println("Error reading username:", err)
		return
	}
	client.username = username

	clients[client] = true

	// Each clients (connections) will have their own thread to write message
	go handleClientWrites(client)

	for {
		var msg Message
		err := conn.ReadJSON(&msg)	// read incomming msg
		if err != nil {
			fmt.Println(err)
			delete(clients, client)
			close(client.channel)
			return
		}

		broadcast<-msg	// add msg to broadcast channel
	}
}

func handleClientWrites(client *Client) {
	for msg := range client.channel {	// get message from a specific client
		err := client.conn.WriteJSON(msg)	// send msg to websocket of that client
		if err != nil {
			fmt.Println(err)
			client.conn.Close()
			delete(clients, client)
		}
	}
}

func handleMessages(){
	for {
		msg := <-broadcast	// get msg from broadcast channel

		for client := range clients {
			select {
			case client.channel <- msg:
				// success
			default: 
				fmt.Printf("Dropping message for client %s: buffer full\n", client.username)
			}
		}
	}
}

func main() {
	// serve static file
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))

    http.HandleFunc("/", homePage)
	http.HandleFunc("/ws", handleConnections)

	go handleMessages()

	fmt.Println("Server started on :8080")
	err := http.ListenAndServe(":8080", nil)
	if err != nil {
		panic("Error starting server: " + err.Error())
	}
}
