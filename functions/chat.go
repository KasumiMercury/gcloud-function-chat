package functions

import (
	"context"
	"github.com/Code-Hex/synchro"
	"github.com/Code-Hex/synchro/tz"
	"github.com/GoogleCloudPlatform/functions-framework-go/functions"
	"google.golang.org/api/option"
	"google.golang.org/api/youtube/v3"
	"log/slog"
	"net/http"
	"os"
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
	// Cache environment variables
	// Because the function is supposed to run on CloudFunctions, it is necessary to read the environment variables here.
	ytApiKey := os.Getenv("YOUTUBE_API_KEY")
	if ytApiKey == "" {
		slog.Error("YOUTUBE_API_KEY is not set")
		w.WriteHeader(http.StatusInternalServerError)
		panic("YOUTUBE_API_KEY is not set")
	}

	// Initialize span
	span := getSpanQuery(r.URL)

	// Create YouTube service
	ytSvc, err := youtube.NewService(r.Context(), option.WithAPIKey(ytApiKey))
	if err != nil {
		slog.Error("Failed to create YouTube service: %v", err)
		return
	}
	slog.Info("chatWatcher")
}

func fetchChatsByChatID(ctx context.Context, ytSvc *youtube.Service, video VideoInfo, length int64) ([]Chat, error) {
	call := ytSvc.LiveChatMessages.List(video.ChatID, []string{"snippet"})

	// If length is not 0, set the length
	if length != 0 {
		call = call.MaxResults(length)
	}

	call = call.Context(ctx)

	resp, err := call.Do()
	if err != nil {
		return nil, err
	}

	result := make([]Chat, 0, len(resp.Items))
	for _, item := range resp.Items {
		pa, err := synchro.ParseISO[tz.AsiaTokyo](item.Snippet.PublishedAt)
		if err != nil {
			return nil, err
		}
		result = append(result, Chat{
			AuthorChannelID: item.Snippet.AuthorChannelId,
			Message:         item.Snippet.DisplayMessage,
			PublishedAt:     pa.Unix(),
			SourceID:        video.SourceID,
		})
	}

	return result, nil
}
