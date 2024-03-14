package functions

type Chat struct {
	AuthorChannelID string
	Message         string
	PublishedAt     int64
	SourceID        string
}

type VideoInfo struct {
	SourceID string `json:"sourceId"`
	Status   string `json:"status"`
	ChatID   string `json:"chatId"`
}
