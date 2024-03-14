package functions

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/Code-Hex/synchro"
	"github.com/Code-Hex/synchro/tz"
	"github.com/GoogleCloudPlatform/functions-framework-go/functions"
	"google.golang.org/api/option"
	"google.golang.org/api/youtube/v3"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"
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
	// Cache common environment variables
	// Because the function is supposed to run on CloudFunctions, it is necessary to read the environment variables here.
	ytApiKey := os.Getenv("YOUTUBE_API_KEY")
	if ytApiKey == "" {
		slog.Error("YOUTUBE_API_KEY is not set")
		w.WriteHeader(http.StatusInternalServerError)
		panic("YOUTUBE_API_KEY is not set")
	}
	targetChannelIdStr := os.Getenv("TARGET_CHANNEL_ID")
	if targetChannelIdStr == "" {
		slog.Error("TARGET_CHANNEL_ID is not set")
		w.WriteHeader(http.StatusInternalServerError)
		panic("TARGET_CHANNEL_ID is not set")
	}
	// Split targetChannelIdStr by comma
	targetChannels := strings.Split(targetChannelIdStr, ",")

	// Initialize span
	span := getSpanQuery(r.URL)
	// Initialize threshold time for filtering chats
	threshold := time.Now().Add(-time.Duration(span) * time.Minute).Unix()

	// Create YouTube service
	ytSvc, err := youtube.NewService(r.Context(), option.WithAPIKey(ytApiKey))
	if err != nil {
		slog.Error("Failed to create YouTube service: %v", err)
		return
	}

	// load info of video from environment variables
	staticEnv := os.Getenv("STATIC_TARGET")
	var staticTarget VideoInfo
	if err := json.Unmarshal([]byte(staticEnv), &staticTarget); err != nil {
		slog.Error("Failed to unmarshal static target: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		panic(fmt.Sprintf("Failed to unmarshal static target: %v", err))
	}

	// Fetch chats from StaticTarget
	staticChats, err := fetchChatsByChatID(r.Context(), ytSvc, staticTarget, 0)
	if err != nil {
		slog.Error("Failed to fetch chats from static target: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	// Filter chats by publishedAt
	staticChats = filterChatsByPublishedAt(staticChats, threshold)
	targetChat, _ := separateChatsByAuthor(staticChats, targetChannels)

	slog.Info("staticChats", targetChat)
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
