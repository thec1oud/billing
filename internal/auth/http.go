package auth

import (
	"strings"
)

func IsWebhookPath(path string) bool {
	return strings.HasPrefix(path, "/api/v1/webhooks/")
}
