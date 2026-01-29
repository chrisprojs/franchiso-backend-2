package config

import (
	"context"
	"os"

	"google.golang.org/genai"
)

func NewGemini() *genai.Client {
	if os.Getenv("GEMINI_ACTIVE") == "true"{
		ctx := context.Background()
		cfg := &genai.ClientConfig{
			APIKey:  os.Getenv("GEMINI_API_KEY"),
			Backend: genai.BackendGeminiAPI,
		}
		geminiClient, err := genai.NewClient(ctx, cfg)
		if err != nil {
			panic("Unable to connect to gemini client" + err.Error())
		}
		return geminiClient
	}else{
		return nil
	}
}