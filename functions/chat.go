package functions

import (
	"github.com/GoogleCloudPlatform/functions-framework-go/functions"
	"log/slog"
	"net/http"
)

func init() {
	tp := InitTracing()
	handler := InstrumentedHandler("chat", chatWatcher, tp)
	functions.HTTP("chat", handler)
}

func chatWatcher(w http.ResponseWriter, r *http.Request) {
	slog.Info("chatWatcher")
}
