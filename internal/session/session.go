package session

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"net/http"
	"sync"
	"time"
)

const SessionName = "veriqid-session"

// SessionData holds per-user session state.
type SessionData struct {
	SpkHex   string
	Username string
	LoggedIn bool
	Expiry   time.Time
}

// Manager stores sessions server-side keyed by an HMAC'd cookie ID.
type Manager struct {
	mu       sync.RWMutex
	sessions map[string]*SessionData
	secret   []byte
}

// NewManager creates a session manager with the given secret key.
func NewManager(secret []byte) *Manager {
	m := &Manager{
		sessions: make(map[string]*SessionData),
		secret:   secret,
	}
	// Cleanup expired sessions every 5 minutes
	go func() {
		for range time.Tick(5 * time.Minute) {
			m.cleanup()
		}
	}()
	return m
}

func (m *Manager) generateID() string {
	b := make([]byte, 32)
	rand.Read(b)
	mac := hmac.New(sha256.New, m.secret)
	mac.Write(b)
	return base64.URLEncoding.EncodeToString(mac.Sum(nil))
}

// Login creates a session and sets the cookie.
func (m *Manager) Login(w http.ResponseWriter, r *http.Request, spkHex, username string) error {
	sessionID := m.generateID()
	m.mu.Lock()
	m.sessions[sessionID] = &SessionData{
		SpkHex:   spkHex,
		Username: username,
		LoggedIn: true,
		Expiry:   time.Now().Add(1 * time.Hour),
	}
	m.mu.Unlock()

	http.SetCookie(w, &http.Cookie{
		Name:     SessionName,
		Value:    sessionID,
		Path:     "/",
		MaxAge:   3600,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
	return nil
}

// Logout clears the session and deletes the cookie.
func (m *Manager) Logout(w http.ResponseWriter, r *http.Request) error {
	cookie, err := r.Cookie(SessionName)
	if err == nil {
		m.mu.Lock()
		delete(m.sessions, cookie.Value)
		m.mu.Unlock()
	}
	http.SetCookie(w, &http.Cookie{
		Name:   SessionName,
		Value:  "",
		Path:   "/",
		MaxAge: -1,
	})
	return nil
}

// IsLoggedIn checks if the user has an active session.
// Returns (loggedIn, spkHex, username).
func (m *Manager) IsLoggedIn(r *http.Request) (bool, string, string) {
	cookie, err := r.Cookie(SessionName)
	if err != nil {
		return false, "", ""
	}
	m.mu.RLock()
	sess, ok := m.sessions[cookie.Value]
	m.mu.RUnlock()
	if !ok || !sess.LoggedIn || time.Now().After(sess.Expiry) {
		return false, "", ""
	}
	return true, sess.SpkHex, sess.Username
}

func (m *Manager) cleanup() {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now()
	for id, sess := range m.sessions {
		if now.After(sess.Expiry) {
			delete(m.sessions, id)
		}
	}
}
