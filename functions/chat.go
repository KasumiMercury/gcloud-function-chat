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
		slog.Error("Failed to initialize tracing",
			slog.Group("tracing", slog.Group("InitTracing", "error", err)),
		)

		// If tracing fails to initialize, the program should exit.
		panic(err)
	}
	handler := InstrumentedHandler("chat", chatWatcher, tp)
	functions.HTTP("chat", handler)
}

func chatWatcher(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Set custom logger
	logger := NewCustomLogger(ctx)
	slog.SetDefault(logger)

	// Cache common environment variables
	// Because the function is supposed to run on CloudFunctions, it is necessary to read the environment variables here.
	// If the environment variable is not set, the function will panic.
	// (To prevent retries by CloudScheduler, the function should panic without returning error responses.)
	ytApiKey := os.Getenv("YOUTUBE_API_KEY")
	if ytApiKey == "" {
		slog.Error("YOUTUBE_API_KEY is not set")
		panic("YOUTUBE_API_KEY is not set")
	}

	dsn := os.Getenv("DSN")
	if dsn == "" {
		slog.Error("DSN is not set")
		panic("DSN is not set")
	}
	targetChannelIdStr := os.Getenv("TARGET_CHANNEL_ID")

	if targetChannelIdStr == "" {
		slog.Error("TARGET_CHANNEL_ID is not set")
		panic("TARGET_CHANNEL_ID is not set")
	}
	// Split targetChannelIdStr by comma
	targetChannels := strings.Split(targetChannelIdStr, ",")

	// Initialize span
	span := getSpanQuery(r.URL)
	// Initialize threshold time for filtering chats
	threshold := time.Now().Add(-time.Duration(span) * time.Minute).Unix()

	// Create YouTube service
	ytSvc, err := youtube.NewService(ctx, option.WithAPIKey(ytApiKey))
	if err != nil {
		slog.Error("Failed to create YouTube service",
			"error", err,
			slog.Group("YouTubeAPI"),
		)
		return
	}
	// Create Database Client
	dbClient, err := NewDBClient(dsn)
	if err != nil {
		slog.Error("Failed to create Database client",
			"error", err,
		)
		return
	}

	// load info of video from environment variables
	staticEnv := os.Getenv("STATIC_TARGET")
	var staticTarget VideoInfo
	if err := json.Unmarshal([]byte(staticEnv), &staticTarget); err != nil {
		slog.Error("Failed to unmarshal static target",
			"error", err,
		)
		panic(fmt.Sprintf("Failed to unmarshal static target: %v", err))
	}

	// Fetch chats from StaticTarget
	staticChats, err := fetchChatsByChatID(ctx, ytSvc, staticTarget, 0)
	if err != nil {
		slog.Error("Failed to fetch chats from static target",
			"error", err,
			slog.Group("chatID", staticTarget.ChatID, slog.Group("YouTubeAPI")),
		)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	// If the length of the staticChats is 0, return
	if len(staticChats) == 0 {
		slog.Info("No chats found")
		w.WriteHeader(http.StatusOK)
		return
	}

	// Check publishedAt of the last chat and update threshold if the last chat is newer than the threshold set by span
	// for preventing the same chat from being inserted multiple times
	lastRecordedChat, err := getLastPublishedAtOfRecord(ctx, dbClient)
	if err != nil {
		slog.Error("Failed to get last recorded chat",
			"error", err,
		)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if lastRecordedChat != 0 && lastRecordedChat > threshold {
		threshold = lastRecordedChat
	}

	// Filter chats by publishedAt
	staticChats = filterChatsByPublishedAt(staticChats, threshold)
	targetChat, _ := separateChatsByAuthor(staticChats, targetChannels)

	// Convert the chats to the chat records
	chatRecords := convertChatsToRecords(targetChat)

	// Insert the chats to the database
	if err := InsertChatRecord(ctx, dbClient, chatRecords); err != nil {
		slog.Error("Failed to insert chat records",
			"error", err,
		)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
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
		slog.Error(
			"Failed to run LiveChatMessages.List",
			"error", err,
			slog.Group("chatID", video.ChatID, slog.Group("YouTubeAPI")),
		)
		return nil, err
	}

	result := make([]Chat, 0, len(resp.Items))
	for _, item := range resp.Items {
		pa, err := synchro.ParseISO[tz.AsiaTokyo](item.Snippet.PublishedAt)
		if err != nil {
			slog.Error("Failed to parse publishedAt",
				"error", err,
				slog.Group("chatID", video.ChatID, slog.Group("formatting")),
			)
			return nil, err
		}
		result = append(result, Chat{
			AuthorChannelID: item.Snippet.AuthorChannelId,
			Message:         item.Snippet.DisplayMessage,
			PublishedAtUnix: pa.Unix(),
			SourceID:        video.SourceID,
		})
	}

	return result, nil
}
