package update

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// CheckInterval is how often the latest release is looked up, and also how
// often the "newer version available" notice may be shown.
const CheckInterval = 24 * time.Hour

// State is the on-disk cache for the background version check.
type State struct {
	CheckedAt  time.Time `json:"checked_at"`
	Latest     string    `json:"latest,omitempty"`
	NotifiedAt time.Time `json:"notified_at"`
}

// StatePath is the cache file under the OS config directory.
func StatePath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "homeyctl", "update-check.json"), nil
}

func loadState(path string) State {
	var s State
	data, err := os.ReadFile(path)
	if err == nil {
		_ = json.Unmarshal(data, &s)
	}
	return s
}

func saveState(path string, s State) {
	data, err := json.Marshal(s)
	if err != nil {
		return
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return
	}
	_ = os.WriteFile(path, data, 0o600)
}

// Pending is a version check started alongside a command. The lookup runs in
// the background so it never delays the command; Finish collects it.
type Pending struct {
	path    string
	current string
	now     time.Time
	state   State
	result  chan string // latest tag, or "" when the lookup failed
}

// StartCheck loads the cache and, when it is older than CheckInterval, looks
// up the latest release in the background. It never returns an error: an
// update check must not be able to break a command.
func StartCheck(client *Client, path, current string, now time.Time) *Pending {
	p := &Pending{path: path, current: current, now: now, state: loadState(path)}
	if now.Sub(p.state.CheckedAt) < CheckInterval {
		return p
	}
	p.result = make(chan string, 1)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		tag, err := client.LatestTag(ctx)
		if err != nil {
			tag = ""
		}
		p.result <- tag
	}()
	return p
}

// Finish waits at most wait for a lookup still in flight, persists what it
// learned, and returns a one-line notice when a newer release exists and the
// user has not been told in the last CheckInterval. The notice is empty
// otherwise.
func (p *Pending) Finish(wait time.Duration) string {
	if p.result != nil {
		select {
		case tag := <-p.result:
			// A failed lookup still counts as a check, so an offline machine
			// does not retry on every single command.
			p.state.CheckedAt = p.now
			if tag != "" {
				p.state.Latest = tag
			}
			saveState(p.path, p.state)
		case <-time.After(wait):
			// Too slow this time; the next command retries.
		}
	}

	if !Newer(p.state.Latest, p.current) || p.now.Sub(p.state.NotifiedAt) < CheckInterval {
		return ""
	}
	p.state.NotifiedAt = p.now
	saveState(p.path, p.state)
	return fmt.Sprintf("homeyctl %s is available (you have %s). Run: homeyctl upgrade",
		p.state.Latest, NormalizeTag(p.current))
}
