package functions

import (
	"log/slog"
	"net/url"
	"slices"
	"strconv"
	"time"
)

func getSpanQuery(u *url.URL) (int, error) {
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
		slog.Info("span is empty")
		return defVal, nil
	}

	// Convert the value to an integer
	spanInt, err := strconv.Atoi(span)
	if err != nil {
		slog.Error("Failed to set span because of invalid value")
		return defVal, err
	}

	// Check if the value is too large
	// Because too large value can cause generating invalid threshold
	// Greater than 10080 minutes (7 days) is out of the use case of this function
	// When the value is too large, return an error
	if spanInt > 10080 {
		slog.Error("Failed to set span because of too large value")
		return defVal, err
	}

	// Return the value
	return spanInt, nil
}

func filterChatsByPublishedAt(chats []Chat, threshold int64) []Chat {
	// Filter the chats by the threshold
	// The chats are already sorted by the publishedAt in ascending order (constraint of the YouTube API)

	var filteredChats []Chat

	for i, chat := range chats {
		// If the chat's publishedAt is greater than the threshold, append the chat to the result
		if chat.PublishedAtUnix > threshold {
			filteredChats = chats[i:]
			break
		}
	}

	// Return the result
	return filteredChats
}

func separateChatsByAuthor(chats []Chat, target []string) ([]Chat, []Chat) {
	// Separate the chats by the author channel ID
	// The target list is the list of the author channel IDs to be separated

	var targetChats []Chat
	var otherChats []Chat

	for _, chat := range chats {
		// If the chat's author is in the target list, append the chat to the targetChats
		if slices.Contains(target, chat.AuthorChannelID) {
			targetChats = append(targetChats, chat)
		} else {
			otherChats = append(otherChats, chat)
		}
	}

	// Return the result
	return targetChats, otherChats
}

func convertChatsToRecords(chats []Chat) []ChatRecord {
	// Convert the chats to the chat records
	// The chat records are the struct for the database

	var chatRecords []ChatRecord

	for _, chat := range chats {
		// Convert the chat to the chat record
		chatRecords = append(chatRecords, ChatRecord{
			Message:     chat.Message,
			SourceID:    chat.SourceID,
			PublishedAt: time.Unix(chat.PublishedAtUnix, 0),
		})
	}

	// Return the result
	return chatRecords
}
