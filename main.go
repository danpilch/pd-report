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

	pdScheduleId := os.Getenv("PAGERDUTY_SCHEDULE_ID")
	if pdScheduleId == "" {
		log.Fatalf("set PAGERDUTY_SCHEDULE_ID environment variable")
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
	// Get the first day of last month golang times suck
	lastMonth := time.Now().AddDate(0, -1, 0).AddDate(0, 0, -time.Now().AddDate(0, -1, 0).Day()+1)

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
		incidentResp, err := client.ListIncidentsWithContext(context.Background(), listOpts)
		if err != nil {
			log.Fatalf("Error fetching incidents: %v", err)
		}

		allIncidents = append(allIncidents, incidentResp.Incidents...)

		if incidentResp.More {
			listOpts.Offset = uint(len(allIncidents))
		} else {
			break
		}
	}

	incidents, err := json.MarshalIndent(allIncidents, "", "  ")
	if err != nil {
		log.Fatalf("Error marshaling incidents: %v", err)
	}

	// Get Schedule for timeperiod
	scheduleOpts := pagerduty.GetScheduleOptions{
		Since:    lastMonth.Format(time.RFC3339),
		Until:    now.Format(time.RFC3339),
		TimeZone: "UTC",
	}

	scheduleResp, err := client.GetScheduleWithContext(context.Background(), pdScheduleId, scheduleOpts)
	if err != nil {
		log.Fatalf("Error fetching schedules: %v", err)
	}

	schedule, err := json.MarshalIndent(scheduleResp, "", "  ")
	if err != nil {
		log.Fatalf("Error marshaling schedule: %v", err)
	}

	oclient := openai.NewClient(openAiToken)
	resp, err := oclient.CreateChatCompletion(
		context.Background(),
		openai.ChatCompletionRequest{
			Model: openai.O3,
			Messages: []openai.ChatCompletionMessage{
				{
					Role:    openai.ChatMessageRoleSystem,
					Content: fmt.Sprintf("PROMPT:\n%s\nREPORT TEMPLATE:\n%s", string(prompt), string(reportTemplate)),
				},
				{
					Role:    openai.ChatMessageRoleUser,
					Content: fmt.Sprintf("REPORT DATE RANGE:%s-%s\n\nSCHEDULE:\n%s\nINCIDENTS:\n%s", string(lastMonth.Format(time.RFC3339)), string(now.Format(time.RFC3339)), string(schedule), string(incidents)),
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
