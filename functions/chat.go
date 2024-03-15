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
			slog.Group("tracing", slog.Group("initTracing", "error", err)),
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
	span, err := getSpanQuery(r.URL)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	// Initialize threshold time for filtering chats
	threshold := time.Now().Add(-time.Duration(span) * time.Minute).Unix()

	// Create YouTube service
	ytSvc, err := youtube.NewService(ctx, option.WithAPIKey(ytApiKey))
	if err != nil {
		slog.Error("Failed to create YouTube service",
			slog.Group("YouTubeAPI", "error", err),
		)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	// Create Database Client
	dbClient, err := NewDBClient(dsn)
	if err != nil {
		slog.Error("Failed to create Database client",
			slog.Group("database", "error", err),
		)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Get info of videos with the target status
	targetStatus := []string{"live", "upcoming"}
	videoRecords, err := getVideoRecordByStatus(ctx, dbClient, targetStatus)
	if err != nil {
		slog.Error("Failed to get video records",
			slog.Group("database", "error", err),
		)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Separate processing by status: live or upcoming
	var liveVideos []VideoInfo
	var upcomingVideos []VideoInfo
	for _, video := range videoRecords {
		if video.Status == "live" {
			liveVideos = append(liveVideos, VideoInfo{
				ChatID:   video.ChatID,
				SourceID: video.SourceID,
			})
		} else {
			upcomingVideos = append(upcomingVideos, VideoInfo{
				ChatID:   video.ChatID,
				SourceID: video.SourceID,
			})
		}
	}

	// If there is a live video, process only that video and skip processing of other videos.
	// Because the chat of the target of acquisition is focused on the live video,
	// and chatting to other videos during the live is not necessary for the use case.
	if len(liveVideos) > 0 {
		// TODO: Implement the process for live videos

		// Other videos are skipped
		w.WriteHeader(http.StatusOK)
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

	// Combine the upcoming videos and the static target as fetching targets
	targetVideos := append(upcomingVideos, staticTarget)

	// Check publishedAt of the last chat and update threshold if the last chat is newer than the threshold set by span
	// for preventing the same chat from being inserted multiple times
	lastRecordedChat, err := getLastPublishedAtOfRecord(ctx, dbClient)
	if err != nil {
		slog.Error("Failed to get last recorded chat",
			slog.Group("saveChat", slog.Group("database", "error", err)),
		)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if lastRecordedChat != 0 && lastRecordedChat > threshold {
		threshold = lastRecordedChat
	}

	// Fetch chats from YouTube API
	var allChats []Chat
	for _, video := range targetVideos {
		chats, err := fetchChatsByChatID(ctx, ytSvc, video, 0)
		if err != nil {
			slog.Error("Failed to fetch chats from YouTube API",
				slog.Group("fetchChat", "chatId", video.ChatID, "error", err),
			)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// Filter chats by publishedAt
		chats = filterChatsByPublishedAt(chats, threshold)
		// Filter chats by author
		chats, _ = separateChatsByAuthor(chats, targetChannels)

		allChats = append(allChats, chats...)
	}

	// If the length of the staticChats is 0, return
	if len(allChats) == 0 {
		slog.Info("No chats found")
		w.WriteHeader(http.StatusOK)
		return
	}

	// Convert the chats to the chat records
	chatRecords := convertChatsToRecords(allChats)

	// Insert the chats to the database
	if err := InsertChatRecord(ctx, dbClient, chatRecords); err != nil {
		slog.Error("Failed to insert chat records",
			slog.Group("saveChat", slog.Group("database", "error", err)),
		)
		http.Error(w, err.Error(), http.StatusInternalServerError)
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
			slog.Group("fetchChat", "chatId", video.ChatID, slog.Group("YouTubeAPI"), "error", err),
		)
		return nil, err
	}

	result := make([]Chat, 0, len(resp.Items))
	for _, item := range resp.Items {
		pa, err := synchro.ParseISO[tz.AsiaTokyo](item.Snippet.PublishedAt)
		if err != nil {
			slog.Error("Failed to parse publishedAt",
				slog.Group("fetchChat", "chatID", video.ChatID, slog.Group("formatting", "error", err, "publishedAt", item.Snippet.PublishedAt)),
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
