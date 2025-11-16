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
	"github.com/gorilla/mux" // Using gorilla/mux for easier route parameters
	"github.com/gorilla/websocket"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
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
	// Fields from frontend mock, added for consistency
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
	// // START: ADDED DROP TABLE STATEMENTS
    // dropSchema := `
    // -- Drop tables in order of dependency
    // DROP TABLE IF EXISTS messages CASCADE;
    // DROP TABLE IF EXISTS room_members CASCADE;
    // DROP TABLE IF EXISTS rooms CASCADE;
    // DROP TABLE IF EXISTS users CASCADE;
    // `
    // if _, err := db.Exec(dropSchema); err != nil {
    //     // Log this as a fatal error if cleanup fails
    //     log.Fatal("Failed to drop existing tables:", err) 
    // }
    // // END: ADDED DROP TABLE STATEMENTS

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
    `

	if _, err := db.Exec(schema); err != nil {
		log.Fatal("Failed to create tables:", err)
	}

	// --- Create System User ---
    // 1. Ensure the sequence starts at 1, even if dropped/recreated
    if _, err := db.Exec("ALTER SEQUENCE users_id_seq RESTART WITH 1"); err != nil {
        log.Fatal("Failed to reset users sequence:", err)
    }

    // 2. Insert the System user (ID 1) if they don't exist.
    // We use an empty password hash as this user cannot log in.
    // The username 'System' is used, and the password_hash is set to a non-null placeholder.
    systemUserSQL := `
    INSERT INTO users (id, username, email, password_hash)
    VALUES (1, 'System', 'system@chathub.io', 'SYSTEM_ACCOUNT_HASH')
    ON CONFLICT (id) DO NOTHING;
    `
    if _, err := db.Exec(systemUserSQL); err != nil {
        log.Fatal("Failed to create System user:", err)
    }

    // 3. Increment the sequence past 1 so that the next *real* user starts at ID 2
    if _, err := db.Exec("SELECT setval('users_id_seq', 2, false)"); err != nil {
        log.Fatal("Failed to update users sequence value:", err)
    }

    log.Println("✅ System user (ID 1) confirmed.")
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
			// Don't add to any room yet, wait for 'joinRoom' message
		case client := <-m.Unregister:
			log.Printf("Client %s disconnected", client.Username)
			// Remove client from all rooms they are in
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
					// Failed to send, assume client disconnected
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
			// Check if user is allowed in room
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

			// Save message to DB
			var savedMsg Message
			err := db.QueryRow(
				"INSERT INTO messages (room_id, sender_id, content) VALUES ($1, $2, $3) RETURNING id, room_id, sender_id, content, created_at",
				msg.RoomID, c.ID, msg.Content,
			).Scan(&savedMsg.ID, &savedMsg.RoomID, &savedMsg.SenderID, &savedMsg.Text, &savedMsg.Timestamp)

			if err != nil {
				log.Println("Failed to save message:", err)
				continue
			}

			// Populate sender info for broadcast
			savedMsg.Sender = c.Username
			savedMsg.Avatar = c.Avatar
			savedMsg.Read = false // Default read status

			// Get the hub and broadcast
			hub := c.Manager.GetOrCreateRoomHub(msg.RoomID)
			hub.Broadcast <- &WSMessage{
				Type:    "roomMessage",
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

		// Add claims to request context
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
	// ... (rest of register logic is unchanged)
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
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}
	// ... (rest of login logic is unchanged)
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

	// Start transaction
	tx, err := db.Begin()
	if err != nil {
		http.Error(w, "Server error", http.StatusInternalServerError)
		return
	}
	defer tx.Rollback() // Rollback if anything fails

	// 1. Create room
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

	// 2. Add creator as admin member
	_, err = tx.Exec(
		"INSERT INTO room_members (room_id, user_id, role) VALUES ($1, $2, $3)",
		roomID, userID, "admin",
	)
	if err != nil {
		log.Println("Failed to add room member:", err)
		http.Error(w, "Failed to create room", http.StatusInternalServerError)
		return
	}

	// 3. Insert System Message ("{{username}} created the room.")
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

	// 4. Broadcast the system message via WebSocket
	hub := roomManager.GetOrCreateRoomHub(roomID)
	savedMsg.Sender = "System"
	savedMsg.Avatar = "S"
	savedMsg.Read = false

	hub.Broadcast <- &WSMessage{
		Type:    "roomMessage",
		Message: &savedMsg,
	}

	// 5. Return the new room object
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

	// 1. Check if room exists and get details
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

	// 2. Check if user is already a member
	if isUserInRoom(userID, roomID) {
		http.Error(w, "Already a member of this room", http.StatusConflict)
		return
	}

	// 3. Start transaction for JOIN and SYSTEM MESSAGE
	tx, err := db.Begin()
	if err != nil {
		http.Error(w, "Server error", http.StatusInternalServerError)
		return
	}
	defer tx.Rollback()

	// 3a. Add user to room_members
	_, err = tx.Exec(
		"INSERT INTO room_members (room_id, user_id, role) VALUES ($1, $2, $3)",
		roomID, userID, "member",
	)
	if err != nil {
		log.Printf("Failed to add room member: %v", err)
		http.Error(w, "Failed to join room", http.StatusInternalServerError)
		return
	}

	// 3b. Add a system message ("{{username}} joined the room at 3:04 PM on Jan 2, 2006.")
	currentTime := time.Now()
	formattedTime := currentTime.Format(SystemMessageTimeFormat)
	systemMessageContent := fmt.Sprintf("%s joined this room at %s.", username, formattedTime)

	// For simplicity, we assume sender_id=1 is the system.
	var savedMsg Message
	_, err = tx.Exec(
		"INSERT INTO messages (room_id, sender_id, content) VALUES ($1, $2, $3)",
		roomID, 1, systemMessageContent,
	)
	if err != nil {
		log.Printf("Failed to add system message: %v", err)
		http.Error(w, "Failed to join room (message fail)", http.StatusInternalServerError)
		return
	}

	if err := tx.Commit(); err != nil {
		http.Error(w, "Server error during commit", http.StatusInternalServerError)
		return
	}

	// 4. Prepare and Broadcast System Message via WebSocket
	hub := roomManager.GetOrCreateRoomHub(roomID)

	// Populate sender info for broadcast
	savedMsg.Sender = "System"
	savedMsg.Avatar = "S" // Avatar for System user
	savedMsg.Read = false

	hub.Broadcast <- &WSMessage{
		Type:     "roomMessage",
		Message:  &savedMsg,
	}
	log.Printf("Broadcasted system message to room %d: %s", roomID, savedMsg.Text)

	// 5. Return the full Room object with updated details
	room.Members = membersCount + 1 // Reflect the new member
	room.Avatar = string(room.Name[0])
	room.LastMessage = fmt.Sprintf("You joined this room at %s.", formattedTime) // Set client-side message
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
            -- NOTE: For simplicity, unread count is hardcoded to 0 in the Go scan for now, 
            -- but you would calculate it here in production.
            0 AS unread_count 
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
        
        // Use sql.Null types to handle potential NULL values from the LEFT JOIN
        var lastMessage sql.NullString
        var lastMessageTime sql.NullTime
		var lastSenderID sql.NullInt64
        
        if err := rows.Scan(
            &room.ID, &room.Name, &room.Description, &room.CreatedBy, &room.CreatedAt, &room.IsPrivate,
            &membersCount,
            &lastMessage,
            &lastMessageTime,
			&lastSenderID,
            &unreadCount, // Scanning the unread_count column
        ); err != nil {
            log.Println("Error scanning room:", err)
            continue
        }
        
        // --- Process and format data ---

        // Assign numeric values
        room.Members = membersCount
        room.Unread = unreadCount

        // Assign actual message content, handling NULL
        if lastMessage.Valid {
            room.LastMessage = lastMessage.String
        } else {
            // For rooms with NO messages (but the user has joined), show status
            room.LastMessage = "No messages yet." 
        }

        // Assign actual timestamp, handling NULL
        if lastMessageTime.Valid {
            room.LastMessageTime = lastMessageTime.Time.Format("3:04 PM")
        } else {
            // If no messages exist, use a placeholder time like room creation time
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

	// Check if user is in the room
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
		m.Read = true // Assume all historical messages are read
		messages = append(messages, m)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(messages)
}

// Fetches all public/open rooms that the current user has NOT joined.
func handleGetAllRooms(w http.ResponseWriter, r *http.Request) {
    // 1. Get User ID from Context
    userIDFloat := r.Context().Value("user_id").(float64)
    userID := int(userIDFloat)

    // 2. Database Query: Simplified for schema without 'is_private'
    // This query selects all rooms that the user ($1) is NOT a member of.
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

        // 3. Scan only the necessary fields
        if err := rows.Scan(&r.ID, &r.Name, &r.Description, &membersCount); err != nil {
            log.Println("Error scanning explorable room:", err)
            continue
        }
        
        // Populate required fields for the frontend exploration view
        r.Members = membersCount
        r.Avatar = string(r.Name[0]) // Avatar is the first letter of the name

        // Set other fields to default/empty values to keep the Room struct structure consistent
        r.CreatedBy = 0 
        r.IsPrivate = false 
        r.LastMessage = ""
        r.LastMessageTime = ""
        r.Unread = 0 
        
        explorableRooms = append(explorableRooms, r)
    }

    // 4. JSON Response
    w.Header().Set("Content-Type", "application/json")
    
    // Ensure we send '[]' for an empty list
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
	api.HandleFunc("/rooms/explore", handleGetAllRooms).Methods("GET", "OPTIONS")
	api.HandleFunc("/rooms/{id}/join", handleJoinRoom).Methods("POST", "OPTIONS") // <-- ADD THIS

	// WebSocket route (token passed as query param, so no middleware)
	r.HandleFunc("/ws", handleWebSocket)

	// Add CORS
	http.Handle("/", enableCORS(r))

	port := getEnv("PORT", "8080")
	log.Printf("Server running on :%s\n", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}