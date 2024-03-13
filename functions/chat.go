package functions

import (
	"github.com/GoogleCloudPlatform/functions-framework-go/functions"
	"log/slog"
	"net/http"
)

func init() {
	tp, err := InitTracing()
	if err != nil {
		group := slog.Group("init", slog.Group("InitTracing"))
		slog.Error("Failed to initialize tracing: %v", err, group)

		// If tracing fails to initialize, the program should exit.
		panic(err)
	}
	handler := InstrumentedHandler("chat", chatWatcher, tp)
	functions.HTTP("chat", handler)
}

func chatWatcher(w http.ResponseWriter, r *http.Request) {
	slog.Info("chatWatcher")
}
