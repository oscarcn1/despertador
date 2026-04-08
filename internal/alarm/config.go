package alarm

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
)

type Weekday int

const (
	Sunday    Weekday = 0
	Monday    Weekday = 1
	Tuesday   Weekday = 2
	Wednesday Weekday = 3
	Thursday  Weekday = 4
	Friday    Weekday = 5
	Saturday  Weekday = 6
)

type AlarmEntry struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	Enabled      bool      `json:"enabled"`
	Hour         int       `json:"hour"`
	Minute       int       `json:"minute"`
	Period       string    `json:"period"`
	Days         []Weekday `json:"days"`
	MusicDir     string    `json:"music_dir"`
	Volume       int       `json:"volume"`
	PlayOrder    string    `json:"play_order"`
	SelectedFile string    `json:"selected_file,omitempty"`
}

// Hour24 converts 12h AM/PM to 24h format for scheduling comparison.
func (a *AlarmEntry) Hour24() int {
	h := a.Hour
	if a.Period == "AM" {
		if h == 12 {
			return 0
		}
		return h
	}
	// PM
	if h == 12 {
		return 12
	}
	return h + 12
}

func (a *AlarmEntry) TimeString() string {
	return fmt.Sprintf("%d:%02d %s", a.Hour, a.Minute, a.Period)
}

type Config struct {
	mu       sync.RWMutex
	filePath string
	nextID   int

	Alarms []AlarmEntry `json:"alarms"`
}

func NewConfig(filePath string) *Config {
	return &Config{
		filePath: filePath,
		nextID:   1,
		Alarms: []AlarmEntry{
			{
				ID:        "1",
				Name:      "Alarma principal",
				Enabled:   true,
				Hour:      7,
				Minute:    0,
				Period:    "AM",
				Days:      []Weekday{Monday, Tuesday, Wednesday, Thursday, Friday},
				MusicDir:  "/home/oscar/Projects/despertador/music",
				Volume:    80,
				PlayOrder: "random",
			},
		},
	}
}

func (c *Config) Load() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	data, err := os.ReadFile(c.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return c.saveLocked()
		}
		return err
	}
	if err := json.Unmarshal(data, c); err != nil {
		return err
	}

	// Compute nextID from existing alarms
	for _, a := range c.Alarms {
		var id int
		if _, err := fmt.Sscanf(a.ID, "%d", &id); err == nil && id >= c.nextID {
			c.nextID = id + 1
		}
	}
	return nil
}

func (c *Config) Save() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.saveLocked()
}

func (c *Config) saveLocked() error {
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(c.filePath, data, 0644)
}

func (c *Config) GetAlarms() []AlarmEntry {
	c.mu.RLock()
	defer c.mu.RUnlock()
	result := make([]AlarmEntry, len(c.Alarms))
	copy(result, c.Alarms)
	return result
}

func (c *Config) GetAlarm(id string) (AlarmEntry, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	for _, a := range c.Alarms {
		if a.ID == id {
			return a, true
		}
	}
	return AlarmEntry{}, false
}

func (c *Config) AddAlarm(entry AlarmEntry) (AlarmEntry, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	entry.ID = fmt.Sprintf("%d", c.nextID)
	c.nextID++
	c.Alarms = append(c.Alarms, entry)
	return entry, c.saveLocked()
}

func (c *Config) UpdateAlarm(entry AlarmEntry) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	for i, a := range c.Alarms {
		if a.ID == entry.ID {
			c.Alarms[i] = entry
			return c.saveLocked()
		}
	}
	return fmt.Errorf("alarm %s not found", entry.ID)
}

func (c *Config) DeleteAlarm(id string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	for i, a := range c.Alarms {
		if a.ID == id {
			c.Alarms = append(c.Alarms[:i], c.Alarms[i+1:]...)
			return c.saveLocked()
		}
	}
	return fmt.Errorf("alarm %s not found", id)
}
