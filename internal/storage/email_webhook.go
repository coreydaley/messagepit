package storage

import (
	"github.com/coreydaley/messagepit/config"
	"github.com/coreydaley/messagepit/internal/mailevents"
	"github.com/jhillyerd/enmime/v2"
)

// fireEmailEvents replays the SendGrid event lifecycle for a stored message.
//
// Real SendGrid fires the event webhook for everything it accepts, over both
// the v3 API and its SMTP relay, so MessagePit does not gate on custom_args
// being present. Categories and custom_args ride in on the X-SMTPAPI header
// (SendGrid's own SMTP mechanism); the legacy X-Notification-Id header is still
// honoured so existing integrations keep working.
func fireEmailEvents(env *enmime.Envelope, to string) {
	if config.EmailWebhookURL == "" {
		return
	}

	categories, uniqueArgs := mailevents.ParseSMTPAPI(env.GetHeader(mailevents.SMTPAPIHeader))

	if notifID := env.GetHeader("X-Notification-Id"); notifID != "" {
		if uniqueArgs == nil {
			uniqueArgs = map[string]string{}
		}
		if _, ok := uniqueArgs["notification_id"]; !ok {
			uniqueArgs["notification_id"] = notifID
		}
	}

	mailevents.Fire(mailevents.Message{
		Email:      to,
		MessageID:  env.GetHeader("Message-ID"),
		Categories: categories,
		UniqueArgs: uniqueArgs,
		Scenario:   env.GetHeader(mailevents.ScenarioHeader),
	})
}
