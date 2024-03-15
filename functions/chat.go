package functions

import (
	language "cloud.google.com/go/language/apiv2"
	"context"
	"encoding/json"
	"fmt"
	"github.com/Code-Hex/synchro"
	"github.com/Code-Hex/synchro/tz"
	"github.com/GoogleCloudPlatform/functions-framework-go/functions"
	"github.com/uptrace/bun"
	"golang.org/x/text/unicode/norm"
	"google.golang.org/api/option"
	"google.golang.org/api/youtube/v3"
	"log/slog"
	"net/http"
	"os"
	"regexp"
	"sort"
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
		slog.Info(
			"Live video found",
			slog.Group("liveVideo", "chatId", liveVideos[0].ChatID),
		)
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
	var allChats []Chat

	// Fetch chats from static target video
	staticChats, err := fetchStaticTarget(ctx, dbClient, ytSvc, staticTarget, threshold, targetChannels)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	slog.Info(
		"Fetched chats from upcoming video",
		slog.Group("fetchChat", "chatId", staticTarget.ChatID, slog.Group("static", "sourceId", staticTarget.SourceID, "count", len(staticChats))),
	)
	allChats = append(allChats, staticChats...)

	// If upcoming videos are more than 1, find the priority target
	// to reduce the number of API requests and prevent overuse of quota of YouTube API
	upcomingTarget, lastPublished, err := findPriorityTarget(ctx, dbClient, upcomingVideos)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	// Fetch chats from upcoming videos
	upcomingChats, err := fetchChatsByChatID(ctx, ytSvc, upcomingTarget, 0)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	slog.Info(
		"Fetched chats from upcoming video",
		slog.Group("fetchChat", "chatId", upcomingTarget.ChatID, slog.Group("upcoming", "sourceId", upcomingTarget.SourceID, "count", len(upcomingChats))),
	)
	// Filter the chats by the threshold if the lastPublished is not 0
	// If the lastPublished is 0, the chats are not filtered and all chats are appended to the allChats
	if lastPublished != 0 {
		upcomingChats = filterChatsByPublishedAt(upcomingChats, lastPublished)
	}
	// Filter the chats by the target channels
	upcomingChats, _ = separateChatsByAuthor(upcomingChats, targetChannels)
	// Append the chats to the allChats
	allChats = append(allChats, upcomingChats...)

	// If the length of the staticChats is 0, return
	if len(allChats) == 0 {
		slog.Info("No chats found")
		w.WriteHeader(http.StatusOK)
		return
	}

	// Convert the chats to the chat records
	chatRecords := convertChatsToRecords(allChats)

	// Create Natural Language API client for sentiment analysis
	nlClient, err := NewAnalysisClient(ctx)
	if err != nil {
		slog.Error("Failed to create Natural Language API client",
			slog.Group("saveChat", slog.Group("NaturalLanguageAPI", "error", err)),
		)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Validate the negativity sentiment of the chats
	// Negative flags are used in other linked services
	chatRecords, err = validateNegativitySentiment(ctx, nlClient, chatRecords)

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

func liveChatWatcher(ctx context.Context, ytSvc *youtube.Service, dbClient *bun.DB, video VideoInfo, threshold int64, target []string) error {
	// Fetch chats by YouTube API
	chats, err := fetchChatsByChatID(ctx, ytSvc, video, 0)
	if err != nil {
		slog.Error("Failed to fetch chats from YouTube API",
			slog.Group("fetchChat", "chatId", video.ChatID, "error", err),
		)
		return err
	}

	// Filter the chats by the threshold
	chats = filterChatsByPublishedAt(chats, threshold)
	// Separate the chats by the author channel ID
	targetChats, _ := separateChatsByAuthor(chats, target)

	if len(targetChats) != 0 {
		// If the length of the targetChats is not 0, save the chats to the database
		chatRecords := convertChatsToRecords(targetChats)
		if err := InsertChatRecord(ctx, dbClient, chatRecords); err != nil {
			slog.Error("Failed to insert chat records",
				slog.Group("saveChat", slog.Group("database", "error", err)),
			)
			return err
		}
	}

	return nil
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
			slog.Group("fetchChat", "chatId", video.ChatID, slog.Group("YouTubeAPI", "error", err)),
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

func fetchStaticTarget(ctx context.Context, db *bun.DB, ytSvc *youtube.Service, video VideoInfo, threshold int64, target []string) ([]Chat, error) {
	// Get the last publishedAt of the record
	pldRec, err := getLastPublishedAtOfRecordEachSource(ctx, db, []string{video.SourceID})
	if err != nil {
		slog.Error("Failed to get last publishedAt of record",
			slog.Group("fetchChat", "sourceId", video.SourceID, slog.Group("database", "error", err)),
		)
		return nil, err
	}
	lastPublished := pldRec[video.SourceID]
	// If the last published is greater than the threshold, set the threshold to the last published
	if lastPublished > threshold {
		threshold = lastPublished
	}

	// Fetch chats by YouTube API
	chats, err := fetchChatsByChatID(ctx, ytSvc, video, 0)
	if err != nil {
		slog.Error("Failed to fetch chats from YouTube API",
			slog.Group("fetchChat", "chatId", video.ChatID, "error", err),
		)
		return nil, err
	}

	// Filter the chats by the threshold
	chats = filterChatsByPublishedAt(chats, threshold)
	// Separate the chats by the author channel ID
	targetChats, _ := separateChatsByAuthor(chats, target)

	return targetChats, nil
}

func findPriorityTarget(ctx context.Context, db *bun.DB, videos []VideoInfo) (VideoInfo, int64, error) {
	// Get the last publishedAt of the record in each upcoming video
	if len(videos) == 0 {
		slog.Error(
			"Failed to find priority target",
			slog.Group("fetchChat", "error", "no videos"),
		)
		return VideoInfo{}, 0, fmt.Errorf("no videos")
	}

	ids := make([]string, len(videos))
	for i, video := range videos {
		ids[i] = video.SourceID
	}

	rec, err := getLastPublishedAtOfRecordEachSource(ctx, db, ids)
	if err != nil {
		slog.Error("Failed to get last publishedAt of record",
			slog.Group("fetchChat", "sourceId", ids, slog.Group("database", "error", err)),
		)
		return VideoInfo{}, 0, err
	}

	switch {
	case len(rec) == 0:
		return videos[0], 0, nil
	case len(videos) == 1:
		return videos[0], rec[videos[0].SourceID], nil
	case len(videos) != len(rec):
		for _, video := range videos {
			if _, ok := rec[video.SourceID]; !ok {
				return video, 0, nil
			}
		}
	}

	var target VideoInfo
	var targetSourceID string
	var latestPublished int64

	// Priority is given to retrieving the last saved data with the oldest PublishedAt.
	// The data with the smallest value in map is the priority target
	vals := make([]int64, 0, len(rec))
	for _, published := range rec {
		vals = append(vals, published)
	}
	// Find the smallest value
	sort.Slice(vals, func(i, j int) bool { return vals[i] < vals[j] })
	latestPublished = vals[0]

	// Find the sourceID with the smallest value in the map
	for sourceID, published := range rec {
		if published == latestPublished {
			targetSourceID = sourceID
			break
		}
	}

	// Find the target video info
	for _, video := range videos {
		if video.SourceID == targetSourceID {
			target = video
			break
		}
	}

	return target, latestPublished, nil
}

func validateNegativitySentiment(ctx context.Context, nlClient *language.Client, chats []ChatRecord) ([]ChatRecord, error) {
	// Validate the negativity sentiment of the chats
	// The chats are validated by the sentiment analysis of the Natural Language API
	// If the sentiment is negative, the chat is appended to the result
	var result []ChatRecord

	// Compile the pattern for the stamp for removing the stamp from the message
	// Stamps are not necessary for the sentiment analysis
	// Stamp pattern is like : xxx :
	stmpPattern := regexp.MustCompile(`:[^:]+:`)

	for _, chat := range chats {
		msg := chat.Message
		// Remove the stamp from the message
		msg = stmpPattern.ReplaceAllString(msg, "")
		// Normalize the message
		msg = norm.NFKC.String(msg)
		// Remove emojis from the message
		// Because the emojis are not necessary for the sentiment analysis and occasionally cause an error
		msg = RemoveEmoji(msg)

		if len(msg) == 0 {
			chat.IsNegative = false
			result = append(result, chat)
			continue
		}

		// Analyze the sentiment of the message
		score, magnitude, err := AnalyzeSentiment(ctx, nlClient, msg)
		if err != nil {
			return nil, err
		}

		// If score is less than -1 * magnitude, treat the message as negative
		// when score is another case, treat the message as non-negative
		if score < -1*magnitude {
			chat.IsNegative = true
			result = append(result, chat)
			continue
		}

		chat.IsNegative = false
		result = append(result, chat)
	}

	return result, nil
}
