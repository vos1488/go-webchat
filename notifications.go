package main

import (
	"encoding/json"
	"log"
	"sync"
	"time"
)

// Notification represents a single notification
type Notification struct {
	ID        int       `json:"id"`
	UserID    string    `json:"user_id"`
	Type      string    `json:"type"`
	Message   string    `json:"message"`
	Read      bool      `json:"read"`
	CreatedAt time.Time `json:"created_at"`
}

// NotificationService represents the notification service structure
type NotificationService struct {
	notifications map[string][]Notification
	mutex         sync.RWMutex
	webhooks      []string
}

// NewNotificationService creates a new notification service
func NewNotificationService() *NotificationService {
	return &NotificationService{
		notifications: make(map[string][]Notification),
		webhooks:      make([]string, 0),
	}
}

func (s *NotificationService) Add(userId string, notifType string, message string) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	notif := Notification{
		ID:        len(s.notifications[userId]) + 1,
		UserID:    userId,
		Type:      notifType,
		Message:   message,
		CreatedAt: time.Now(),
	}

	s.notifications[userId] = append(s.notifications[userId], notif)
	s.triggerWebhooks(notif)
}

func (s *NotificationService) AddGroupNotification(groupUsers []string, message string) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	for _, userId := range groupUsers {
		notif := Notification{
			ID:        len(s.notifications[userId]) + 1,
			UserID:    userId,
			Type:      "group_message",
			Message:   message,
			CreatedAt: time.Now(),
		}
		s.notifications[userId] = append(s.notifications[userId], notif)
	}
}

func (s *NotificationService) GetUnread(userId string) []Notification {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	var unread []Notification
	for _, n := range s.notifications[userId] {
		if !n.Read {
			unread = append(unread, n)
		}
	}
	return unread
}

func (s *NotificationService) GetGroupNotifications(userId string) []Notification {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	var notifications []Notification
	for _, n := range s.notifications[userId] {
		if n.Type == "group_message" {
			notifications = append(notifications, n)
		}
	}
	return notifications
}

func (s *NotificationService) MarkRead(userId string, notifId int) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	for i := range s.notifications[userId] {
		if s.notifications[userId][i].ID == notifId {
			s.notifications[userId][i].Read = true
			break
		}
	}
}

func (s *NotificationService) AddWebhook(url string) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.webhooks = append(s.webhooks, url)
}

func (s *NotificationService) triggerWebhooks(notif Notification) {
	go func() {
		data, _ := json.Marshal(notif)
		for _, url := range s.webhooks {
			// Отправляем уведомление на вебхук
			// В реальном приложении здесь должна быть HTTP POST запрос
			log.Printf("Webhook triggered: %s, data: %s", url, string(data))
		}
	}()
}

// Добавляем новые методы в NotificationService
func (s *NotificationService) GetAllByUser(userId string) []Notification {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	return s.notifications[userId]
}

func (s *NotificationService) ClearAll(userId string) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	delete(s.notifications, userId)
}

func (s *NotificationService) MarkAllRead(userId string) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	for i := range s.notifications[userId] {
		s.notifications[userId][i].Read = true
	}
}
