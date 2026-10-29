package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"mgarnier11.fr/go/autosaver/backup"
	"mgarnier11.fr/go/autosaver/stats"
)

func TestStatsHandler(t *testing.T) {
	backup.Stats = stats.Open(filepath.Join(t.TempDir(), "s.stats.json"), time.Now)
	backup.Stats.Start()
	backup.Stats.Step("zip")
	backup.Stats.Progress(12.5)

	rec := httptest.NewRecorder()
	statsHandler(rec, httptest.NewRequest(http.MethodGet, "/stats", nil))

	if rec.Code != http.StatusOK || rec.Header().Get("Content-Type") != "application/json" {
		t.Fatalf("réponse %d, Content-Type %q", rec.Code, rec.Header().Get("Content-Type"))
	}
	var got map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("JSON invalide : %v", err)
	}
	progress, _ := got["progress"].(map[string]any)
	if got["running"] != true || progress["step"] != "zip" || progress["percent"] != 12.5 {
		t.Errorf("réponse %s", rec.Body.String())
	}
	// Champs attendus par homepage même sans succès ni historique.
	for _, key := range []string{"lastSuccessAt", "consecutiveFailures", "history"} {
		if _, ok := got[key]; !ok {
			t.Errorf("champ %s absent : %s", key, rec.Body.String())
		}
	}
	if history, ok := got["history"].([]any); !ok || len(history) != 0 {
		t.Errorf("history doit être une liste vide, obtenu %v", got["history"])
	}
}
