package functions

import (
	language "cloud.google.com/go/language/apiv2"
	"cloud.google.com/go/language/apiv2/languagepb"
	"context"
)

func NewAnalysisClient(ctx context.Context) (*language.Client, error) {
	client, err := language.NewClient(ctx)
	if err != nil {
		return nil, err
	}
	return client, nil
}

func AnalyzeSentiment(ctx context.Context, client *language.Client, text string) (float32, float32, error) {
	sentiment, err := client.AnalyzeSentiment(ctx, &languagepb.AnalyzeSentimentRequest{
		Document: &languagepb.Document{
			Source: &languagepb.Document_Content{
				Content: text,
			},
			Type: languagepb.Document_PLAIN_TEXT,
		},
	})
	if err != nil {
		return 0, 0, err
	}
	return sentiment.DocumentSentiment.Score, sentiment.DocumentSentiment.Magnitude, nil
}
