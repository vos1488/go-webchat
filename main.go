package main

import (
	"log"
	"net/http"
	"sync"

	"github.com/gorilla/mux"
	"github.com/gorilla/sessions"
	"github.com/gorilla/websocket"
)

var (
	store    = sessions.NewCookieStore([]byte("secret-key"))
	upgrader = websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin:     func(r *http.Request) bool { return true },
	}
	jwtKey              = []byte("your-secret-key")
	clients             = make(map[string]*websocket.Conn)
	clientsMutex        sync.RWMutex
	notificationService *NotificationService
)

func init() {
	notificationService = NewNotificationService()
}

func main() {
	r := mux.NewRouter()

	// Static files
	r.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))

	// Auth routes
	r.HandleFunc("/register", handleRegister).Methods("GET", "POST")
	r.HandleFunc("/login", handleLogin).Methods("GET", "POST")
	r.HandleFunc("/logout", handleLogout).Methods("GET")

	// Message routes
	r.HandleFunc("/messages", handleMessages).Methods("GET")
	r.HandleFunc("/send", handleSendMessage).Methods("POST")

	// WebSocket route
	r.HandleFunc("/ws", handleWebSocket)

	// Profile routes
	r.HandleFunc("/profile", handleProfile).Methods("GET", "POST")
	// Remove undefined handler

	// Enhanced API routes with JWT middleware
	api := r.PathPrefix("/api").Subrouter()
	api.Use(jwtAuthMiddleware)

	// API routes
	r.HandleFunc("/api/messages", handleAPI)
	r.HandleFunc("/api/users/online", handleAPI)
	r.HandleFunc("/api/messages/delete", handleAPI)
	r.HandleFunc("/api/messages/edit", handleEditMessage).Methods("POST") // Добавляем маршрут для редактирования
	api.HandleFunc("/messages/reply", handleReplyMessage).Methods("POST") // Добавляем маршрут для ответов

	// Notification routes
	r.HandleFunc("/api/notifications", handleNotifications).Methods("GET", "POST")

	// Добавляем новые API endpoints
	api.HandleFunc("/messages/search", handleMessageSearch).Methods("GET")
	api.HandleFunc("/messages/stats", handleMessageStats).Methods("GET")
	api.HandleFunc("/users/status", handleUserStatus).Methods("GET")
	api.HandleFunc("/groups/create", handleCreateGroup).Methods("POST")
	api.HandleFunc("/settings", handleSettings).Methods("GET", "POST")

	// Add reaction endpoint
	api.HandleFunc("/messages/react", handleMessageReaction).Methods("POST")

	// Add message logs endpoint
	api.HandleFunc("/messages/logs", handleMessageLogs).Methods("GET")

	// Home page
	r.HandleFunc("/", handleHome).Methods("GET")

	log.Fatal(http.ListenAndServe(":8080", r))
}
