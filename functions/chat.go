package functions

import (
	"github.com/GoogleCloudPlatform/functions-framework-go/functions"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
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
	// Initialize span
	span := getSpanQuery(r.URL)
	slog.Info("chatWatcher")
}

func getSpanQuery(u *url.URL) int {
	group := slog.Group("getSpanQuery")
	// Default value is 60 minutes
	// Because the update timing of the service group in the upper layer is every 60 minutes
	defVal := 60

	// Check if the request has a query parameter named "span"
	// If it does, return the value of the parameter as an integer
	// If it does not, return default value
	// If the value is not a number, return default value

	// Get the value of the query parameter named "span"
	span := u.Query().Get("span")
	if span == "" {
		slog.Info("span is empty", group)
		return defVal
	}

	// Convert the value to an integer
	spanInt, err := strconv.Atoi(span)
	if err != nil {
		slog.Error("Failed to set span because of invalid value", group)
		return defVal
	}

	// Return the value
	return spanInt
}
