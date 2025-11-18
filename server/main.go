package main

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"github.com/joho/godotenv"
	"github.com/lib/pq"
)

var db *sql.DB
var roomManager *RoomManager
var jwtKey []byte

const SystemMessageTimeFormat = "3:04 PM on Jan 2, 2006"

// --- Struct Definitions ---

type User struct {
	ID       int    `json:"id"`
	Username string `json:"username"`
	Email    string `json:"email,omitempty"`
}

// Room represents a chat room
type Room struct {
	ID          int       `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	CreatedBy   int       `json:"created_by"`
	CreatedAt   time.Time `json:"created_at"`
	LastMessage     string `json:"lastMessage"`
	LastSenderID    int    `json:"lastSenderId"`
	LastMessageTime string `json:"lastMessageTime"`
	Unread          int    `json:"unread"`
	IsPrivate       bool   `json:"isPrivate"`
	Members         int    `json:"members"`
	Avatar          string `json:"avatar"`
}

// Message represents a chat message
type Message struct {
	ID        int       `json:"id"`
	RoomID    int       `json:"room_id"`
	SenderID  int       `json:"sender_id"`
	Sender    string    `json:"sender"`  
	Avatar    string    `json:"avatar"`
	Text      string    `json:"text"`
	Timestamp time.Time `json:"timestamp"`
	Read      bool      `json:"read"`
}

// WSMessage is the envelope for WebSocket communication
type WSMessage struct {
	Type     string `json:"type"` // "joinRoom", "sendMessage", "roomMessage", "error"
	RoomID   int    `json:"room_id,omitempty"`
	Content  string `json:"content,omitempty"` // For "sendMessage"
	*Message        // For "roomMessage"
}

// Client represents a connected WebSocket client
type Client struct {
	ID       int
	Username string
	Avatar   string
	Conn     *websocket.Conn
	Send     chan *WSMessage
	Manager  *RoomManager
}

// RoomHub manages clients for a single room
type RoomHub struct {
	RoomID     int
	Clients    map[*Client]bool
	Broadcast  chan *WSMessage
	Register   chan *Client
	Unregister chan *Client
	Manager    *RoomManager
	mu         sync.RWMutex
}

// RoomManager manages all RoomHubs
type RoomManager struct {
	Rooms      map[int]*RoomHub
	Register   chan *Client
	Unregister chan *Client
	mu         sync.RWMutex
}

// --- Environment & DB Init ---

func loadEnv() {
	if err := godotenv.Load(); err != nil {
		fmt.Println("Warning: .env file not found, reading from system environment")
	}
}

func getEnv(key, def string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return def
}

func initDB() {
	loadEnv()
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
	if err = db.Ping(); err != nil {
		log.Fatal("Database ping failed:", err)
	}

	createTables()
	log.Println("✅ Database connected successfully")
}

func createTables() {
	
	schema := `
    CREATE TABLE IF NOT EXISTS users (
        id SERIAL PRIMARY KEY,
        username VARCHAR(255) UNIQUE NOT NULL,
        email VARCHAR(255) UNIQUE NOT NULL,
        password_hash VARCHAR(255) NOT NULL,
        created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
    );
    CREATE TABLE IF NOT EXISTS rooms (
        id SERIAL PRIMARY KEY,
        name VARCHAR(255) NOT NULL,
        description TEXT,
        created_by INT NOT NULL REFERENCES users(id) ON DELETE SET NULL,
        created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		is_private BOOLEAN NOT NULL DEFAULT FALSE
    );
    CREATE TABLE IF NOT EXISTS room_members (
        id SERIAL PRIMARY KEY,
        room_id INT NOT NULL REFERENCES rooms(id) ON DELETE CASCADE,
        user_id INT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
        role VARCHAR(50) DEFAULT 'member', -- 'admin', 'member'
        joined_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
        UNIQUE(room_id, user_id)
    );
    CREATE TABLE IF NOT EXISTS messages (
        id SERIAL PRIMARY KEY,
        room_id INT NOT NULL REFERENCES rooms(id) ON DELETE CASCADE,
        sender_id INT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
        content TEXT NOT NULL,
        created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
    );
    CREATE INDEX IF NOT EXISTS idx_messages_room_id_created_at ON messages(room_id, created_at);

    CREATE TABLE IF NOT EXISTS message_reads (
        id SERIAL PRIMARY KEY,
        message_id INT NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
        user_id INT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
        read_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
        UNIQUE(message_id, user_id)
    );
    CREATE INDEX IF NOT EXISTS idx_message_reads_user_message ON message_reads(user_id, message_id);
    CREATE INDEX IF NOT EXISTS idx_message_reads_message ON message_reads(message_id);
    `

	if _, err := db.Exec(schema); err != nil {
		log.Fatal("Failed to create tables:", err)
	}

	// --- Create System User ---
	// Only create system user and adjust sequence if it doesn't exist
	var systemUserExists bool
	err := db.QueryRow("SELECT EXISTS(SELECT 1 FROM users WHERE id = 1)").Scan(&systemUserExists)
	if err != nil {
		log.Fatal("Failed to check for System user:", err)
	}

	if !systemUserExists {
		// Create system user with explicit ID
		systemUserSQL := `
		INSERT INTO users (id, username, email, password_hash)
		VALUES (1, 'System', 'system@chathub.io', 'SYSTEM_ACCOUNT_HASH');
		`
		if _, err := db.Exec(systemUserSQL); err != nil {
			log.Fatal("Failed to create System user:", err)
		}

		// Adjust sequence to start at 2 (only on first creation)
		if _, err := db.Exec("SELECT setval('users_id_seq', 1, true)"); err != nil {
			log.Fatal("Failed to update users sequence value:", err)
		}

		log.Println("✅ System user (ID 1) created and sequence initialized.")
	} else {
		log.Println("✅ System user (ID 1) already exists.")
	}
}

// --- Hub & Manager Logic ---

func NewRoomManager() *RoomManager {
	return &RoomManager{
		Rooms:      make(map[int]*RoomHub),
		Register:   make(chan *Client),
		Unregister: make(chan *Client),
	}
}

// Global manager for registering/unregistering clients
func (m *RoomManager) Run() {
	for {
		select {
		case client := <-m.Register:
			log.Printf("Client %s connected", client.Username)
		case client := <-m.Unregister:
			log.Printf("Client %s disconnected", client.Username)
			m.mu.Lock()
			for _, hub := range m.Rooms {
				hub.mu.Lock()
				if _, ok := hub.Clients[client]; ok {
					delete(hub.Clients, client)
					close(client.Send)
				}
				hub.mu.Unlock()
			}
			m.mu.Unlock()
		}
	}
}

func (m *RoomManager) GetOrCreateRoomHub(roomID int) *RoomHub {
	m.mu.Lock()
	defer m.mu.Unlock()

	if hub, ok := m.Rooms[roomID]; ok {
		return hub
	}

	hub := &RoomHub{
		RoomID:     roomID,
		Clients:    make(map[*Client]bool),
		Broadcast:  make(chan *WSMessage, 256),
		Register:   make(chan *Client),
		Unregister: make(chan *Client),
		Manager:    m,
	}
	m.Rooms[roomID] = hub

	go hub.Run()
	return hub
}

func (h *RoomHub) Run() {
	log.Printf("Starting hub for room %d", h.RoomID)
	for {
		select {
		case client := <-h.Register:
			h.mu.Lock()
			h.Clients[client] = true
			log.Printf("Client %s joined room %d. Total clients in room: %d", client.Username, h.RoomID, len(h.Clients))
			h.mu.Unlock()

		case client := <-h.Unregister:
			h.mu.Lock()
			if _, ok := h.Clients[client]; ok {
				delete(h.Clients, client)
				log.Printf("Client %s left room %d. Total clients in room: %d", client.Username, h.RoomID, len(h.Clients))
			}
			h.mu.Unlock()

		case message := <-h.Broadcast:
			h.mu.RLock()
			for client := range h.Clients {
				select {
				case client.Send <- message:
				default:
					go func(c *Client) { h.Manager.Unregister <- c }(client)
				}
			}
			h.mu.RUnlock()
		}
	}
}

// --- WebSocket Client Logic ---

func (c *Client) readPump() {
	defer func() { c.Manager.Unregister <- c; c.Conn.Close() }()

	for {
		var msg WSMessage
		if err := c.Conn.ReadJSON(&msg); err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("error: %v", err)
			}
			break
		}

		msg.Message = &Message{
			SenderID: c.ID,
			Sender:   c.Username,
			Avatar:   c.Avatar,
		}

		switch msg.Type {
		case "joinRoom":
			if !isUserInRoom(c.ID, msg.RoomID) {
				log.Printf("Auth error: User %d tried to join room %d", c.ID, msg.RoomID)
				c.Send <- &WSMessage{Type: "error", Content: "Not authorized for this room"}
				continue
			}
			hub := c.Manager.GetOrCreateRoomHub(msg.RoomID)
			hub.Register <- c
			log.Printf("Client %s joined room %d", c.Username, msg.RoomID)

		case "sendMessage":
			if msg.Content == "" || msg.RoomID == 0 {
				log.Println("Invalid message from client")
				continue
			}
			
			if !isUserInRoom(c.ID, msg.RoomID) {
				log.Printf("Auth error: User %d tried to send to room %d", c.ID, msg.RoomID)
				c.Send <- &WSMessage{Type: "error", Content: "Not authorized to send to this room"}
				continue
			}

			var savedMsg Message
			err := db.QueryRow(
				"INSERT INTO messages (room_id, sender_id, content) VALUES ($1, $2, $3) RETURNING id, room_id, sender_id, content, created_at",
				msg.RoomID, c.ID, msg.Content,
			).Scan(&savedMsg.ID, &savedMsg.RoomID, &savedMsg.SenderID, &savedMsg.Text, &savedMsg.Timestamp)

			if err != nil {
				log.Println("Failed to save message:", err)
				continue
			}

			savedMsg.Sender = c.Username
			savedMsg.Avatar = c.Avatar
			savedMsg.Read = false 

			hub := c.Manager.GetOrCreateRoomHub(msg.RoomID)
			hub.Broadcast <- &WSMessage{
				Type:    "roomMessage",
				RoomID:   savedMsg.RoomID, 
				Message: &savedMsg,
			}
		}
	}
}

func (c *Client) writePump() {
	defer c.Conn.Close()
	for msg := range c.Send {
		if err := c.Conn.WriteJSON(msg); err != nil {
			log.Println("WebSocket write error:", err)
			break
		}
	}
}

// --- Auth & Helpers ---

func init() {
	loadEnv()
	jwtKey = []byte(getEnv("JWT_SECRET", "your-secret-key-super-secret"))
	roomManager = NewRoomManager()
}

func hashPassword(password string) string {
	hash := sha256.Sum256([]byte(password))
	return hex.EncodeToString(hash[:])
}

func verifyPassword(password, hash string) bool {
	return hashPassword(password) == hash
}

func generateJWT(userID int, username string) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id":  userID,
		"username": username,
		"exp":      time.Now().Add(24 * time.Hour).Unix(),
	})
	return token.SignedString(jwtKey)
}

func parseJWT(tokenString string) (jwt.MapClaims, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return jwtKey, nil
	})

	if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
		return claims, nil
	}
	return nil, err
}

// Auth middleware to get user claims
func authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			http.Error(w, "Missing authorization token", http.StatusUnauthorized)
			return
		}

		tokenString := authHeader
		if len(authHeader) > 7 && authHeader[:7] == "Bearer " {
			tokenString = authHeader[7:]
		}

		claims, err := parseJWT(tokenString)
		if err != nil {
			http.Error(w, "Invalid token", http.StatusUnauthorized)
			return
		}

		ctx := r.Context()
		ctx = context.WithValue(ctx, "user_id", claims["user_id"].(float64))
		ctx = context.WithValue(ctx, "username", claims["username"].(string))
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func isUserInRoom(userID, roomID int) bool {
	var exists bool
	err := db.QueryRow("SELECT EXISTS(SELECT 1 FROM room_members WHERE user_id = $1 AND room_id = $2)", userID, roomID).Scan(&exists)
	if err != nil {
		log.Printf("Error checking room membership: %v", err)
		return false
	}
	return exists
}

// --- HTTP Handlers ---

func handleRegister(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	// Validate input
	if req.Username == "" || req.Email == "" || req.Password == "" {
		http.Error(w, "Username, email, and password are required", http.StatusBadRequest)
		return
	}

	hashed := hashPassword(req.Password)
	var userID int
	err := db.QueryRow(
		"INSERT INTO users (username, email, password_hash) VALUES ($1, $2, $3) RETURNING id",
		req.Username, req.Email, hashed,
	).Scan(&userID)

	if err != nil {
		// Check if it's a PostgreSQL unique constraint violation
		if pqErr, ok := err.(*pq.Error); ok && pqErr.Code == "23505" {
			// 23505 is the PostgreSQL error code for unique_violation
			if pqErr.Constraint == "users_username_key" {
				http.Error(w, "Username already taken", http.StatusConflict)
				return
			} else if pqErr.Constraint == "users_email_key" {
				http.Error(w, "Email already registered", http.StatusConflict)
				return
			}
			http.Error(w, "User already exists", http.StatusConflict)
			return
		}
		// Other database errors
		log.Printf("Registration error: %v", err)
		http.Error(w, "Registration failed. Please try again.", http.StatusInternalServerError)
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

// Create a new room
func handleCreateRoom(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}
	if req.Name == "" {
		http.Error(w, "Room name is required", http.StatusBadRequest)
		return
	}

	userID := int(r.Context().Value("user_id").(float64))

	tx, err := db.Begin()
	if err != nil {
		http.Error(w, "Server error", http.StatusInternalServerError)
		return
	}
	defer tx.Rollback() 

	var roomID int
	var createdAt time.Time
	err = tx.QueryRow(
		"INSERT INTO rooms (name, description, created_by) VALUES ($1, $2, $3) RETURNING id, created_at",
		req.Name, req.Description, userID,
	).Scan(&roomID, &createdAt)

	if err != nil {
		log.Println("Failed to create room:", err)
		http.Error(w, "Failed to create room", http.StatusInternalServerError)
		return
	}

	_, err = tx.Exec(
		"INSERT INTO room_members (room_id, user_id, role) VALUES ($1, $2, $3)",
		roomID, userID, "admin",
	)
	if err != nil {
		log.Println("Failed to add room member:", err)
		http.Error(w, "Failed to create room", http.StatusInternalServerError)
		return
	}

	currentTime := time.Now()
	formattedTime := currentTime.Format(SystemMessageTimeFormat)

	username := r.Context().Value("username").(string)

	systemMessageContent := fmt.Sprintf("%s created this room at %s.", username, formattedTime)

	var savedMsg Message
	err = tx.QueryRow(
		"INSERT INTO messages (room_id, sender_id, content) VALUES ($1, $2, $3) RETURNING id, room_id, sender_id, content, created_at",
		roomID, 1, systemMessageContent,
	).Scan(&savedMsg.ID, &savedMsg.RoomID, &savedMsg.SenderID, &savedMsg.Text, &savedMsg.Timestamp)

	if err != nil {
		log.Printf("Failed to add creation system message: %v", err)
		http.Error(w, "Room created, but system message failed", http.StatusInternalServerError)
		return
	}

	if err := tx.Commit(); err != nil {
		http.Error(w, "Server error during commit", http.StatusInternalServerError)
		return
	}

	hub := roomManager.GetOrCreateRoomHub(roomID)
	savedMsg.Sender = "System"
	savedMsg.Avatar = "S"
	savedMsg.Read = false

	hub.Broadcast <- &WSMessage{
		Type:    "roomMessage",
		RoomID:   savedMsg.RoomID, 
		Message: &savedMsg,
	}

	newRoom := Room{
		ID:          roomID,
		Name:        req.Name,
		Description: req.Description,
		CreatedBy:   userID,
		CreatedAt:   createdAt,
		LastMessage: fmt.Sprintf("You created this room at %s.", formattedTime),
		LastMessageTime: currentTime.Format("3:04 PM"),
		Unread: 0,
		IsPrivate: false,
		Members: 1,
		Avatar: string(req.Name[0]),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(newRoom)
}

// handleJoinRoom adds the current user to a room and creates a system message.
func handleJoinRoom(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	roomID, err := strconv.Atoi(vars["id"])
	if err != nil {
		http.Error(w, "Invalid room ID", http.StatusBadRequest)
		return
	}

	userID := int(r.Context().Value("user_id").(float64))
	username := r.Context().Value("username").(string)

	var room Room
	var membersCount int
	err = db.QueryRow(`
		SELECT r.id, r.name, r.description, r.created_by, r.created_at,
			(SELECT COUNT(*) FROM room_members rm WHERE rm.room_id = r.id) as members_count
		FROM rooms r WHERE r.id = $1
	`, roomID).Scan(&room.ID, &room.Name, &room.Description, &room.CreatedBy, &room.CreatedAt, &membersCount)

	if err == sql.ErrNoRows {
		http.Error(w, "Room not found", http.StatusNotFound)
		return
	} else if err != nil {
		log.Printf("DB error fetching room details: %v", err)
		http.Error(w, "Server error", http.StatusInternalServerError)
		return
	}

	if isUserInRoom(userID, roomID) {
		http.Error(w, "Already a member of this room", http.StatusConflict)
		return
	}

	tx, err := db.Begin()
	if err != nil {
		http.Error(w, "Server error", http.StatusInternalServerError)
		return
	}
	defer tx.Rollback()

	_, err = tx.Exec(
		"INSERT INTO room_members (room_id, user_id, role) VALUES ($1, $2, $3)",
		roomID, userID, "member",
	)
	if err != nil {
		log.Printf("Failed to add room member: %v", err)
		http.Error(w, "Failed to join room", http.StatusInternalServerError)
		return
	}

	currentTime := time.Now()
	formattedTime := currentTime.Format(SystemMessageTimeFormat)
	systemMessageContent := fmt.Sprintf("%s joined this room at %s.", username, formattedTime)

	var savedMsg Message
	err = tx.QueryRow(
		"INSERT INTO messages (room_id, sender_id, content) VALUES ($1, $2, $3) RETURNING id, room_id, sender_id, content, created_at",
		roomID, 1, systemMessageContent,
	).Scan(&savedMsg.ID, &savedMsg.RoomID, &savedMsg.SenderID, &savedMsg.Text, &savedMsg.Timestamp)
	if err != nil {
		log.Printf("Failed to add system message: %v", err)
		http.Error(w, "Failed to join room (message fail)", http.StatusInternalServerError)
		return
	}

	if err := tx.Commit(); err != nil {
		http.Error(w, "Server error during commit", http.StatusInternalServerError)
		return
	}

	hub := roomManager.GetOrCreateRoomHub(roomID)

	savedMsg.Sender = "System"
	savedMsg.Avatar = "S" 
	savedMsg.Read = false

	hub.Broadcast <- &WSMessage{
		Type:     "roomMessage",
		RoomID:   savedMsg.RoomID,
		Message:  &savedMsg,
	}
	log.Printf("Broadcasted system message to room %d: %s", roomID, savedMsg.Text)

	room.Members = membersCount + 1 
	room.Avatar = string(room.Name[0])
	room.LastMessage = fmt.Sprintf("You joined this room at %s.", formattedTime)
	room.LastMessageTime = currentTime.Format("3:04 PM")
	room.Unread = 0
	room.IsPrivate = false

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(room)
}

// Get all rooms for the current user
func handleGetRooms(w http.ResponseWriter, r *http.Request) {
    userID := int(r.Context().Value("user_id").(float64))
    rooms := []Room{}

    rows, err := db.Query(`
        SELECT
            r.id, r.name, r.description, r.created_by, r.created_at, r.is_private,
            (SELECT COUNT(*) FROM room_members WHERE room_id = r.id) AS members_count,
            lm.content,
            lm.created_at,
			lm.sender_id,
            -- Calculate unread count: messages not sent by user and not in message_reads
            (
                SELECT COUNT(*)
                FROM messages m
                WHERE m.room_id = r.id
                    AND m.sender_id != $1  -- Exclude own messages
                    AND NOT EXISTS (
                        SELECT 1 FROM message_reads mr
                        WHERE mr.message_id = m.id AND mr.user_id = $1
                    )
            ) AS unread_count
        FROM rooms r
        JOIN room_members rm ON rm.room_id = r.id
        LEFT JOIN (
            -- Subquery to find the single latest message (lm = Latest Message)
			SELECT DISTINCT ON (room_id) id, room_id, content, created_at, sender_id
            FROM messages
            ORDER BY room_id, created_at DESC
        ) lm ON lm.room_id = r.id
        WHERE rm.user_id = $1
        ORDER BY lm.created_at DESC NULLS LAST -- Order by latest activity
    `, userID)

    if err != nil {
        log.Println("Failed to get rooms:", err)
        http.Error(w, "Failed to get rooms", http.StatusInternalServerError)
        return
    }
    defer rows.Close()

    for rows.Next() {
        var room Room
        var membersCount int
        var unreadCount int
        
        var lastMessage sql.NullString
        var lastMessageTime sql.NullTime
		var lastSenderID sql.NullInt64
        
        if err := rows.Scan(
            &room.ID, &room.Name, &room.Description, &room.CreatedBy, &room.CreatedAt, &room.IsPrivate,
            &membersCount,
            &lastMessage,
            &lastMessageTime,
			&lastSenderID,
            &unreadCount, 
        ); err != nil {
            log.Println("Error scanning room:", err)
            continue
        }
        
        room.Members = membersCount
        room.Unread = unreadCount

        if lastMessage.Valid {
            room.LastMessage = lastMessage.String
        } else {
            room.LastMessage = "No messages yet." 
        }

        if lastMessageTime.Valid {
            room.LastMessageTime = lastMessageTime.Time.Format("3:04 PM")
        } else {
            room.LastMessageTime = room.CreatedAt.Format("3:04 PM") 
        }

		if lastSenderID.Valid {
    		room.LastSenderID = int(lastSenderID.Int64)
		} else {
    		room.LastSenderID = 0 // Default to 0 or another non-system ID if no messages
		}

        room.Avatar = string(room.Name[0])
        
        rooms = append(rooms, room)
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(rooms)
}

// Get messages for a specific room
func handleGetRoomMessages(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	roomID, err := strconv.Atoi(vars["id"])
	if err != nil {
		http.Error(w, "Invalid room ID", http.StatusBadRequest)
		return
	}

	userID := int(r.Context().Value("user_id").(float64))

	if !isUserInRoom(userID, roomID) {
		http.Error(w, "Not authorized", http.StatusForbidden)
		return
	}

	rows, err := db.Query(
		`SELECT m.id, m.room_id, m.sender_id, u.username, m.content, m.created_at
         FROM messages m
         JOIN users u ON m.sender_id = u.id
         WHERE m.room_id = $1
         ORDER BY m.created_at ASC
         LIMIT 100`,
		roomID,
	)
	if err != nil {
		http.Error(w, "Failed to fetch messages", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var messages []Message
	for rows.Next() {
		var m Message
		if err := rows.Scan(&m.ID, &m.RoomID, &m.SenderID, &m.Sender, &m.Text, &m.Timestamp); err != nil {
			log.Println("Error scanning message:", err)
			continue
		}
		m.Avatar = string(m.Sender[0])
		m.Read = true
		messages = append(messages, m)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(messages)
}

// Mark all messages in a room as read for the current user
func handleMarkRoomAsRead(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	roomID, err := strconv.Atoi(vars["id"])
	if err != nil {
		http.Error(w, "Invalid room ID", http.StatusBadRequest)
		return
	}

	userID := int(r.Context().Value("user_id").(float64))

	if !isUserInRoom(userID, roomID) {
		http.Error(w, "Not authorized", http.StatusForbidden)
		return
	}

	result, err := db.Exec(`
		INSERT INTO message_reads (message_id, user_id)
		SELECT m.id, $1
		FROM messages m
		WHERE m.room_id = $2
			AND m.sender_id != $1  -- Don't mark own messages
			AND NOT EXISTS (
				SELECT 1 FROM message_reads mr
				WHERE mr.message_id = m.id AND mr.user_id = $1
			)
		ON CONFLICT (message_id, user_id) DO NOTHING
	`, userID, roomID)

	if err != nil {
		log.Printf("Failed to mark messages as read: %v", err)
		http.Error(w, "Failed to mark messages as read", http.StatusInternalServerError)
		return
	}

	rowsAffected, _ := result.RowsAffected()

	if rowsAffected > 0 {
		hub := roomManager.GetOrCreateRoomHub(roomID)
		hub.Broadcast <- &WSMessage{
			Type:   "messagesRead",
			RoomID: roomID,
		}
		log.Printf("User %d marked %d messages as read in room %d", userID, rowsAffected, roomID)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "success"})
}

// Get all members of a specific room
func handleGetRoomMembers(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	roomID, err := strconv.Atoi(vars["id"])
	if err != nil {
		http.Error(w, "Invalid room ID", http.StatusBadRequest)
		return
	}

	userID := int(r.Context().Value("user_id").(float64))

	if !isUserInRoom(userID, roomID) {
		http.Error(w, "Not authorized", http.StatusForbidden)
		return
	}

	rows, err := db.Query(`
		SELECT u.id, u.username, u.email, rm.role, rm.joined_at
		FROM room_members rm
		JOIN users u ON rm.user_id = u.id
		WHERE rm.room_id = $1
		ORDER BY
			CASE rm.role
				WHEN 'admin' THEN 1
				ELSE 2
			END,
			rm.joined_at ASC
	`, roomID)

	if err != nil {
		log.Printf("Failed to get room members: %v", err)
		http.Error(w, "Failed to get room members", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	type RoomMember struct {
		ID       int       `json:"id"`
		Username string    `json:"username"`
		Email    string    `json:"email"`
		Avatar   string    `json:"avatar"`
		Role     string    `json:"role"`
		JoinedAt time.Time `json:"joined_at"`
	}

	var members []RoomMember
	for rows.Next() {
		var m RoomMember
		if err := rows.Scan(&m.ID, &m.Username, &m.Email, &m.Role, &m.JoinedAt); err != nil {
			log.Printf("Error scanning member: %v", err)
			continue
		}
		m.Avatar = string(m.Username[0])
		members = append(members, m)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(members)
}

// Remove a member from a room (admin only)
func handleRemoveMember(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	roomID, err := strconv.Atoi(vars["id"])
	if err != nil {
		http.Error(w, "Invalid room ID", http.StatusBadRequest)
		return
	}

	memberID, err := strconv.Atoi(vars["memberId"])
	if err != nil {
		http.Error(w, "Invalid member ID", http.StatusBadRequest)
		return
	}

	userID := int(r.Context().Value("user_id").(float64))

	var role string
	err = db.QueryRow("SELECT role FROM room_members WHERE room_id = $1 AND user_id = $2", roomID, userID).Scan(&role)
	if err != nil || role != "admin" {
		http.Error(w, "Only admins can remove members", http.StatusForbidden)
		return
	}

	if memberID == userID {
		http.Error(w, "Cannot remove yourself. Use leave room instead", http.StatusBadRequest)
		return
	}

	result, err := db.Exec("DELETE FROM room_members WHERE room_id = $1 AND user_id = $2", roomID, memberID)
	if err != nil {
		log.Printf("Failed to remove member: %v", err)
		http.Error(w, "Failed to remove member", http.StatusInternalServerError)
		return
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		http.Error(w, "Member not found in room", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "success"})
}

// Delete a room (admin only)
func handleDeleteRoom(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	roomID, err := strconv.Atoi(vars["id"])
	if err != nil {
		http.Error(w, "Invalid room ID", http.StatusBadRequest)
		return
	}

	userID := int(r.Context().Value("user_id").(float64))

	var role string
	err = db.QueryRow("SELECT role FROM room_members WHERE room_id = $1 AND user_id = $2", roomID, userID).Scan(&role)
	if err != nil || role != "admin" {
		http.Error(w, "Only admins can delete rooms", http.StatusForbidden)
		return
	}

	_, err = db.Exec("DELETE FROM rooms WHERE id = $1", roomID)
	if err != nil {
		log.Printf("Failed to delete room: %v", err)
		http.Error(w, "Failed to delete room", http.StatusInternalServerError)
		return
	}

	log.Printf("Room %d deleted by user %d", roomID, userID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "success"})
}

// Leave a room
func handleLeaveRoom(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	roomID, err := strconv.Atoi(vars["id"])
	if err != nil {
		http.Error(w, "Invalid room ID", http.StatusBadRequest)
		return
	}

	userID := int(r.Context().Value("user_id").(float64))

	var adminCount int
	err = db.QueryRow("SELECT COUNT(*) FROM room_members WHERE room_id = $1 AND role = 'admin'", roomID).Scan(&adminCount)
	if err != nil {
		http.Error(w, "Server error", http.StatusInternalServerError)
		return
	}

	var userRole string
	err = db.QueryRow("SELECT role FROM room_members WHERE room_id = $1 AND user_id = $2", roomID, userID).Scan(&userRole)
	if err != nil {
		http.Error(w, "You are not a member of this room", http.StatusForbidden)
		return
	}

	if userRole == "admin" && adminCount == 1 {
		http.Error(w, "Cannot leave: You are the only admin. Delete the room or promote another member first", http.StatusBadRequest)
		return
	}

	_, err = db.Exec("DELETE FROM room_members WHERE room_id = $1 AND user_id = $2", roomID, userID)
	if err != nil {
		log.Printf("Failed to leave room: %v", err)
		http.Error(w, "Failed to leave room", http.StatusInternalServerError)
		return
	}

	log.Printf("User %d left room %d", userID, roomID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "success"})
}

// Fetches all public/open rooms that the current user has NOT joined.
func handleGetAllRooms(w http.ResponseWriter, r *http.Request) {
    userIDFloat := r.Context().Value("user_id").(float64)
    userID := int(userIDFloat)

    query := `
        SELECT 
            r.id, r.name, r.description,
            (SELECT COUNT(*) FROM room_members rm_count WHERE rm_count.room_id = r.id) as members_count
        FROM rooms r
        LEFT JOIN room_members rm ON r.id = rm.room_id AND rm.user_id = $1
        WHERE rm.user_id IS NULL 
        ORDER BY r.created_at DESC
    `
    rows, err := db.Query(query, userID)
    if err != nil {
        log.Printf("DB error fetching explorable rooms for user %d: %v", userID, err)
        http.Error(w, "Failed to query rooms", http.StatusInternalServerError)
        return
    }
    defer rows.Close()

    var explorableRooms []Room
    for rows.Next() {
        var r Room
        var membersCount int

        if err := rows.Scan(&r.ID, &r.Name, &r.Description, &membersCount); err != nil {
            log.Println("Error scanning explorable room:", err)
            continue
        }
        
        r.Members = membersCount
        r.Avatar = string(r.Name[0])

        r.CreatedBy = 0 
        r.IsPrivate = false 
        r.LastMessage = ""
        r.LastMessageTime = ""
        r.Unread = 0 
        
        explorableRooms = append(explorableRooms, r)
    }

    w.Header().Set("Content-Type", "application/json")
    
    if len(explorableRooms) == 0 {
        w.Write([]byte("[]"))
        return
    }

    if err := json.NewEncoder(w).Encode(explorableRooms); err != nil {
        log.Printf("Error encoding response: %v", err)
        http.Error(w, "Error encoding response", http.StatusInternalServerError)
    }
}

// WebSocket handler
func handleWebSocket(w http.ResponseWriter, r *http.Request) {
	tokenString := r.URL.Query().Get("token")
	if tokenString == "" {
		http.Error(w, "Missing auth token", http.StatusUnauthorized)
		return
	}

	claims, err := parseJWT(tokenString)
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

	userID := int(claims["user_id"].(float64))
	username := claims["username"].(string)

	client := &Client{
		ID:       userID,
		Username: username,
		Avatar:   string(username[0]),
		Conn:     conn,
		Send:     make(chan *WSMessage, 256),
		Manager:  roomManager,
	}

	roomManager.Register <- client
	go client.readPump()
	go client.writePump()
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

// --- Main ---

func main() {
	initDB()
	defer db.Close()

	go roomManager.Run()

	r := mux.NewRouter()

	// Auth routes (no middleware)
	r.HandleFunc("/api/register", handleRegister).Methods("POST", "OPTIONS")
	r.HandleFunc("/api/login", handleLogin).Methods("POST", "OPTIONS")

	// API subrouter with auth middleware
	api := r.PathPrefix("/api").Subrouter()
	api.Use(authMiddleware)
	api.HandleFunc("/rooms", handleCreateRoom).Methods("POST", "OPTIONS")
	api.HandleFunc("/rooms", handleGetRooms).Methods("GET", "OPTIONS")
	api.HandleFunc("/rooms/{id}/messages", handleGetRoomMessages).Methods("GET", "OPTIONS")
	api.HandleFunc("/rooms/{id}/members", handleGetRoomMembers).Methods("GET", "OPTIONS")
	api.HandleFunc("/rooms/{id}/members/{memberId}", handleRemoveMember).Methods("DELETE", "OPTIONS")
	api.HandleFunc("/rooms/{id}/read", handleMarkRoomAsRead).Methods("POST", "OPTIONS")
	api.HandleFunc("/rooms/{id}", handleDeleteRoom).Methods("DELETE", "OPTIONS")
	api.HandleFunc("/rooms/{id}/leave", handleLeaveRoom).Methods("POST", "OPTIONS")
	api.HandleFunc("/rooms/explore", handleGetAllRooms).Methods("GET", "OPTIONS")
	api.HandleFunc("/rooms/{id}/join", handleJoinRoom).Methods("POST", "OPTIONS")

	// WebSocket route (token passed as query param, so no middleware)
	r.HandleFunc("/ws", handleWebSocket)

	http.Handle("/", enableCORS(r))

	port := getEnv("PORT", "8080")
	log.Printf("Server running on :%s\n", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}