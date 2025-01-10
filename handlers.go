package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/dgrijalva/jwt-go"
)

func handleHome(w http.ResponseWriter, r *http.Request) {
	session, _ := store.Get(r, "session-name")
	username, ok := session.Values["username"].(string)
	if !ok {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	tmpl := template.Must(template.ParseFiles("templates/home.html"))
	tmpl.Execute(w, username)
}

func handleRegister(w http.ResponseWriter, r *http.Request) {
	if r.Method == "POST" {
		username := strings.TrimSpace(r.FormValue("username"))
		password := r.FormValue("password")

		if findUser(username) != nil {
			http.Error(w, "User already exists", http.StatusBadRequest)
			return
		}

		if err := createUser(username, password); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	tmpl := template.Must(template.ParseFiles("templates/register.html"))
	tmpl.Execute(w, nil)
}

func handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method == "POST" {
		username := strings.TrimSpace(r.FormValue("username"))
		password := r.FormValue("password")

		if !validateUser(username, password) {
			http.Error(w, "Invalid credentials", http.StatusUnauthorized)
			return
		}

		updateUserStatus(username, true)
		session, _ := store.Get(r, "session-name")
		session.Values["username"] = username
		session.Save(r, w)

		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	tmpl := template.Must(template.ParseFiles("templates/login.html"))
	tmpl.Execute(w, nil)
}

func handleLogout(w http.ResponseWriter, r *http.Request) {
	session, _ := store.Get(r, "session-name")
	if username, ok := session.Values["username"].(string); ok {
		updateUserStatus(username, false)
	}
	session.Values = make(map[interface{}]interface{})
	session.Save(r, w)
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

func handleMessages(w http.ResponseWriter, r *http.Request) {
	session, _ := store.Get(r, "session-name")
	username, ok := session.Values["username"].(string)
	if !ok {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	// Filter messages using the loaded messages from JSON
	allMessages := loadMessages()
	userMessages := []Message{}
	for _, msg := range allMessages {
		if msg.FromUser == username || msg.ToUser == username {
			userMessages = append(userMessages, msg)
		}
	}

	data := struct {
		Messages    []Message
		OnlineUsers []string
		CurrentUser string
	}{
		Messages:    userMessages,
		OnlineUsers: getOnlineUsers(),
		CurrentUser: username,
	}

	tmpl := template.Must(template.ParseFiles("templates/messages.html"))
	tmpl.Execute(w, data)
}

func handleSendMessage(w http.ResponseWriter, r *http.Request) {
	session, _ := store.Get(r, "session-name")
	from, ok := session.Values["username"].(string)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	to := strings.TrimSpace(r.FormValue("to"))
	content := strings.TrimSpace(r.FormValue("content"))
	isGroup := r.FormValue("is_group") == "true"

	// Create message based on type
	var err error
	if isGroup {
		groupUsers := strings.Split(to, ",")
		err = createGroupMessage(from, groupUsers, content)
	} else {
		// Check for file attachment
		file, header, fileErr := r.FormFile("attachment")
		if fileErr == nil {
			defer file.Close()
			// Validate file
			if err := validateFileUpload(header.Filename, header.Size); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			data, err := io.ReadAll(file)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			fileData := base64.StdEncoding.EncodeToString(data)
			fileName := header.Filename

			messages := loadMessages()
			newMessage := Message{
				ID:        len(messages) + 1,
				FromUser:  from,
				ToUser:    to,
				Content:   content,
				CreatedAt: time.Now(),
				HasFile:   true,
				FileName:  fileName,
				FileData:  fileData,
			}
			err = saveMessages(append(messages, newMessage))
		} else {
			err = createMessage(from, to, content)
		}
	}

	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Добавляем уведомление получателю
	notificationService.Add(to, "new_message",
		fmt.Sprintf("New message from %s", from))

	http.Redirect(w, r, "/messages", http.StatusSeeOther)
}

func handleAPI(w http.ResponseWriter, r *http.Request) {
	session, _ := store.Get(r, "session-name")
	username, ok := session.Values["username"].(string)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	switch r.URL.Path {
	case "/api/messages":
		allMessages := loadMessages()
		userMessages := []Message{}
		query := strings.ToLower(r.URL.Query().Get("q"))

		for _, msg := range allMessages {
			if msg.FromUser == username || msg.ToUser == username {
				if query == "" || strings.Contains(strings.ToLower(msg.Content), query) {
					userMessages = append(userMessages, msg)
				}
			}
		}
		json.NewEncoder(w).Encode(userMessages)

	case "/api/users/online":
		json.NewEncoder(w).Encode(getOnlineUsers())

	case "/api/messages/delete":
		if r.Method != "POST" {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var reqData struct {
			MessageID int `json:"message_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&reqData); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		if err := deleteMessage(reqData.MessageID, username); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusOK)

	case "/api/messages/read":
		if r.Method != "POST" {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var reqData struct {
			MessageID int `json:"message_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&reqData); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		if err := markMessageAsRead(reqData.MessageID, username); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusOK)

	case "/api/groups":
		groups := getUserGroups(username)
		json.NewEncoder(w).Encode(groups)

	case "/api/typing":
		var typingData struct {
			ToUser   string `json:"to_user"`
			IsTyping bool   `json:"is_typing"`
		}
		if err := json.NewDecoder(r.Body).Decode(&typingData); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		broadcastTypingStatus(username, typingData.ToUser, typingData.IsTyping)
		w.WriteHeader(http.StatusOK)

	case "/api/history":
		withUser := r.URL.Query().Get("with")
		if withUser == "" {
			http.Error(w, "User parameter required", http.StatusBadRequest)
			return
		}
		history := getMessageHistory(username, withUser)
		json.NewEncoder(w).Encode(history)

	case "/api/avatar":
		var avatarData struct {
			Avatar string `json:"avatar"`
		}
		if err := json.NewDecoder(r.Body).Decode(&avatarData); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := updateUserAvatar(username, avatarData.Avatar); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusOK)

	case "/api/messages/edit":
		if r.Method != "POST" {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var reqData struct {
			MessageID  int    `json:"message_id"`
			NewContent string `json:"content"`
		}
		if err := json.NewDecoder(r.Body).Decode(&reqData); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		if err := editMessage(reqData.MessageID, username, reqData.NewContent); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusOK)

	case "/api/messages/reply":
		if r.Method != "POST" {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var reqData struct {
			ReplyTo int    `json:"reply_to"`
			Content string `json:"content"`
			ToUser  string `json:"to_user"`
		}
		if err := json.NewDecoder(r.Body).Decode(&reqData); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		messages := loadMessages()
		newMessage := Message{
			ID:        len(messages) + 1,
			FromUser:  username,
			ToUser:    reqData.ToUser,
			Content:   reqData.Content,
			CreatedAt: time.Now(),
			ReplyTo:   reqData.ReplyTo,
		}
		if err := saveMessages(append(messages, newMessage)); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusOK)

	case "/api/preview":
		var previewData struct {
			Content string `json:"content"`
		}
		if err := json.NewDecoder(r.Body).Decode(&previewData); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		processed := processMessageContent(previewData.Content)
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(processed))

	case "/api/messages/export":
		data, err := exportMessageHistory(username)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Disposition",
			fmt.Sprintf("attachment; filename=messages_%s_%s.json",
				username, time.Now().Format("2006-01-02")))
		w.Write(data)

	case "/api/messages/upload":
		file, header, err := r.FormFile("file")
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		defer file.Close()

		if err := validateMediaFile(header.Filename, header.Size); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		// Продолжаем обработку файла...
	}
}

func jwtAuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tokenString := r.Header.Get("Authorization")
		if tokenString == "" {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		claims := &Claims{}
		token, err := jwt.ParseWithClaims(tokenString, claims, func(t *jwt.Token) (interface{}, error) {
			return jwtKey, nil
		})

		if err != nil || !token.Valid {
			http.Error(w, "Invalid token", http.StatusUnauthorized)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade error: %v", err)
		return
	}

	session, _ := store.Get(r, "session-name")
	username := session.Values["username"].(string)

	// Add user to active connections
	clientsMutex.Lock()
	clients[username] = conn
	clientsMutex.Unlock()

	// Create message channel for this user
	messageChan := make(chan Message, 10)
	go handleUserMessages(username, messageChan)

	defer func() {
		clientsMutex.Lock()
		delete(clients, username)
		clientsMutex.Unlock()
		close(messageChan)
		conn.Close()
	}()

	// Read messages from WebSocket
	for {
		var msg Message
		if err := conn.ReadJSON(&msg); err != nil {
			break
		}

		// Format and broadcast message
		msg = formatMessage(msg)
		if msg.IsGroup {
			broadcastGroupMessage(msg)
		} else {
			messageChan <- msg
		}
	}
}

func handleUserMessages(username string, ch chan Message) {
	for msg := range ch {
		// Get cached messages or load from storage
		messages := getCachedMessages(username)
		if messages == nil {
			messages = loadMessages()
			cacheMessages(username, messages)
		}

		// Add new message
		messages = append(messages, msg)
		saveMessages(messages)

		// Update cache
		cacheMessages(username, messages)

		// Notify recipient
		if client, ok := clients[msg.ToUser]; ok {
			client.WriteJSON(msg)
		}
	}
}

func broadcastGroupMessage(msg Message) {
	clientsMutex.RLock()
	defer clientsMutex.RUnlock()

	for _, user := range msg.GroupUsers {
		if conn, ok := clients[user]; ok {
			conn.WriteJSON(msg)
		}
	}
}

func handleProfile(w http.ResponseWriter, r *http.Request) {
	session, _ := store.Get(r, "session-name")
	username := session.Values["username"].(string)

	if r.Method == "POST" {
		var profile UserProfile
		if err := json.NewDecoder(r.Body).Decode(&profile); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		if err := updateProfile(username, profile); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		w.WriteHeader(http.StatusOK)
		return
	}

	user := findUser(username)
	if user == nil {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	tmpl := template.Must(template.ParseFiles("templates/profile.html"))
	tmpl.Execute(w, user)
}

func handleNotifications(w http.ResponseWriter, r *http.Request) {
	session, _ := store.Get(r, "session-name")
	username, ok := session.Values["username"].(string)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	switch r.Method {
	case "GET":
		notifications := notificationService.GetUnread(username)
		json.NewEncoder(w).Encode(notifications)
	case "POST":
		var reqData struct {
			NotificationID int `json:"notification_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&reqData); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		notificationService.MarkRead(username, reqData.NotificationID)
		w.WriteHeader(http.StatusOK)
	}
}

func handleMessageSearch(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	startDate := r.URL.Query().Get("start")
	endDate := r.URL.Query().Get("end")

	// Конвертируем даты
	start, _ := time.Parse("2006-01-02", startDate)
	end, _ := time.Parse("2006-01-02", endDate)

	session, _ := store.Get(r, "session-name")
	username := session.Values["username"].(string)

	results := searchMessageHistory(username, query, start, end)
	json.NewEncoder(w).Encode(results)
}

func handleMessageStats(w http.ResponseWriter, r *http.Request) {
	session, _ := store.Get(r, "session-name")
	username := session.Values["username"].(string)

	stats := struct {
		TotalMessages int
		UnreadCount   int
		GroupCount    int
	}{
		TotalMessages: len(loadMessages()),
		UnreadCount:   getUnreadCount(username),
		GroupCount:    len(getUserGroups(username)),
	}

	json.NewEncoder(w).Encode(stats)
}

func handleSettings(w http.ResponseWriter, r *http.Request) {
	session, _ := store.Get(r, "session-name")
	username := session.Values["username"].(string)

	if r.Method == "POST" {
		var settings UserSettings
		json.NewDecoder(r.Body).Decode(&settings)
		updateUserSettings(username, settings)
		w.WriteHeader(http.StatusOK)
		return
	}

	user := findUser(username)
	json.NewEncoder(w).Encode(user.Settings)
}

func handleUserStatus(w http.ResponseWriter, r *http.Request) {
	session, _ := store.Get(r, "session-name")
	username := session.Values["username"].(string)

	status := getUserStatus(username)
	if status == (UserStatus{}) {
		status = UserStatus{
			IsOnline: true,
			LastSeen: time.Now(),
		}
	}

	json.NewEncoder(w).Encode(status)
}

func handleCreateGroup(w http.ResponseWriter, r *http.Request) {
	session, _ := store.Get(r, "session-name")
	username := session.Values["username"].(string)

	var groupData struct {
		Name  string   `json:"name"`
		Users []string `json:"users"`
	}

	if err := json.NewDecoder(r.Body).Decode(&groupData); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Добавляем создателя группы в список участников
	groupData.Users = append(groupData.Users, username)

	// Создаем группу
	group := Group{
		ID:        len(getUserGroups(username)) + 1,
		Name:      groupData.Name,
		Users:     groupData.Users,
		CreatedAt: time.Now(),
	}

	// Отправляем первое сообщение в группу
	err := createGroupMessage(username, groupData.Users,
		fmt.Sprintf("Group '%s' created by %s", groupData.Name, username))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(group)
}
