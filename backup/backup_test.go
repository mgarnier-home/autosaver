package backup

import (
	"archive/zip"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ProtonMail/gopenpgp/v3/crypto"
	"github.com/mgarnier-home/utils/logger"
	"mgarnier11.fr/go/autosaver/stats"
)

func TestMain(m *testing.M) {
	logger.InitAppLogger("TEST")
	os.Exit(m.Run())
}

// Une sauvegarde n'a de valeur que si on sait la relire : zip, chiffrement, puis l'inverse.
func TestZipEncryptRoundTrip(t *testing.T) {
	src := t.TempDir()
	files := map[string]string{
		"a.txt":            "contenu a",
		"sub/b.txt":        "contenu b",
		"sub/deep/c.bin":   string([]byte{0, 1, 2, 255}),
		"sub/empty-file":   "",
		"other/large.data": string(make([]byte, 1<<20)),
	}
	for name, content := range files {
		path := filepath.Join(src, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink(filepath.Join(src, "a.txt"), filepath.Join(src, "link")); err != nil {
		t.Fatal(err)
	}

	out := t.TempDir()
	zipPath := filepath.Join(out, "backup.zip")
	gpgPath := zipPath + ".gpg"

	rec := stats.Open(filepath.Join(out, "s.stats.json"), time.Now)
	sourceBytes, err := zipFolder(src, zipPath, rec)
	if err != nil {
		t.Fatalf("zipFolder: %v", err)
	}
	contentBytes := 0
	for _, content := range files {
		contentBytes += len(content)
	}
	if sourceBytes < int64(contentBytes) {
		t.Errorf("taille source %d, au moins %d attendus", sourceBytes, contentBytes)
	}
	password, err := encryptFile(zipPath, gpgPath, rec)
	if err != nil {
		t.Fatalf("encryptFile: %v", err)
	}

	encrypted, err := os.ReadFile(gpgPath)
	if err != nil {
		t.Fatal(err)
	}
	dec, err := crypto.PGP().Decryption().Password([]byte(password)).New()
	if err != nil {
		t.Fatal(err)
	}
	decrypted, err := dec.Decrypt(encrypted, crypto.Auto)
	if err != nil {
		t.Fatalf("déchiffrement: %v", err)
	}

	original, err := os.ReadFile(zipPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(decrypted.Bytes()) != string(original) {
		t.Fatal("le fichier déchiffré diffère du zip d'origine")
	}

	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		t.Fatal(err)
	}
	defer zr.Close()

	got := map[string]string{}
	for _, f := range zr.File {
		if f.FileInfo().IsDir() {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		content, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			t.Fatal(err)
		}
		got[f.Name] = string(content)
	}

	if _, ok := got["link"]; ok {
		t.Error("le lien symbolique ne doit pas être archivé")
	}
	for name, want := range files {
		if got[name] != want {
			t.Errorf("%s : contenu différent après dézippage", name)
		}
	}
	if len(got) != len(files) {
		t.Errorf("%d fichiers dans le zip, %d attendus", len(got), len(files))
	}
}

func TestEncryptFileUsesFreshPassword(t *testing.T) {
	dir := t.TempDir()
	in := filepath.Join(dir, "in")
	if err := os.WriteFile(in, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	rec := stats.Open(filepath.Join(dir, "s.stats.json"), time.Now)
	p1, err := encryptFile(in, filepath.Join(dir, "out1"), rec)
	if err != nil {
		t.Fatal(err)
	}
	p2, err := encryptFile(in, filepath.Join(dir, "out2"), rec)
	if err != nil {
		t.Fatal(err)
	}
	if p1 == "" || p1 == p2 {
		t.Errorf("mots de passe attendus non vides et différents, obtenus %q et %q", p1, p2)
	}
}
