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

const ParentSessionName = "veriqid_parent_session"

// ParentSessionData holds per-parent session state.
type ParentSessionData struct {
	ParentID int64
	LoggedIn bool
	Expiry   time.Time
}

// ParentManager stores parent sessions server-side keyed by an HMAC'd cookie ID.
type ParentManager struct {
	mu       sync.RWMutex
	sessions map[string]*ParentSessionData
	secret   []byte
}

// NewParentManager creates a parent session manager with the given secret key.
func NewParentManager(secret []byte) *ParentManager {
	m := &ParentManager{
		sessions: make(map[string]*ParentSessionData),
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

func (m *ParentManager) generateID() string {
	b := make([]byte, 32)
	rand.Read(b)
	mac := hmac.New(sha256.New, m.secret)
	mac.Write(b)
	return base64.URLEncoding.EncodeToString(mac.Sum(nil))
}

// Create creates a new parent session and returns the session ID.
func (m *ParentManager) Create(parentID int64) string {
	sessionID := m.generateID()
	m.mu.Lock()
	m.sessions[sessionID] = &ParentSessionData{
		ParentID: parentID,
		LoggedIn: true,
		Expiry:   time.Now().Add(24 * time.Hour),
	}
	m.mu.Unlock()
	return sessionID
}

// Get retrieves the parent ID from a session ID. Returns (parentID, ok).
func (m *ParentManager) Get(sessionID string) (int64, bool) {
	m.mu.RLock()
	sess, ok := m.sessions[sessionID]
	m.mu.RUnlock()
	if !ok || !sess.LoggedIn || time.Now().After(sess.Expiry) {
		return 0, false
	}
	return sess.ParentID, true
}

// Login creates a parent session and sets the cookie.
func (m *ParentManager) Login(w http.ResponseWriter, parentID int64) string {
	sessionID := m.Create(parentID)
	http.SetCookie(w, &http.Cookie{
		Name:     ParentSessionName,
		Value:    sessionID,
		Path:     "/",
		MaxAge:   86400, // 24 hours
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
	return sessionID
}

// Logout clears the parent session and deletes the cookie.
func (m *ParentManager) Logout(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(ParentSessionName)
	if err == nil {
		m.mu.Lock()
		delete(m.sessions, cookie.Value)
		m.mu.Unlock()
	}
	http.SetCookie(w, &http.Cookie{
		Name:   ParentSessionName,
		Value:  "",
		Path:   "/",
		MaxAge: -1,
	})
}

// IsLoggedIn checks if the parent has an active session.
// Returns (loggedIn, parentID).
func (m *ParentManager) IsLoggedIn(r *http.Request) (bool, int64) {
	cookie, err := r.Cookie(ParentSessionName)
	if err != nil {
		return false, 0
	}
	parentID, ok := m.Get(cookie.Value)
	if !ok {
		return false, 0
	}
	return true, parentID
}

func (m *ParentManager) cleanup() {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now()
	for id, sess := range m.sessions {
		if now.After(sess.Expiry) {
			delete(m.sessions, id)
		}
	}
}
