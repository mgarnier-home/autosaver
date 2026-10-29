package stats

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

type clock struct{ t time.Time }

func (c *clock) now() time.Time          { return c.t }
func (c *clock) advance(d time.Duration) { c.t = c.t.Add(d) }

func newClock() *clock {
	return &clock{t: time.Date(2026, 10, 29, 3, 0, 0, 0, time.UTC)}
}

func statePath(t *testing.T) string {
	return filepath.Join(t.TempDir(), "config-test.stats.json")
}

// run simule une sauvegarde complète d'une seconde par étape.
func run(s *Store, c *clock, err error) {
	s.Start()
	for _, step := range []string{"zip", "encrypt", "upload", "cleanup"} {
		s.Step(step)
		c.advance(time.Second)
	}
	s.Finish(err)
}

func TestRunRecordsStepsSizesAndProgress(t *testing.T) {
	c := newClock()
	s := Open(statePath(t), c.now)
	start := c.t

	s.Start()
	s.Step("zip")
	c.advance(10 * time.Second)
	s.Progress(50)

	snap := s.Snapshot()
	if !snap.Running || snap.Progress == nil || *snap.Progress != (Progress{Step: "zip", Percent: 50, StartedAt: start}) {
		t.Fatalf("progression pendant le zip : %+v", snap.Progress)
	}

	s.SetSourceBytes(1000)
	s.Step("encrypt")
	c.advance(5 * time.Second)
	s.SetArchiveBytes(600)
	s.Step("upload")
	c.advance(20 * time.Second)
	s.Step("cleanup")
	c.advance(time.Second)
	s.Finish(nil)

	snap = s.Snapshot()
	if snap.Running || snap.Progress != nil {
		t.Errorf("sauvegarde terminée mais running=%v progress=%+v", snap.Running, snap.Progress)
	}
	if len(snap.History) != 1 {
		t.Fatalf("%d exécutions, 1 attendue", len(snap.History))
	}
	want := Run{
		StartedAt:       start,
		FinishedAt:      c.t,
		Success:         true,
		DurationSeconds: 36,
		Steps:           map[string]int64{"zip": 10, "encrypt": 5, "upload": 20, "cleanup": 1},
		SourceBytes:     1000,
		ArchiveBytes:    600,
	}
	if !reflect.DeepEqual(snap.History[0], want) {
		t.Errorf("exécution %+v, attendu %+v", snap.History[0], want)
	}
	if snap.LastSuccessAt == nil || !snap.LastSuccessAt.Equal(c.t) || snap.ConsecutiveFailures != 0 {
		t.Errorf("lastSuccessAt=%v consecutiveFailures=%d", snap.LastSuccessAt, snap.ConsecutiveFailures)
	}
}

func TestFailuresAndLastSuccess(t *testing.T) {
	c := newClock()
	s := Open(statePath(t), c.now)

	run(s, c, nil)
	firstSuccess := c.t

	// Échec pendant le chiffrement : seules les étapes atteintes sont gardées.
	s.Start()
	s.Step("zip")
	c.advance(time.Second)
	s.Step("encrypt")
	c.advance(time.Second)
	s.Finish(errors.New("disque plein"))
	run(s, c, errors.New("boom"))

	snap := s.Snapshot()
	if snap.ConsecutiveFailures != 2 || !snap.LastSuccessAt.Equal(firstSuccess) {
		t.Errorf("consecutiveFailures=%d lastSuccessAt=%v", snap.ConsecutiveFailures, snap.LastSuccessAt)
	}
	failed := snap.History[1]
	if failed.Success || failed.Error != "disque plein" || !reflect.DeepEqual(failed.Steps, map[string]int64{"zip": 1, "encrypt": 1}) {
		t.Errorf("échec enregistré : %+v", failed)
	}

	run(s, c, nil)
	if snap := s.Snapshot(); snap.ConsecutiveFailures != 0 || !snap.LastSuccessAt.Equal(c.t) {
		t.Errorf("après un succès : consecutiveFailures=%d lastSuccessAt=%v", snap.ConsecutiveFailures, snap.LastSuccessAt)
	}
}

func TestHistoryKeepsLast30Runs(t *testing.T) {
	c := newClock()
	s := Open(statePath(t), c.now)

	var sixthStart time.Time
	for i := 1; i <= 35; i++ {
		if i == 6 {
			sixthStart = c.t
		}
		run(s, c, nil)
	}

	history := s.Snapshot().History
	if len(history) != 30 || !history[0].StartedAt.Equal(sixthStart) {
		t.Errorf("%d exécutions, la plus ancienne commence à %v (attendu %v)", len(history), history[0].StartedAt, sixthStart)
	}
}

func TestStateSurvivesRestart(t *testing.T) {
	c := newClock()
	path := statePath(t)
	s := Open(path, c.now)
	run(s, c, nil)
	run(s, c, errors.New("boom"))

	if got, want := Open(path, c.now).Snapshot(), s.Snapshot(); !reflect.DeepEqual(got, want) {
		t.Errorf("après redémarrage %+v, attendu %+v", got, want)
	}
}

func TestOpenWithoutUsableFileStartsEmpty(t *testing.T) {
	c := newClock()
	corrupt := statePath(t)
	if err := os.WriteFile(corrupt, []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}

	for name, path := range map[string]string{"absent": statePath(t), "corrompu": corrupt} {
		snap := Open(path, c.now).Snapshot()
		if snap.Running || snap.LastSuccessAt != nil || snap.ConsecutiveFailures != 0 || snap.History == nil || len(snap.History) != 0 {
			t.Errorf("fichier %s : %+v", name, snap)
		}
	}
}

func TestInterruptedRunIsRecordedAtRestart(t *testing.T) {
	c := newClock()
	path := statePath(t)
	s := Open(path, c.now)
	run(s, c, nil)

	start := c.t
	s.Start()
	s.Step("upload")
	c.advance(time.Hour) // le conteneur redémarre pendant l'envoi

	snap := Open(path, c.now).Snapshot()
	if len(snap.History) != 2 || snap.ConsecutiveFailures != 1 || snap.Running {
		t.Fatalf("après redémarrage : %+v", snap)
	}
	interrupted := snap.History[1]
	if interrupted.Success || interrupted.Error != "interrompue par un redémarrage" ||
		!interrupted.StartedAt.Equal(start) || !interrupted.FinishedAt.Equal(c.t) || interrupted.DurationSeconds != 3600 {
		t.Errorf("exécution interrompue : %+v", interrupted)
	}

	// L'interruption n'est comptée qu'une fois.
	if again := Open(path, c.now).Snapshot(); len(again.History) != 2 || again.ConsecutiveFailures != 1 {
		t.Errorf("second redémarrage : %+v", again)
	}
}
