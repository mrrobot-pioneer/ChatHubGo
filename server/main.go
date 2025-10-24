package main

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/gorilla/websocket"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

// Database connection
var db *sql.DB

// Load environment variables from .env
func loadEnv() {
	err := godotenv.Load()
	if err != nil {
		fmt.Println("Warning: .env file not found, reading from system environment")
	}
}

// User represents a chat user
type User struct {
	ID       int    `json:"id"`
	Username string `json:"username"`
	Email    string `json:"email"`
}

// Message represents a chat message
type Message struct {
	ID        int       `json:"id"`
	SenderID  int       `json:"sender_id"`
	Sender    string    `json:"sender"`
	Content   string    `json:"content"`
	Timestamp time.Time `json:"timestamp"`
}

// WebSocket message types
type WSMessage struct {
	Type     string    `json:"type"` // "message", "user_joined", "user_left", "typing"
	Sender   string    `json:"sender"`
	SenderID int       `json:"sender_id"`
	Content  string    `json:"content"`
	Time     time.Time `json:"timestamp"`
}

// Client represents a connected WebSocket client
type Client struct {
	ID   int
	Name string
	Conn *websocket.Conn
	Send chan *WSMessage
}

// Hub manages all connected clients
type Hub struct {
	clients    map[*Client]bool
	broadcast  chan *WSMessage
	register   chan *Client
	unregister chan *Client
	mu         sync.RWMutex
}

var hub *Hub

func init() {
	hub = &Hub{
		clients:    make(map[*Client]bool),
		broadcast:  make(chan *WSMessage, 256),
		register:   make(chan *Client),
		unregister: make(chan *Client),
	}
}

// Hub run loop
func (h *Hub) run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = true
			h.mu.Unlock()

			msg := &WSMessage{
				Type:     "user_joined",
				Sender:   client.Name,
				SenderID: client.ID,
				Time:     time.Now(),
			}
			h.broadcast <- msg
			log.Printf("User %s joined. Total clients: %d\n", client.Name, len(h.clients))

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.Send)

				msg := &WSMessage{
					Type:     "user_left",
					Sender:   client.Name,
					SenderID: client.ID,
					Time:     time.Now(),
				}
				h.broadcast <- msg
				log.Printf("User %s left. Total clients: %d\n", client.Name, len(h.clients))
			}
			h.mu.Unlock()

		case message := <-h.broadcast:
			h.mu.RLock()
			for client := range h.clients {
				select {
				case client.Send <- message:
				default:
					go func(c *Client) { h.unregister <- c }(client)
				}
			}
			h.mu.RUnlock()
		}
	}
}

// Get online users
func (h *Hub) GetOnlineUsers() []User {
	h.mu.RLock()
	defer h.mu.RUnlock()

	users := []User{}
	for client := range h.clients {
		users = append(users, User{ID: client.ID, Username: client.Name})
	}
	return users
}

// Initialize DB connection
func initDB() {
	loadEnv() // load .env first

	connStr := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		getEnv("DB_HOST", "localhost"),
		getEnv("DB_PORT", "5432"),
		getEnv("DB_USER", "postgres"),
		getEnv("DB_PASSWORD", "password"),
		getEnv("DB_NAME", "chathubdb"),
	)

	var err error
	db, err = sql.Open("postgres", connStr)
	if err != nil {
		log.Fatal("Failed to connect to database:", err)
	}

	if err := db.Ping(); err != nil {
		log.Fatal("Database ping failed:", err)
	}

	createTables()
	log.Println("✅ Database connected successfully")
}

// Create tables if not exist
func createTables() {
	schema := `
	CREATE TABLE IF NOT EXISTS users (
		id SERIAL PRIMARY KEY,
		username VARCHAR(255) UNIQUE NOT NULL,
		email VARCHAR(255) UNIQUE NOT NULL,
		password_hash VARCHAR(255) NOT NULL,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);
	CREATE TABLE IF NOT EXISTS messages (
		id SERIAL PRIMARY KEY,
		sender_id INT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
		content TEXT NOT NULL,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);
	CREATE INDEX IF NOT EXISTS idx_messages_created_at ON messages(created_at);
	`

	if _, err := db.Exec(schema); err != nil {
		log.Fatal("Failed to create tables:", err)
	}
}

// Helper: hash password
func hashPassword(password string) string {
	hash := sha256.Sum256([]byte(password))
	return hex.EncodeToString(hash[:])
}

// Helper: verify password
func verifyPassword(password, hash string) bool {
	return hashPassword(password) == hash
}

// Generate JWT
func generateJWT(userID int, username string) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id":  userID,
		"username": username,
		"exp":      time.Now().Add(24 * time.Hour).Unix(),
	})
	return token.SignedString([]byte(getEnv("JWT_SECRET", "your-secret-key")))
}

// Parse JWT
func parseJWT(tokenString string) (map[string]interface{}, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		return []byte(getEnv("JWT_SECRET", "your-secret-key")), nil
	})
	if err != nil || !token.Valid {
		return nil, err
	}

	if claims, ok := token.Claims.(jwt.MapClaims); ok {
		return claims, nil
	}
	return nil, fmt.Errorf("invalid token claims")
}

// --- Handlers --- //

func handleRegister(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Username string `json:"username"`
		Email    string `json:"email"`
		Password string `json:"password"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	if req.Username == "" || req.Email == "" || req.Password == "" {
		http.Error(w, "Missing fields", http.StatusBadRequest)
		return
	}

	hashed := hashPassword(req.Password)
	var userID int
	err := db.QueryRow(
		"INSERT INTO users (username, email, password_hash) VALUES ($1, $2, $3) RETURNING id",
		req.Username, req.Email, hashed,
	).Scan(&userID)

	if err != nil {
		http.Error(w, "User already exists", http.StatusConflict)
		return
	}

	token, _ := generateJWT(userID, req.Username)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"token":    token,
		"user_id":  userID,
		"username": req.Username,
	})
}

func handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	var userID int
	var hash string
	err := db.QueryRow(
		"SELECT id, password_hash FROM users WHERE username = $1",
		req.Username,
	).Scan(&userID, &hash)

	if err != nil || !verifyPassword(req.Password, hash) {
		http.Error(w, "Invalid credentials", http.StatusUnauthorized)
		return
	}

	token, _ := generateJWT(userID, req.Username)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"token":    token,
		"user_id":  userID,
		"username": req.Username,
	})
}

// Fetch last 50 messages
func handleMessages(w http.ResponseWriter, r *http.Request) {
	rows, err := db.Query(
		`SELECT m.id, m.sender_id, u.username, m.content, m.created_at
		 FROM messages m
		 JOIN users u ON m.sender_id = u.id
		 ORDER BY m.created_at DESC
		 LIMIT 50`,
	)
	if err != nil {
		http.Error(w, "Failed to fetch messages", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var messages []Message
	for rows.Next() {
		var m Message
		if err := rows.Scan(&m.ID, &m.SenderID, &m.Sender, &m.Content, &m.Timestamp); err != nil {
			continue
		}
		messages = append(messages, m)
	}

	// Reverse to chronological order
	for i, j := 0, len(messages)-1; i < j; i, j = i+1, j-1 {
		messages[i], messages[j] = messages[j], messages[i]
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(messages)
}

// Online users
func handleOnlineUsers(w http.ResponseWriter, r *http.Request) {
	users := hub.GetOnlineUsers()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(users)
}

// WebSocket handler
func handleWebSocket(w http.ResponseWriter, r *http.Request) {
	auth := r.Header.Get("Authorization")
	if auth == "" {
		http.Error(w, "Missing authorization", http.StatusUnauthorized)
		return
	}

	claims, err := parseJWT(auth)
	if err != nil {
		http.Error(w, "Invalid token", http.StatusUnauthorized)
		return
	}

	upgrader := websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("WebSocket upgrade error:", err)
		return
	}

	client := &Client{
		ID:   int(claims["user_id"].(float64)),
		Name: claims["username"].(string),
		Conn: conn,
		Send: make(chan *WSMessage, 256),
	}

	hub.register <- client
	go client.readPump()
	go client.writePump()
}

// Read/Write pumps
func (c *Client) readPump() {
	defer func() { hub.unregister <- c; c.Conn.Close() }()
	for {
		var msg WSMessage
		if err := c.Conn.ReadJSON(&msg); err != nil {
			break
		}
		msg.Type = "message"
		msg.Sender = c.Name
		msg.SenderID = c.ID
		msg.Time = time.Now()

		// Save
		_, err := db.Exec("INSERT INTO messages (sender_id, content) VALUES ($1,$2)", c.ID, msg.Content)
		if err != nil {
			log.Println("Failed to save message:", err)
		}
		hub.broadcast <- &msg
	}
}

func (c *Client) writePump() {
	defer c.Conn.Close()
	for msg := range c.Send {
		if err := c.Conn.WriteJSON(msg); err != nil {
			break
		}
	}
}

// Helpers
func getEnv(key, def string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return def
}

func enableCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET,POST,PUT,DELETE,OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type,Authorization")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func main() {
	initDB()
	defer db.Close()

	go hub.run()

	mux := http.NewServeMux()
	mux.HandleFunc("/api/register", handleRegister)
	mux.HandleFunc("/api/login", handleLogin)
	mux.HandleFunc("/api/messages", handleMessages)
	mux.HandleFunc("/api/users/online", handleOnlineUsers)
	mux.HandleFunc("/ws", handleWebSocket)

	http.Handle("/", enableCORS(mux))

	port := getEnv("PORT", "8080")
	log.Printf("Server running on :%s\n", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}