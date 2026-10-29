// Package stats tient l'historique des sauvegardes, servi par /stats et conservé dans un fichier
// d'état pour survivre aux redémarrages.
package stats

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/mgarnier-home/utils/logger"
)

const maxHistory = 30

type Progress struct {
	Step      string    `json:"step"`
	Percent   float64   `json:"percent"`
	StartedAt time.Time `json:"startedAt"`
}

type Run struct {
	StartedAt       time.Time        `json:"startedAt"`
	FinishedAt      time.Time        `json:"finishedAt"`
	Success         bool             `json:"success"`
	Error           string           `json:"error"`
	DurationSeconds int64            `json:"durationSeconds"`
	Steps           map[string]int64 `json:"steps"`
	SourceBytes     int64            `json:"sourceBytes"`
	ArchiveBytes    int64            `json:"archiveBytes"`
}

type Snapshot struct {
	Running             bool       `json:"running"`
	Progress            *Progress  `json:"progress"`
	LastSuccessAt       *time.Time `json:"lastSuccessAt"`
	ConsecutiveFailures int        `json:"consecutiveFailures"`
	History             []Run      `json:"history"`
}

// state est ce qui est écrit dans le fichier ; la progression n'y figure pas.
type state struct {
	LastSuccessAt       *time.Time `json:"lastSuccessAt"`
	ConsecutiveFailures int        `json:"consecutiveFailures"`
	History             []Run      `json:"history"`
	CurrentStartedAt    *time.Time `json:"currentStartedAt,omitempty"`
}

type Store struct {
	mu    sync.Mutex
	path  string
	now   func() time.Time
	state state

	// Exécution en cours, nil hors sauvegarde.
	current       *Run
	step          string
	stepStartedAt time.Time
	percent       float64
}

// Open charge l'état depuis path. Un fichier absent ou illisible donne un état vide : les stats ne
// doivent jamais empêcher le service de démarrer.
func Open(path string, now func() time.Time) *Store {
	s := &Store{path: path, now: now}

	data, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		logger.Errorf("Failed to read stats file %s: %v", path, err)
	} else if err == nil {
		if err := json.Unmarshal(data, &s.state); err != nil {
			logger.Errorf("Ignoring unreadable stats file %s: %v", path, err)
			s.state = state{}
		}
	}

	if started := s.state.CurrentStartedAt; started != nil {
		finished := now().UTC().Truncate(time.Second)
		s.record(Run{
			StartedAt:       *started,
			FinishedAt:      finished,
			Error:           "interrompue par un redémarrage",
			DurationSeconds: seconds(finished.Sub(*started)),
		})
		s.save()
	}

	return s
}

func (s *Store) Start() {
	s.mu.Lock()
	defer s.mu.Unlock()

	started := s.now().UTC().Truncate(time.Second)
	s.current = &Run{StartedAt: started, Steps: map[string]int64{}}
	s.step = ""
	s.state.CurrentStartedAt = &started
	s.save()
}

// Step termine l'étape en cours et démarre la suivante.
func (s *Store) Step(name string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.current == nil {
		return
	}
	s.closeStep()
	s.step, s.stepStartedAt, s.percent = name, s.now(), 0
}

func (s *Store) Progress(percent float64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.percent = percent
}

func (s *Store) SetSourceBytes(n int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.current != nil {
		s.current.SourceBytes = n
	}
}

func (s *Store) SetArchiveBytes(n int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.current != nil {
		s.current.ArchiveBytes = n
	}
}

func (s *Store) Finish(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.current == nil {
		return
	}
	s.closeStep()

	run := *s.current
	run.FinishedAt = s.now().UTC().Truncate(time.Second)
	run.DurationSeconds = seconds(run.FinishedAt.Sub(run.StartedAt))
	run.Success = err == nil
	if err != nil {
		run.Error = err.Error()
	}

	s.current = nil
	s.state.CurrentStartedAt = nil
	s.record(run)
	s.save()
}

func (s *Store) Snapshot() Snapshot {
	s.mu.Lock()
	defer s.mu.Unlock()

	snap := Snapshot{
		Running:             s.current != nil,
		LastSuccessAt:       s.state.LastSuccessAt,
		ConsecutiveFailures: s.state.ConsecutiveFailures,
		History:             append([]Run{}, s.state.History...),
	}
	if s.current != nil {
		snap.Progress = &Progress{Step: s.step, Percent: s.percent, StartedAt: s.current.StartedAt}
	}
	return snap
}

func (s *Store) closeStep() {
	if s.step != "" {
		s.current.Steps[s.step] = seconds(s.now().Sub(s.stepStartedAt))
	}
}

func (s *Store) record(run Run) {
	if run.Success {
		finished := run.FinishedAt
		s.state.LastSuccessAt = &finished
		s.state.ConsecutiveFailures = 0
	} else {
		s.state.ConsecutiveFailures++
	}
	s.state.CurrentStartedAt = nil
	s.state.History = append(s.state.History, run)
	if len(s.state.History) > maxHistory {
		s.state.History = s.state.History[len(s.state.History)-maxHistory:]
	}
}

// save écrit dans un fichier temporaire puis le renomme : un arrêt brutal ne laisse pas de JSON tronqué.
func (s *Store) save() {
	data, err := json.Marshal(s.state)
	if err == nil {
		tmp := s.path + ".tmp"
		if err = os.WriteFile(tmp, data, 0o644); err == nil {
			err = os.Rename(tmp, s.path)
		}
	}
	if err != nil {
		logger.Errorf("Failed to write stats file %s: %v", filepath.Base(s.path), err)
	}
}

func seconds(d time.Duration) int64 {
	return int64(d.Round(time.Second) / time.Second)
}
