package backup

import (
	"fmt"
	"os"

	"github.com/ProtonMail/gopenpgp/v3/crypto"
	"github.com/mgarnier-home/utils/common"
	"github.com/mgarnier-home/utils/logger"
)

func encryptFile(zipFile, outputFile string) (string, error) {
	password, err := common.GenerateRandomString(20)

	if err != nil {
		return "", fmt.Errorf("failed to generate password: %w", err)
	}

	logger.Infof("Encrypting backup with password: %s", password)

	encryptPercent, lastEncryptPercent := 0.0, 0.0

	err = encryptFileWithPasswordWithProgress(
		zipFile,
		outputFile,
		password,
		func(written int, totalWritten int64, totalSize int64) {
			encryptPercent = float64(totalWritten) / float64(totalSize) * 100
			if encryptPercent-lastEncryptPercent > 1 {
				lastEncryptPercent = encryptPercent
				logger.Infof("Encrypting backup: %d", int(encryptPercent))
			}
		})

	if err != nil {
		return "", fmt.Errorf("failed to encrypt backup: %w", err)
	}

	logger.Infof("Successfully encrypted backup")

	return password, nil
}

func encryptFileWithPasswordWithProgress(inputPath, outputPath, password string, progressCallback func(int, int64, int64)) error {
	messageReader, err := os.Open(inputPath)
	if err != nil {
		return err
	}
	defer messageReader.Close()

	ciphertextWriter, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer ciphertextWriter.Close()

	passwordBytes := []byte(password)
	pgp := crypto.PGP()

	encHandle, err := pgp.Encryption().Password(passwordBytes).New()
	if err != nil {
		return err
	}
	writer, err := encHandle.EncryptingWriter(ciphertextWriter, crypto.Auto)
	if err != nil {
		return err
	}
	defer writer.Close()

	// Get the size of the input file for progress calculation
	fileInfo, err := messageReader.Stat()
	if err != nil {
		return err
	}
	totalSize := fileInfo.Size()

	common.CopyWithProgress(writer, messageReader, func(written int, total int64) {
		if progressCallback != nil {
			progressCallback(written, total, totalSize)
		}
	})

	return nil
}
