package event

import "sync"

// NOTE NOTE NOTE
// This was never used.
// NOTE NOTE NOTE

// Package event provides a way for listeners to subscribe to synchronous events.

// Listener receives events
// We use an interface instead of a function, because functions cannot be compared for equality.
// Comparison for equality is essential for removing an existing listener.
type Listener interface {
	OnEvent(sender *Sender, event any)
}

// Sender sends events
type Sender struct {
	listenersLock sync.Mutex
	listeners     []Listener
}

// Add a new listener
// If the listener is already present, then the function returns immediately
func (s *Sender) AddListener(listener Listener) {
	s.listenersLock.Lock()
	defer s.listenersLock.Unlock()
	for _, l := range s.listeners {
		if l == listener {
			return
		}
	}
	s.listeners = append(s.listeners, listener)
}

// Remove an existing listener
// If the listener is not present, then the function returns immediately
func (s *Sender) RemoveListener(listener Listener) {
	s.listenersLock.Lock()
	defer s.listenersLock.Unlock()
	for i, l := range s.listeners {
		if l == listener {
			s.listeners = append(s.listeners[:i], s.listeners[i+1:]...)
			return
		}
	}
}

// Send an event to all listeners
func (s *Sender) SendEvent(event any) {
	s.listenersLock.Lock()
	list := make([]Listener, len(s.listeners))
	copy(list, s.listeners)
	s.listenersLock.Unlock()

	for _, l := range list {
		l.OnEvent(s, event)
	}
}
