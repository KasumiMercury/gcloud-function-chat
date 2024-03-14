package functions

import (
	"log/slog"
	"net/url"
	"strconv"
)

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

func filterChatsByPublishedAt(chats []Chat, threshold int64) []Chat {
	// Filter the chats by the threshold
	// The chats are already sorted by the publishedAt in ascending order (constraint of the YouTube API)

	var filteredChats []Chat

	for i, chat := range chats {
		// If the chat's publishedAt is greater than the threshold, append the chat to the result
		if chat.PublishedAt > threshold {
			filteredChats = chats[i:]
			break
		}
	}

	// Return the result
	return filteredChats
}
