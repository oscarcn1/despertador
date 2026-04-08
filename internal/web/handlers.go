package web

import (
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"strings"

	"despertador/internal/alarm"
)

type Server struct {
	config    *alarm.Config
	scheduler *alarm.Scheduler
	templates *template.Template
}

type AlarmAPI struct {
	ID           string          `json:"id,omitempty"`
	Name         string          `json:"name"`
	Enabled      bool            `json:"enabled"`
	Hour         int             `json:"hour"`
	Minute       int             `json:"minute"`
	Period       string          `json:"period"`
	Days         []alarm.Weekday `json:"days"`
	MusicDir     string          `json:"music_dir"`
	Volume       int             `json:"volume"`
	PlayOrder    string          `json:"play_order"`
	SelectedFile string          `json:"selected_file,omitempty"`
}

type StatusResponse struct {
	Alarms      []AlarmAPI `json:"alarms"`
	Ringing     bool       `json:"ringing"`
	RingingInfo *struct {
		AlarmID   string `json:"alarm_id"`
		AlarmName string `json:"alarm_name"`
		Since     string `json:"since"`
	} `json:"ringing_info,omitempty"`
}

func NewServer(config *alarm.Config, scheduler *alarm.Scheduler) *Server {
	tmpl := template.Must(template.ParseGlob("web/templates/*.html"))
	return &Server{
		config:    config,
		scheduler: scheduler,
		templates: tmpl,
	}
}

func (s *Server) SetupRoutes() http.Handler {
	mux := http.NewServeMux()

	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("web/static"))))
	mux.HandleFunc("/", s.handleIndex)
	mux.HandleFunc("/api/status", s.handleStatus)
	mux.HandleFunc("/api/alarms", s.handleAlarms)
	mux.HandleFunc("/api/alarms/", s.handleAlarmByID)
	mux.HandleFunc("/api/dismiss", s.handleDismiss)
	mux.HandleFunc("/api/test/", s.handleTest)
	mux.HandleFunc("/api/music-files", s.handleMusicFiles)

	return mux
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	s.templates.ExecuteTemplate(w, "index.html", nil)
}

func entryToAPI(a alarm.AlarmEntry) AlarmAPI {
	return AlarmAPI{
		ID:           a.ID,
		Name:         a.Name,
		Enabled:      a.Enabled,
		Hour:         a.Hour,
		Minute:       a.Minute,
		Period:       a.Period,
		Days:         a.Days,
		MusicDir:     a.MusicDir,
		Volume:       a.Volume,
		PlayOrder:    a.PlayOrder,
		SelectedFile: a.SelectedFile,
	}
}

func apiToEntry(a AlarmAPI) alarm.AlarmEntry {
	return alarm.AlarmEntry{
		ID:           a.ID,
		Name:         a.Name,
		Enabled:      a.Enabled,
		Hour:         a.Hour,
		Minute:       a.Minute,
		Period:       a.Period,
		Days:         a.Days,
		MusicDir:     a.MusicDir,
		Volume:       a.Volume,
		PlayOrder:    a.PlayOrder,
		SelectedFile: a.SelectedFile,
	}
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	alarms := s.config.GetAlarms()
	apiAlarms := make([]AlarmAPI, len(alarms))
	for i, a := range alarms {
		apiAlarms[i] = entryToAPI(a)
	}

	resp := StatusResponse{
		Alarms:  apiAlarms,
		Ringing: s.scheduler.IsRinging(),
	}

	if info := s.scheduler.GetRingingInfo(); info != nil {
		resp.RingingInfo = &struct {
			AlarmID   string `json:"alarm_id"`
			AlarmName string `json:"alarm_name"`
			Since     string `json:"since"`
		}{
			AlarmID:   info.AlarmID,
			AlarmName: info.AlarmName,
			Since:     info.Since.Format("03:04:05 PM"),
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// POST /api/alarms - create new alarm
func (s *Server) handleAlarms(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req AlarmAPI
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if err := validateAlarm(req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	entry, err := s.config.AddAlarm(apiToEntry(req))
	if err != nil {
		log.Printf("Error adding alarm: %v", err)
		http.Error(w, "Error saving alarm", http.StatusInternalServerError)
		return
	}

	log.Printf("Alarm added: [%s] %s at %d:%02d %s", entry.ID, entry.Name, entry.Hour, entry.Minute, entry.Period)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(entryToAPI(entry))
}

// PUT /api/alarms/{id} - update, DELETE /api/alarms/{id} - delete
func (s *Server) handleAlarmByID(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/alarms/")
	if id == "" {
		http.Error(w, "Missing alarm ID", http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodPut:
		var req AlarmAPI
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}
		if err := validateAlarm(req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		req.ID = id
		entry := apiToEntry(req)
		if err := s.config.UpdateAlarm(entry); err != nil {
			http.Error(w, "Alarm not found", http.StatusNotFound)
			return
		}

		log.Printf("Alarm updated: [%s] %s at %d:%02d %s", id, req.Name, req.Hour, req.Minute, req.Period)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(entryToAPI(entry))

	case http.MethodDelete:
		if err := s.config.DeleteAlarm(id); err != nil {
			http.Error(w, "Alarm not found", http.StatusNotFound)
			return
		}
		log.Printf("Alarm deleted: %s", id)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "deleted"})

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleDismiss(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	s.scheduler.Dismiss()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "dismissed"})
}

func (s *Server) handleTest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/api/test/")
	entry, ok := s.config.GetAlarm(id)
	if !ok {
		http.Error(w, "Alarm not found", http.StatusNotFound)
		return
	}
	go s.scheduler.TestAlarm(entry)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "testing"})
}

func (s *Server) handleMusicFiles(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	dir := r.URL.Query().Get("dir")
	if dir == "" {
		dir = "/home/oscar/Projects/despertador/music"
	}

	files, _ := alarm.ListMP3Files(dir)
	if files == nil {
		files = []string{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(files)
}

func validateAlarm(a AlarmAPI) error {
	if a.Hour < 1 || a.Hour > 12 {
		return fmt.Errorf("hour must be 1-12")
	}
	if a.Minute < 0 || a.Minute > 59 {
		return fmt.Errorf("minute must be 0-59")
	}
	if a.Period != "AM" && a.Period != "PM" {
		return fmt.Errorf("period must be AM or PM")
	}
	if a.Volume < 0 || a.Volume > 100 {
		return fmt.Errorf("volume must be 0-100")
	}
	if a.PlayOrder != "random" && a.PlayOrder != "sequential" && a.PlayOrder != "single" {
		return fmt.Errorf("play_order must be random, sequential, or single")
	}
	return nil
}
