package functions

import (
	"github.com/GoogleCloudPlatform/functions-framework-go/functions"
	"log/slog"
	"net/http"
)

func init() {
	functions.HTTP("chat", chatWatcher)
}

func chatWatcher(w http.ResponseWriter, r *http.Request) {
	slog.Info("chatWatcher")
}
