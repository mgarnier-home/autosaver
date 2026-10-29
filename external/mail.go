package external

import (
	"gopkg.in/gomail.v2"

	"mgarnier11.fr/go/autosaver/config"
)

func SendMail(
	mailConfig *config.MailConfig,
	to,
	subject,
	body string,
) error {

	dialer := gomail.NewDialer(mailConfig.Host, mailConfig.Port, mailConfig.Login, mailConfig.Password)

	message := gomail.NewMessage()
	message.SetHeader("From", mailConfig.Login)
	message.SetHeader("To", to)
	message.SetHeader("Subject", subject)
	message.SetBody("text/plain", body)

	return dialer.DialAndSend(message)

}
