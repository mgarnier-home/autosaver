package external

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mgarnier-home/utils/logger"
	"mgarnier11.fr/go/autosaver/config"
)

func TestMain(m *testing.M) {
	logger.InitAppLogger("TEST")
	os.Exit(m.Run())
}

func TestDeleteOldFolders(t *testing.T) {
	config.Config = &config.AppEnvConfig{AppConfig: &config.AppConfigFile{KeepDuration: 14}}

	now := time.Now()
	day := func(t time.Time) string { return t.Format("2006-01-02") }
	// Premier du mois situé entre 60 et 90 jours dans le passé : au-delà de KeepDuration, en deçà d'un an.
	twoMonthsAgo := now.AddDate(0, 0, -60)
	firstOfMonth := time.Date(twoMonthsAgo.Year(), twoMonthsAgo.Month(), 1, 0, 0, 0, 0, time.UTC)

	dirs := map[string]bool{ // nom -> doit être conservé
		day(now.AddDate(0, 0, -3)):         true,  // récent
		day(firstOfMonth):                  true,  // mensuelle de moins d'un an
		day(firstOfMonth.AddDate(0, 0, 1)): false, // trop vieille, pas un premier du mois
		day(time.Date(now.Year()-2, now.Month(), 1, 0, 0, 0, 0, time.UTC)): false, // mensuelle de plus d'un an
		"not-a-date": true,
	}

	root := t.TempDir()
	for name := range dirs {
		if err := os.Mkdir(filepath.Join(root, name), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	// Un fichier au nom de date ancienne n'est pas un dossier de sauvegarde : il reste.
	oldFile := day(now.AddDate(-2, 0, 2))
	if err := os.WriteFile(filepath.Join(root, oldFile), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	dirs[oldFile] = true

	if err := deleteOldFolders(osReadDir, os.RemoveAll, root); err != nil {
		t.Fatalf("deleteOldFolders: %v", err)
	}

	for name, keep := range dirs {
		_, err := os.Stat(filepath.Join(root, name))
		if exists := err == nil; exists != keep {
			t.Errorf("%s : présent=%v, attendu=%v", name, exists, keep)
		}
	}
}

func TestDeleteOldFoldersMissingDir(t *testing.T) {
	config.Config = &config.AppEnvConfig{AppConfig: &config.AppConfigFile{KeepDuration: 14}}

	if err := deleteOldFolders(osReadDir, os.RemoveAll, filepath.Join(t.TempDir(), "absent")); err == nil {
		t.Fatal("erreur attendue pour un dossier inexistant")
	}
}

func osReadDir(path string) ([]os.FileInfo, error) {
	//function to cast os.ReadDir to os.FileInfo
	// os.ReadDir returns a slice of os.DirEntry, we need to convert it to []os.FileInfo
	dirs, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}

	var fileInfos []os.FileInfo
	for _, dir := range dirs {
		info, err := dir.Info()
		if err != nil {
			return nil, err
		}
		fileInfos = append(fileInfos, info)
	}
	return fileInfos, nil
}
