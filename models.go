package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/dgrijalva/jwt-go"
	"github.com/microcosm-cc/bluemonday"
	"github.com/russross/blackfriday/v2"
	"golang.org/x/crypto/bcrypt"
)

type UserSettings struct {
	SoundEnabled   bool `json:"sound_enabled"`
	NotifyEnabled  bool `json:"notify_enabled"`
	DarkTheme      bool `json:"dark_theme"`
	ShowReadStatus bool `json:"show_read_status"`
}

type User struct {
	ID       int          `json:"id"`
	Username string       `json:"username"`
	Password string       `json:"password"`
	LastSeen time.Time    `json:"last_seen"`
	IsOnline bool         `json:"is_online"`
	Avatar   string       `json:"avatar"` // Base64 encoded image
	Settings UserSettings `json:"settings"`
}

type Message struct {
	ID         int       `json:"id"`
	FromUser   string    `json:"from_user"`
	ToUser     string    `json:"to_user"`
	Content    string    `json:"content"`
	CreatedAt  time.Time `json:"created_at"`
	IsRead     bool      `json:"is_read"`
	IsGroup    bool      `json:"is_group"`
	GroupUsers []string  `json:"group_users,omitempty"`
	HasFile    bool      `json:"has_file"`
	FileName   string    `json:"file_name,omitempty"`
	FileData   string    `json:"file_data,omitempty"` // Base64 encoded file
	IsEdited   bool      `json:"is_edited"`
	EditedAt   time.Time `json:"edited_at,omitempty"`
	ReplyTo    int       `json:"reply_to,omitempty"`
}

type Group struct {
	ID        int       `json:"id"`
	Name      string    `json:"name"`
	Users     []string  `json:"users"`
	CreatedAt time.Time `json:"created_at"`
}

type UserStatus struct {
	IsOnline   bool      `json:"is_online"`
	LastSeen   time.Time `json:"last_seen"`
	LastTyping time.Time `json:"last_typing"`
}

type UserProfile struct {
	User
	Bio          string    `json:"bio"`
	DisplayName  string    `json:"display_name"`
	ProfileImage string    `json:"profile_image"`
	JoinedAt     time.Time `json:"joined_at"`
}

type Claims struct {
	Username string `json:"username"`
	jwt.StandardClaims
}

var (
	userMutex    sync.RWMutex
	messageMutex sync.RWMutex
	usersFile    = "data/users.json"
	messagesFile = "data/messages.json"
	typingUsers  = make(map[string]map[string]bool) // map[from]map[to]isTyping
	typingMutex  sync.RWMutex
	userStatus   = make(map[string]UserStatus)
	statusMutex  sync.RWMutex
	messageCache = make(map[string][]Message) // кеш сообщений для каждого пользователя
	cacheMutex   sync.RWMutex
	cacheExpiry  = 5 * time.Minute
)

func init() {
	// Create data directory if it doesn't exist
	os.MkdirAll("data", 0755)

	// Create files if they don't exist
	if _, err := os.Stat(usersFile); os.IsNotExist(err) {
		saveUsers([]User{})
	}
	if _, err := os.Stat(messagesFile); os.IsNotExist(err) {
		saveMessages([]Message{})
	}
}

func loadUsers() []User {
	userMutex.RLock()
	defer userMutex.RUnlock()

	data, err := os.ReadFile(usersFile)
	if err != nil {
		return []User{}
	}

	var users []User
	json.Unmarshal(data, &users)
	return users
}

func saveUsers(users []User) error {
	userMutex.Lock()
	defer userMutex.Unlock()

	data, err := json.MarshalIndent(users, "", "    ")
	if err != nil {
		return err
	}
	return os.WriteFile(usersFile, data, 0644)
}

func loadMessages() []Message {
	messageMutex.RLock()
	defer messageMutex.RUnlock()

	data, err := os.ReadFile(messagesFile)
	if err != nil {
		return []Message{}
	}

	var messages []Message
	json.Unmarshal(data, &messages)
	return messages
}

func saveMessages(messages []Message) error {
	messageMutex.Lock()
	defer messageMutex.Unlock()

	data, err := json.MarshalIndent(messages, "", "    ")
	if err != nil {
		return err
	}
	return os.WriteFile(messagesFile, data, 0644)
}

func createUser(username, password string) error {
	// Validate input
	if len(username) < 3 || len(password) < 6 {
		return errors.New("username must be at least 3 characters and password at least 6 characters")
	}

	// Hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	users := loadUsers()
	newUser := User{
		ID:       len(users) + 1,
		Username: strings.TrimSpace(username),
		Password: string(hashedPassword),
	}
	users = append(users, newUser)
	return saveUsers(users)
}

func createUserWithAvatar(username, password, avatar string) error {
	// Validate input
	if len(username) < 3 || len(password) < 6 {
		return errors.New("username must be at least 3 characters and password at least 6 characters")
	}

	// Hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	users := loadUsers()
	newUser := User{
		ID:       len(users) + 1,
		Username: strings.TrimSpace(username),
		Password: string(hashedPassword),
		Avatar:   avatar,
	}
	users = append(users, newUser)
	return saveUsers(users)
}

func updateUserAvatar(username, avatar string) error {
	users := loadUsers()
	for i := range users {
		if users[i].Username == username {
			users[i].Avatar = avatar
			return saveUsers(users)
		}
	}
	return errors.New("user not found")
}

func updateUserSettings(username string, settings UserSettings) error {
	users := loadUsers()
	for i := range users {
		if users[i].Username == username {
			users[i].Settings = settings
			return saveUsers(users)
		}
	}
	return errors.New("user not found")
}

func validateUser(username, password string) bool {
	user := findUser(username)
	if user == nil {
		return false
	}
	err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password))
	return err == nil
}

func findUser(username string) *User {
	users := loadUsers()
	for _, u := range users {
		if u.Username == username {
			return &u
		}
	}
	return nil
}

func createMessage(from, to, content string) error {
	// Validate input
	if len(content) == 0 {
		return errors.New("message content cannot be empty")
	}
	if findUser(to) == nil {
		return errors.New("recipient user does not exist")
	}

	messages := loadMessages()
	newMessage := Message{
		ID:        len(messages) + 1,
		FromUser:  from,
		ToUser:    to,
		Content:   strings.TrimSpace(content),
		CreatedAt: time.Now(),
	}
	messages = append(messages, newMessage)
	return saveMessages(messages)
}

func updateUserStatus(username string, online bool) error {
	users := loadUsers()
	for i := range users {
		if users[i].Username == username {
			users[i].LastSeen = time.Now()
			users[i].IsOnline = online
			return saveUsers(users)
		}
	}
	return errors.New("user not found")
}

func getOnlineUsers() []string {
	users := loadUsers()
	var onlineUsers []string
	for _, u := range users {
		if u.IsOnline {
			onlineUsers = append(onlineUsers, u.Username)
		}
	}
	return onlineUsers
}

func deleteMessage(messageID int, username string) error {
	messages := loadMessages()
	for i, msg := range messages {
		if msg.ID == messageID {
			if msg.FromUser != username {
				return errors.New("can only delete your own messages")
			}
			messages = append(messages[:i], messages[i+1:]...)
			return saveMessages(messages)
		}
	}
	return errors.New("message not found")
}

func markMessageAsRead(messageID int, username string) error {
	messages := loadMessages()
	for i := range messages {
		if messages[i].ID == messageID && messages[i].ToUser == username {
			messages[i].IsRead = true
			return saveMessages(messages)
		}
	}
	return errors.New("message not found")
}

func createGroupMessage(from string, groupUsers []string, content string) error {
	if len(groupUsers) < 2 {
		return errors.New("group must have at least 2 recipients")
	}

	messages := loadMessages()
	newMessage := Message{
		ID:         len(messages) + 1,
		FromUser:   from,
		ToUser:     "group",
		Content:    strings.TrimSpace(content),
		CreatedAt:  time.Now(),
		IsGroup:    true,
		GroupUsers: groupUsers,
	}
	messages = append(messages, newMessage)
	return saveMessages(messages)
}

func getUserGroups(username string) [][]string {
	messages := loadMessages()
	groups := make(map[string]bool)
	var uniqueGroups [][]string

	for _, msg := range messages {
		if msg.IsGroup {
			key := strings.Join(msg.GroupUsers, ",")
			if !groups[key] && containsUser(msg.GroupUsers, username) {
				groups[key] = true
				uniqueGroups = append(uniqueGroups, msg.GroupUsers)
			}
		}
	}
	return uniqueGroups
}

func containsUser(users []string, username string) bool {
	for _, u := range users {
		if u == username {
			return true
		}
	}
	return false
}

func broadcastTypingStatus(from, to string, isTyping bool) {
	typingMutex.Lock()
	defer typingMutex.Unlock()

	if typingUsers[from] == nil {
		typingUsers[from] = make(map[string]bool)
	}
	typingUsers[from][to] = isTyping
}

func getTypingStatus(from, to string) bool {
	typingMutex.RLock()
	defer typingMutex.RUnlock()

	if users, ok := typingUsers[from]; ok {
		return users[to]
	}
	return false
}

func getMessageHistory(user1, user2 string) []Message {
	messages := loadMessages()
	var history []Message

	for _, msg := range messages {
		if (msg.FromUser == user1 && msg.ToUser == user2) ||
			(msg.FromUser == user2 && msg.ToUser == user1) {
			history = append(history, msg)
		}
	}
	return history
}

func processMessageContent(content string) string {
	// Convert Markdown to HTML
	unsafe := blackfriday.Run([]byte(content))
	// Sanitize HTML
	html := bluemonday.UGCPolicy().SanitizeBytes(unsafe)

	// Process emojis
	processed := string(html)
	emojis := map[string]string{
		":)": "😊", ":(": "😢", ":D": "😃",
		"<3": "❤️", ":P": "😛",
		":thumbsup:": "👍", ":ok:": "👌",
		":fire:": "🔥", ":star:": "⭐",
	}

	for text, emoji := range emojis {
		processed = strings.ReplaceAll(processed, text, emoji)
	}

	return processed
}

func editMessage(messageID int, username, newContent string) error {
	messages := loadMessages()
	for i := range messages {
		if messages[i].ID == messageID {
			if messages[i].FromUser != username {
				return errors.New("can only edit your own messages")
			}
			messages[i].Content = newContent
			messages[i].IsEdited = true
			messages[i].EditedAt = time.Now()
			return saveMessages(messages)
		}
	}
	return errors.New("message not found")
}

func searchMessages(username, query string) []Message {
	messages := loadMessages()
	var results []Message
	query = strings.ToLower(query)

	for _, msg := range messages {
		if msg.FromUser == username || msg.ToUser == username ||
			(msg.IsGroup && containsUser(msg.GroupUsers, username)) {
			if strings.Contains(strings.ToLower(msg.Content), query) {
				results = append(results, msg)
			}
		}
	}
	return results
}

func searchMessageHistory(username, query string, startDate, endDate time.Time) []Message {
	messages := loadMessages()
	var results []Message

	for _, msg := range messages {
		if (msg.FromUser == username || msg.ToUser == username) &&
			msg.CreatedAt.After(startDate) &&
			msg.CreatedAt.Before(endDate) {
			if query == "" ||
				strings.Contains(strings.ToLower(msg.Content), strings.ToLower(query)) {
				results = append(results, msg)
			}
		}
	}
	return results
}

func getUnreadCount(username string) int {
	messages := loadMessages()
	count := 0
	for _, msg := range messages {
		if msg.ToUser == username && !msg.IsRead {
			count++
		}
	}
	return count
}

func getUserStatus(username string) UserStatus {
	statusMutex.RLock()
	defer statusMutex.RUnlock()
	return userStatus[username]
}

func updateTypingTime(username string) {
	statusMutex.Lock()
	defer statusMutex.Unlock()
	if status, ok := userStatus[username]; ok {
		status.LastTyping = time.Now()
		userStatus[username] = status
	}
}

func validateFileUpload(fileName string, fileSize int64) error {
	// Check file size (max 10MB)
	if fileSize > 10*1024*1024 {
		return errors.New("file too large (max 10MB)")
	}

	// Check file extension
	ext := strings.ToLower(filepath.Ext(fileName))
	allowedExts := map[string]bool{
		".jpg": true, ".jpeg": true, ".png": true,
		".gif": true, ".pdf": true, ".doc": true,
		".docx": true, ".txt": true,
	}

	if !allowedExts[ext] {
		return errors.New("unsupported file type")
	}

	return nil
}

func validateMediaFile(fileName string, fileSize int64) error {
	// Расширяем список разрешенных форматов
	allowedExts := map[string]bool{
		".jpg": true, ".jpeg": true, ".png": true, ".gif": true,
		".mp4": true, ".webm": true, ".mov": true,
		".pdf": true, ".doc": true, ".docx": true, ".txt": true,
	}

	maxSizes := map[string]int64{
		"image":    10 * 1024 * 1024, // 10MB
		"video":    50 * 1024 * 1024, // 50MB
		"document": 20 * 1024 * 1024, // 20MB
	}

	ext := strings.ToLower(filepath.Ext(fileName))
	if !allowedExts[ext] {
		return errors.New("unsupported file type")
	}

	// Определяем тип файла и проверяем размер
	switch {
	case strings.HasPrefix(ext, ".jp"), ext == ".png", ext == ".gif":
		if fileSize > maxSizes["image"] {
			return errors.New("image too large (max 10MB)")
		}
	case ext == ".mp4", ext == ".webm", ext == ".mov":
		if fileSize > maxSizes["video"] {
			return errors.New("video too large (max 50MB)")
		}
	default:
		if fileSize > maxSizes["document"] {
			return errors.New("file too large (max 20MB)")
		}
	}
	return nil
}

func sanitizeContent(content string) string {
	// Создаем безопасную политику HTML
	p := bluemonday.UGCPolicy()
	p.AllowStandardURLs()
	p.AllowStandardAttributes()
	// Разрешаем безопасные HTML теги
	p.AllowElements("b", "i", "u", "em", "strong", "a", "code", "pre")

	return p.Sanitize(content)
}

func exportMessageHistory(username string) ([]byte, error) {
	messages := loadMessages()
	var userMessages []Message

	for _, msg := range messages {
		if msg.FromUser == username || msg.ToUser == username {
			userMessages = append(userMessages, msg)
		}
	}

	data := struct {
		User       string    `json:"user"`
		Messages   []Message `json:"messages"`
		ExportDate time.Time `json:"export_date"`
	}{
		User:       username,
		Messages:   userMessages,
		ExportDate: time.Now(),
	}

	return json.MarshalIndent(data, "", "    ")
}

func generateToken(username string) (string, error) {
	expirationTime := time.Now().Add(24 * time.Hour)
	claims := &Claims{
		Username: username,
		StandardClaims: jwt.StandardClaims{
			ExpiresAt: expirationTime.Unix(),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(jwtKey)
}

func updateProfile(username string, profile UserProfile) error {
	users := loadUsers()
	for i := range users {
		if users[i].Username == username {
			users[i].Avatar = profile.Avatar
			// Additional profile fields...
			return saveUsers(users)
		}
	}
	return errors.New("user not found")
}

// Add WebSocket broadcast function
func broadcastMessage(msg Message) {
	clientsMutex.RLock()
	defer clientsMutex.RUnlock()

	for username, conn := range clients {
		if msg.ToUser == username || msg.FromUser == username {
			conn.WriteJSON(msg)
		}
	}
}

func getCachedMessages(username string) []Message {
	cacheMutex.RLock()
	defer cacheMutex.RUnlock()

	if messages, ok := messageCache[username]; ok {
		return messages
	}
	return nil
}

func cacheMessages(username string, messages []Message) {
	cacheMutex.Lock()
	defer cacheMutex.Unlock()

	messageCache[username] = messages
	go func() {
		time.Sleep(cacheExpiry)
		cacheMutex.Lock()
		delete(messageCache, username)
		cacheMutex.Unlock()
	}()
}

func formatMessage(msg Message) Message {
	// Форматируем текст сообщения
	msg.Content = processMessageContent(msg.Content)

	// Добавляем метаданные
	if msg.IsGroup {
		msg.Content = fmt.Sprintf("[Group Message] %s", msg.Content)
	}

	// Добавляем информацию о файле
	if msg.HasFile {
		fileInfo := fmt.Sprintf("\n[Attachment: %s]", msg.FileName)
		msg.Content = msg.Content + fileInfo
	}

	// Добавляем информацию о редактировании
	if msg.IsEdited {
		msg.Content = fmt.Sprintf("%s (edited at %s)",
			msg.Content,
			msg.EditedAt.Format("15:04:05"))
	}

	return msg
}
