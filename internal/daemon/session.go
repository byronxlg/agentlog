package daemon

import "github.com/google/uuid"

func (d *Daemon) createSession() string {
	id := uuid.New().String()
	d.mu.Lock()
	d.sessions[id] = true
	d.mu.Unlock()
	return id
}

func (d *Daemon) listActiveSessions() []string {
	d.mu.RLock()
	defer d.mu.RUnlock()
	result := make([]string, 0, len(d.sessions))
	for id := range d.sessions {
		result = append(result, id)
	}
	return result
}
