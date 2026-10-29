package backup

import (
	"context"
	"fmt"
	"time"

	"github.com/mgarnier-home/utils/common"
	httputils "github.com/mgarnier-home/utils/http"
	"github.com/mgarnier-home/utils/logger"
	"github.com/mgarnier-home/utils/ntfy"
	"mgarnier11.fr/go/autosaver/config"
	"mgarnier11.fr/go/autosaver/external"
)

type Execution struct {
	Success       bool
	Duration      time.Duration
	TimeFormatted string
}

var running bool = false

var LastExecution *Execution = &Execution{
	Success:       false,
	Duration:      time.Duration(0),
	TimeFormatted: "",
}

func formatDuration(d time.Duration) string {
	// Calculate total minutes and seconds
	totalMinutes := int(d.Minutes())
	minutes := totalMinutes % 60
	seconds := int(d.Seconds()) % 60

	// Format as mm:ss
	return fmt.Sprintf("%02dm %02ds", minutes, seconds)
}

func RunSave(appConfig *config.AppConfigFile) bool {
	if running {
		return false
	}
	running = true

	go func() {
		ctx, cancel := context.WithCancel(context.Background())

		defer func() {
			running = false
			cancel()
		}()

		if appConfig.KeepAliveUrl != "" {
			logger.Infof("Starting keep alive url: %s", appConfig.KeepAliveUrl)
			go common.RunPeriodic(ctx, 30*time.Second, func() {
				logger.Infof("Running keep alive request to %s", appConfig.KeepAliveUrl)
				err := httputils.GetRequest(appConfig.KeepAliveUrl)

				if err != nil {
					logger.Errorf("Failed to send keep alive request: %s", err)
				} else {
					logger.Infof("Keep alive request sent successfully")
				}
			})
		}

		timeStart := time.Now()

		err := save(appConfig)

		LastExecution.Duration = time.Since(timeStart)
		LastExecution.TimeFormatted = formatDuration(LastExecution.Duration)
		LastExecution.Success = err == nil

		if err != nil {
			logger.Errorf("Failed to save: %s in %s", err, LastExecution.TimeFormatted)

			err = external.SendMail(
				appConfig.Mail,
				appConfig.Mail.ErrorTo,
				fmt.Sprintf("Error for %s of %s", appConfig.FileName, common.GetDateOfDay()),
				fmt.Sprintf("Error: %s", err),
			)
			if err != nil {
				logger.Errorf("Failed to send mail: %s", err)
			}

			err = ntfy.SendNotification(
				"Autosaver",
				fmt.Sprintf("🔴 Backup of %s failed in %s", appConfig.FileName, LastExecution.TimeFormatted),
				"bomb",
			)
			if err != nil {
				logger.Errorf("Failed to send ntfy notification: %s", err)
			}

			return
		} else {
			err = ntfy.SendNotification(
				"Autosaver",
				fmt.Sprintf("🟢 Backup of %s success in %s", appConfig.FileName, LastExecution.TimeFormatted),
				"partying_face",
			)
			if err != nil {
				logger.Errorf("Failed to send ntfy notification: %s", err)
			}

		}
		logger.Infof("Successfully saved")
	}()

	return true
}

func save(appConfig *config.AppConfigFile) error {

	logger.Infof("Starting backup")
	var err error

	err = zipFolder(appConfig.BackupSrc, appConfig.FileName)
	if err != nil {
		return err
	}

	encryptedFileName := appConfig.FileName + ".gpg"

	password, err := encryptFile(appConfig.FileName, encryptedFileName)
	if err != nil {
		return err
	}

	err = external.SendMail(
		appConfig.Mail,
		appConfig.Mail.InfoTo,
		fmt.Sprintf("Infos for %s of %s", appConfig.FileName, common.GetDateOfDay()),
		fmt.Sprintf("Archive password is : %s", password),
	)
	if err != nil {
		logger.Errorf("Failed to send mail: %s", err)
	}

	if appConfig.LocalDest != "" {
		err = external.CopyToLocal(
			appConfig.LocalDest,
			appConfig.FileName,
		)
		if err != nil {
			return err
		}
	}

	if appConfig.RemoteDest != nil {
		err = external.CopyToRemote(
			appConfig.RemoteDest,
			appConfig.FileName+".gpg",
		)
		if err != nil {
			return err
		}
	}

	return nil

}
