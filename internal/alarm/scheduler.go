package alarm

import (
	"log"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"despertador/internal/player"
)

type RingingInfo struct {
	AlarmID   string
	AlarmName string
	Since     time.Time
}

type Scheduler struct {
	config  *Config
	player  *player.Player
	stopCh  chan struct{}
	mu      sync.RWMutex
	ringing *RingingInfo
	// Track which alarms already fired this minute to avoid re-triggering
	firedAt map[string]string // alarmID -> "HH:MM"
	// Track sequential playback index per alarm
	seqIndex map[string]int
}

func NewScheduler(config *Config, p *player.Player) *Scheduler {
	return &Scheduler{
		config:   config,
		player:   p,
		stopCh:   make(chan struct{}),
		firedAt:  make(map[string]string),
		seqIndex: make(map[string]int),
	}
}

func (s *Scheduler) Start() {
	log.Println("Alarm scheduler started")
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.check()
		case <-s.stopCh:
			return
		}
	}
}

func (s *Scheduler) Stop() {
	close(s.stopCh)
}

func (s *Scheduler) IsRinging() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.ringing != nil
}

func (s *Scheduler) GetRingingInfo() *RingingInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.ringing
}

func (s *Scheduler) Dismiss() {
	s.player.Stop()
	s.mu.Lock()
	s.ringing = nil
	s.mu.Unlock()
	log.Println("Alarm dismissed")
}

func (s *Scheduler) TestAlarm(entry AlarmEntry) {
	s.trigger(entry)
}

func (s *Scheduler) check() {
	s.mu.RLock()
	isRinging := s.ringing != nil
	s.mu.RUnlock()
	if isRinging {
		return
	}

	now := time.Now()
	currentDay := Weekday(now.Weekday())
	currentTime := now.Format("15:04")

	// Clean old firedAt entries
	for id, t := range s.firedAt {
		if t != currentTime {
			delete(s.firedAt, id)
		}
	}

	alarms := s.config.GetAlarms()
	for _, a := range alarms {
		if !a.Enabled {
			continue
		}

		// Already fired this minute
		if s.firedAt[a.ID] == currentTime {
			continue
		}

		dayMatch := false
		for _, d := range a.Days {
			if d == currentDay {
				dayMatch = true
				break
			}
		}
		if !dayMatch {
			continue
		}

		if now.Hour() == a.Hour24() && now.Minute() == a.Minute {
			s.firedAt[a.ID] = currentTime
			s.trigger(a)
			return
		}
	}
}

func (s *Scheduler) trigger(a AlarmEntry) {
	musicFile, err := s.pickMusic(a)
	if err != nil {
		log.Printf("Error selecting music for alarm %s: %v", a.ID, err)
		return
	}

	log.Printf("ALARM [%s] %s! Playing: %s", a.ID, a.Name, musicFile)

	s.mu.Lock()
	s.ringing = &RingingInfo{
		AlarmID:   a.ID,
		AlarmName: a.Name,
		Since:     time.Now(),
	}
	s.mu.Unlock()

	if err := s.player.Play(musicFile, a.Volume); err != nil {
		log.Printf("Error playing alarm: %v", err)
		s.mu.Lock()
		s.ringing = nil
		s.mu.Unlock()
	}
}

func (s *Scheduler) pickMusic(a AlarmEntry) (string, error) {
	if a.PlayOrder == "single" && a.SelectedFile != "" {
		if _, err := os.Stat(a.SelectedFile); err == nil {
			return a.SelectedFile, nil
		}
	}

	files, err := listMP3(a.MusicDir)
	if err != nil {
		return "", err
	}
	if len(files) == 0 {
		return "", os.ErrNotExist
	}

	switch a.PlayOrder {
	case "sequential":
		idx := s.seqIndex[a.ID] % len(files)
		s.seqIndex[a.ID] = idx + 1
		return files[idx], nil
	default: // "random"
		return files[rand.Intn(len(files))], nil
	}
}

func listMP3(dir string) ([]string, error) {
	var files []string
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.HasSuffix(strings.ToLower(info.Name()), ".mp3") {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(files)
	return files, nil
}

// ListMP3Files is exported for use by the web handlers.
func ListMP3Files(dir string) ([]string, error) {
	return listMP3(dir)
}
