package backup

import (
	"context"
	"fmt"
	"os"
	"sync/atomic"
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

var running atomic.Bool

// LastExecution reste nil tant qu'aucune sauvegarde n'a tourné depuis le démarrage.
var LastExecution atomic.Pointer[Execution]

func RunSave(appConfig *config.AppConfigFile) bool {
	// CompareAndSwap : deux /run simultanés ne peuvent pas lancer deux sauvegardes sur le même fichier.
	if !running.CompareAndSwap(false, true) {
		return false
	}

	go func() {
		ctx, cancel := context.WithCancel(context.Background())

		defer func() {
			running.Store(false)
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

		duration := time.Since(timeStart)
		execution := &Execution{
			Success:       err == nil,
			Duration:      duration,
			TimeFormatted: duration.Round(time.Second).String(),
		}
		LastExecution.Store(execution)

		if err != nil {
			logger.Errorf("Failed to save: %s in %s", err, execution.TimeFormatted)

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
				fmt.Sprintf("🔴 Backup of %s failed in %s", appConfig.FileName, execution.TimeFormatted),
				"bomb",
			)
			if err != nil {
				logger.Errorf("Failed to send ntfy notification: %s", err)
			}

			return
		} else {
			err = ntfy.SendNotification(
				"Autosaver",
				fmt.Sprintf("🟢 Backup of %s success in %s", appConfig.FileName, execution.TimeFormatted),
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

	encryptedFileName := appConfig.FileName + ".gpg"
	// Les archives sont écrites hors volume : sans ce nettoyage, elles s'accumulent dans le conteneur.
	defer os.Remove(appConfig.FileName)
	defer os.Remove(encryptedFileName)

	err = zipFolder(appConfig.BackupSrc, appConfig.FileName)
	if err != nil {
		return err
	}

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
