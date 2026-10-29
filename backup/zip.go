package backup

import (
	"archive/zip"
	"fmt"
	"math"
	"os"
	"path/filepath"

	"github.com/mgarnier-home/utils/common"
	"github.com/mgarnier-home/utils/logger"
)

func zipFolder(backupSrc string, fileName string) error {
	filePercent, lastFilePercent := 0.0, 0.0
	totalPercent, lastTotalPercent := 0.0, 0.0

	logger.Infof("Zipping folder %s", backupSrc)

	err := zipFolderWithProgress(
		backupSrc,
		fileName,
		func(
			fileName string,
			written int,
			fileWritten,
			fileSize,
			totalWritten,
			totalSize int64,
		) {
			filePercent = float64(fileWritten) / float64(fileSize) * 100
			totalPercent = float64(totalWritten) / float64(totalSize) * 100

			if math.Abs(filePercent-lastFilePercent) > 1 {
				lastFilePercent = filePercent
				logger.Debugf("Zipping file %s: %d", fileName, int(filePercent))
			}

			if totalPercent-lastTotalPercent > 1 {
				lastTotalPercent = totalPercent
				logger.Infof("Zipping folder: %d", int(totalPercent))
			}

		})

	if err != nil {
		return fmt.Errorf("failed to zip folder: %w", err)
	}

	logger.Infof("Successfully zipped folder")

	return nil
}

func zipFolderWithProgress(
	folderPath,
	zipPath string,
	progressFunc func(
		fileName string,
		written int,
		fileWritten int64,
		fileSize int64,
		totalWritten int64,
		totalSize int64,
	),
) error {
	zipFile, err := os.Create(zipPath)
	if err != nil {
		return fmt.Errorf("failed to create zip file: %w", err)
	}
	defer zipFile.Close()

	zipWriter := zip.NewWriter(zipFile)
	defer zipWriter.Close()

	totalWritten := int64(0)
	totalSize, err := common.GetDirSize(folderPath)
	if err != nil {
		return fmt.Errorf("failed to get folder size: %w", err)
	}

	err = filepath.Walk(folderPath, func(filePath string, info os.FileInfo, err error) error {
		if err != nil {
			return fmt.Errorf("failed to walk through folder: %w", err)
		}

		if info.Mode()&os.ModeSymlink != 0 {
			logger.Debugf("Skipping symlink: %s", filePath)
			return nil
		}

		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return fmt.Errorf("failed to create zip header: %w", err)
		}

		header.Name, _ = filepath.Rel(filepath.Clean(folderPath), filePath)
		if info.IsDir() {
			header.Name += "/"
		} else {
			header.Method = zip.Deflate
		}

		writer, err := zipWriter.CreateHeader(header)
		if err != nil {
			return fmt.Errorf("failed to create zip writer: %w", err)
		}

		if !info.IsDir() {
			file, err := os.Open(filePath)
			if err != nil {
				return fmt.Errorf("failed to open file: %w", err)
			}
			defer file.Close()
			fileSize := info.Size()
			_, err = common.CopyWithProgress(writer, file, func(written int, fileWritten int64) {
				totalWritten += int64(written)
				if progressFunc != nil {
					progressFunc(
						filePath,
						written,
						fileWritten,
						fileSize,
						totalWritten,
						totalSize,
					)
				}
			})

			if err != nil {
				return fmt.Errorf("failed to write file to zip: %w", err)
			}
		}
		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to zip files: %w", err)
	}

	return nil
}
