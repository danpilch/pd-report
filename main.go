package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	pagerduty "github.com/PagerDuty/go-pagerduty"
	openai "github.com/sashabaranov/go-openai"
)

func main() {
	pdToken := os.Getenv("PAGERDUTY_API_TOKEN")
	if pdToken == "" {
		log.Fatalf("set PAGERDUTY_API_TOKEN environment variable")
	}

	openAiToken := os.Getenv("OPENAI_API_KEY")
	if openAiToken == "" {
		log.Fatalf("set OPENAI_API_KEY environment variable")
	}

	reportTemplate, err := os.ReadFile("report_template.md")
	if err != nil {
		log.Fatalf("Failed to read file: %v", err)
	}
	prompt, err := os.ReadFile("PROMPT")
	if err != nil {
		log.Fatalf("Failed to read file: %v", err)
	}

	client := pagerduty.NewClient(pdToken)

	now := time.Now().UTC()
	lastMonth := now.AddDate(0, -1, 0)

	listOpts := pagerduty.ListIncidentsOptions{
		Since:    lastMonth.Format(time.RFC3339),
		Until:    now.Format(time.RFC3339),
		TimeZone: "UTC",
		Limit:    200,
		Offset:   0,
		Statuses: []string{"triggered", "acknowledged", "resolved"},
		SortBy:   "created_at:desc",
	}

	var allIncidents []pagerduty.Incident
	for {
		resp, err := client.ListIncidentsWithContext(context.Background(), listOpts)
		if err != nil {
			log.Fatalf("Error fetching incidents: %v", err)
		}

		allIncidents = append(allIncidents, resp.Incidents...)

		if resp.More {
			listOpts.Offset = uint(len(allIncidents))
		} else {
			break
		}
	}

	incidents, err := json.MarshalIndent(allIncidents, "", "  ")
	if err != nil {
		log.Fatalf("Error marshaling incidents: %v", err)
	}

	oclient := openai.NewClient(openAiToken)
	resp, err := oclient.CreateChatCompletion(
		context.Background(),
		openai.ChatCompletionRequest{
			Model: openai.GPT4Dot1,
			Messages: []openai.ChatCompletionMessage{
				{
					Role:    openai.ChatMessageRoleSystem,
					Content: fmt.Sprintf("PROMPT:\n%s\nREPORT TEMPLATE:\n%s", string(prompt), string(reportTemplate)),
				},
				{
					Role:    openai.ChatMessageRoleUser,
					Content: string(incidents),
				},
			},
		},
	)

	if err != nil {
		fmt.Printf("ChatCompletion error: %v\n", err)
		return
	}

	fmt.Println(resp.Choices[0].Message.Content)
}
