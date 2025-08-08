package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"

	"github.com/ollama/ollama/api"
)

func getStorageInfo() (string, error) {
	cmd := exec.Command("df", "-h", ".")
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(output), nil
}

func getMemoryInfo() (string, error) {
	cmd := exec.Command("memory_pressure")
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(output), nil
}

func chat(client *api.Client, messages []api.Message, tools []api.Tool, formatted bool) (*api.ChatResponse, error) {
	var stream bool = false
	req := &api.ChatRequest{
		Model:    os.Getenv("ENV_MODEL_NAME"),
		Messages: messages,
		Stream:   &stream,
		Tools:    tools,
		Format: json.RawMessage(`{
			"type":"object",
			"properties":{
				"overall_health":{"type":"string","enum":["excellent","good","warn","fatal"]},
				"storage_health":{"type":"string","enum":["excellent","good","warn","fatal"]},
				"memory_health":{"type":"string","enum":["excellent","good","warn","fatal"]},
				"memory_free_percent":{"type":"number"},
				"storage_used_percent":{"type":"number"}
			},
			"required":[
				"overall_health",
				"storage_health",
				"memory_health",
				"memory_free_percent",
				"storage_used_percent"
			]
		}`),
	}
	if !formatted {
		req.Format = json.RawMessage("")
	}
	if req.Model == "" {
		req.Model = "llama3.2:3b"
	}

	var response *api.ChatResponse
	err := client.Chat(context.Background(), req, func(res api.ChatResponse) error {
		response = &res
		return nil
	})
	if err != nil {
		return nil, err
	}

	return response, nil
}

func main() {
	if err := runChat(); err != nil {
		log.Fatal(err)
	}
}

func setupChat(msg string) ([]api.Message, []api.Tool) {
	messages := []api.Message{
		{
			Role:    "user",
			Content: msg,
		},
	}

	tools := []api.Tool{
		{
			Type: "function",
			Function: api.ToolFunction{
				Name:        "get_storage_info",
				Description: "Get storage info",
			},
		},
		{
			Type: "function",
			Function: api.ToolFunction{
				Name:        "get_memory_info",
				Description: "Get memory info",
			},
		},
	}
	return messages, tools
}

func runChat() error {
	client, err := api.ClientFromEnvironment()
	if err != nil {
		return err
	}

	msg := os.Getenv("ENV_PROMPT")
	if msg == "" {
		msg = "Get Storage/memory info and generate short health report(<350 chars)."
	}

	messages, tools := setupChat(msg)

	res0, err := chat(client, messages, tools, false)
	if err != nil {
		return err
	}

	if len(res0.Message.ToolCalls) == 0 {
		log.Println("no calls got. try again")
		return nil
	}

	const expectedToolCalls = 2
	if len(res0.Message.ToolCalls) != expectedToolCalls {
		log.Println("too few calls. try again")
		return nil
	}

	var updatedMessages []api.Message
	updatedMessages, err = processToolCalls(messages, res0.Message.ToolCalls)
	if err != nil {
		return err
	}
	messages = updatedMessages

	final, err := chat(client, messages, []api.Tool{}, true)
	if err != nil {
		return err
	}

	fmt.Println(final.Message.Content)
	return nil
}

var ErrNoSuchFunc = errors.New("no such func")

func processToolCalls(messages []api.Message, toolCalls []api.ToolCall) ([]api.Message, error) {
	for _, tool := range toolCalls {
		var output string
		var err error
		switch tool.Function.Name {
		case "get_storage_info":
			output, err = getStorageInfo()
		case "get_memory_info":
			output, err = getMemoryInfo()
		default:
			log.Printf("no such func: %s\n", tool.Function.Name)
			return nil, ErrNoSuchFunc
		}

		if err != nil {
			return nil, err
		}

		messages = append(messages, api.Message{
			Role:    "tool",
			Content: output,
			ToolCalls: []api.ToolCall{
				{
					Function: api.ToolCallFunction{
						Name:      tool.Function.Name,
						Arguments: tool.Function.Arguments,
					},
				},
			},
		})
	}
	return messages, nil
}
