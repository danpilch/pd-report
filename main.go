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
	apiToken := os.Getenv("PAGERDUTY_API_TOKEN")
	if apiToken == "" {
		log.Fatal("Set PAGERDUTY_API_TOKEN environment variable")
	}

	client := pagerduty.NewClient(apiToken)

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

	b, err := json.MarshalIndent(allIncidents, "", "  ")
	if err != nil {
		log.Fatalf("Error marshaling incidents: %v", err)
	}

	oclient := openai.NewClient(os.Getenv("OPENAI_API_KEY"))
	resp, err := oclient.CreateChatCompletion(
		context.Background(),
		openai.ChatCompletionRequest{
			Model: openai.GPT4Dot1,
			Messages: []openai.ChatCompletionMessage{
				{
					Role: openai.ChatMessageRoleSystem,
					Content: `
You are an incident response guru, your job is to take this json payload of 
the previous month's pagerduty incidents analyse and spot trends, find metrics 
e.g. who was paged most, find patterns in alerts etc. and create a exec summary/report 
of your findings. You should also analyse the on-call rotation for who was on call, 
when and for how long and weave that into your response. Your response should be in markdown 
format. You should include incident Supporting Data like 
'Incident Frequency by Service/Handler', 'Incidents by Category' as tables and Conclusions and Next Steps.
Make sure you are ACCURATE with the amount of incidents, we need to be concise`,
				},
				{
					Role:    openai.ChatMessageRoleUser,
					Content: string(b),
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
