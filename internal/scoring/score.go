package scoring

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"
)

const (
	StartingPoints  = 1000
	SessionFile     = ".bugbuster-session.json"
	LeaderboardFile = ".bugbuster-leaderboard.json"
)

// Session represents the current active session state.
type Session struct {
	Scenario   string      `json:"scenario"`
	StartTime  time.Time   `json:"start_time"`
	Points     int         `json:"points"`
	HintsUsed  []int       `json:"hints_used"`
	Active     bool        `json:"active"`
	Submission *Submission `json:"submission,omitempty"`
}

// Submission captures what the user submitted as their RCA.
type Submission struct {
	Category    string `json:"category"`
	Services    string `json:"services"`
	Description string `json:"description"`
	Fix         string `json:"fix"`
}

// LeaderboardEntry is one row in the local leaderboard.
type LeaderboardEntry struct {
	Scenario    string  `json:"scenario"`
	Score       int     `json:"score"`
	TimeMinutes float64 `json:"time_minutes"`
	Date        string  `json:"date"`
}

// NewSession creates and persists a fresh session.
func NewSession(projectRoot, scenario string) error {
	sess := &Session{
		Scenario:  scenario,
		StartTime: time.Now(),
		Points:    StartingPoints,
		HintsUsed: []int{},
		Active:    true,
	}
	return SaveSession(projectRoot, sess)
}

// LoadSession reads the current session from disk.
func LoadSession(projectRoot string) (*Session, error) {
	path := filepath.Join(projectRoot, SessionFile)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("no session file: %w", err)
	}
	var sess Session
	if err := json.Unmarshal(data, &sess); err != nil {
		return nil, fmt.Errorf("corrupt session file: %w", err)
	}
	return &sess, nil
}

// SaveSession persists session state to disk.
func SaveSession(projectRoot string, sess *Session) error {
	path := filepath.Join(projectRoot, SessionFile)
	data, err := json.MarshalIndent(sess, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// ElapsedMinutes returns how many minutes have passed since the session started.
func ElapsedMinutes(sess *Session) float64 {
	return time.Since(sess.StartTime).Minutes()
}

// RecordLeaderboard adds a solved scenario to the leaderboard, sorted by score descending.
func RecordLeaderboard(projectRoot, scenario string, score int, timeMinutes float64) error {
	entries, _ := LoadLeaderboard(projectRoot)

	entry := LeaderboardEntry{
		Scenario:    scenario,
		Score:       score,
		TimeMinutes: timeMinutes,
		Date:        time.Now().Format("2006-01-02 15:04"),
	}
	entries = append(entries, entry)

	// Sort by score descending
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Score > entries[j].Score
	})

	// Keep top 50
	if len(entries) > 50 {
		entries = entries[:50]
	}

	path := filepath.Join(projectRoot, LeaderboardFile)
	data, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// LoadLeaderboard reads the leaderboard from disk.
func LoadLeaderboard(projectRoot string) ([]LeaderboardEntry, error) {
	path := filepath.Join(projectRoot, LeaderboardFile)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var entries []LeaderboardEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, err
	}
	return entries, nil
}
