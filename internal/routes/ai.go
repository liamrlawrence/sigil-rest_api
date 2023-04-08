package routes

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/liamrlawrence/sigil-rest_api/internal/logging"
	"github.com/liamrlawrence/sigil-rest_api/internal/server"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

func ChatGPTRequest(s *server.Server, GptModel string, GptTemperature float32) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		switch GptModel {
		case "gpt-3.5-turbo":
			logging.APIEndpoint(r, "POST", fmt.Sprintf("/api/ai/gpt3 | %v %v", GptModel, GptTemperature))
		case "gpt-4-0314":
			logging.APIEndpoint(r, "POST", fmt.Sprintf("/api/ai/gpt4 | %v %v", GptModel, GptTemperature))
		}

		// Parse the request from the user
		type RequestBody struct {
			Name    string `json:"name"`
			Message string `json:"message"`
		}

		var requestBody RequestBody

		err := json.NewDecoder(r.Body).Decode(&requestBody)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			fmt.Fprintf(w, `{
	"status": "failed",
	"message": "failed to read request body"
}`)
			return
		}

		if requestBody.Message == "" || requestBody.Name == "" {
			w.WriteHeader(http.StatusBadRequest)
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			fmt.Fprintf(w, `{
	"status": "failed",
	"message": "request body requires fields 'message' and 'name'"
}`)
			return
		}

		// Set up the GPT API request
		var OpenaiApiKey = os.Getenv("OPENAI_API_KEY")

		client := &http.Client{
			Timeout: 300 * time.Second,
		}

		data := []byte(fmt.Sprintf(`{
	"model": "%v",
	"messages": [{
		"role": "user",
		"content": "%v"
	}],
	"temperature": %v
}`,
			GptModel, strings.Replace(strings.Replace(requestBody.Message, "\n", " ", -1), "\"", "\\\"", -1), GptTemperature))
		req, err := http.NewRequest("POST", "https://api.openai.com/v1/chat/completions", bytes.NewBuffer(data))
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			fmt.Fprintf(w, `{
	"status": "failed",
	"message": "failed to create GPT API request"
}`)
			return
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+OpenaiApiKey)

		// Send request
		gptResp, err := client.Do(req)

		// Parse the GPT API response
		type Message struct {
			Content string `json:"content"`
			Role    string `json:"role"`
		}

		type Choice struct {
			FinishReason string  `json:"finish_reason"`
			Index        int     `json:"index"`
			Message      Message `json:"message"`
		}

		type Usage struct {
			CompletionTokens int `json:"completion_tokens"`
			PromptTokens     int `json:"prompt_tokens"`
			TotalTokens      int `json:"total_tokens"`
		}

		type ChatCompletionResponse struct {
			Choices []Choice `json:"choices"`
			Created float64  `json:"created"`
			ID      string   `json:"id"`
			Model   string   `json:"model"`
			Object  string   `json:"object"`
			Usage   Usage    `json:"usage"`
		}

		var responseBody ChatCompletionResponse

		err = json.NewDecoder(gptResp.Body).Decode(&responseBody)
		if err != nil {
			http.Error(w, "Failed to read GPT API response body", http.StatusBadRequest)
			return
		}

		// Log the expenses in Postgres
		sessionID := r.Header.Get("X-Grimoire-Token")
		conn, err := s.DBPool.Query(context.Background(), "CALL SP_Insert_GPT_Bill($1, $2, $3, $4, $5);",
			sessionID, requestBody.Name, GptModel, responseBody.Usage.PromptTokens, responseBody.Usage.CompletionTokens)
		if err != nil {
			log.Fatalf("Failed to execute query: %v", err)
		}
		defer conn.Close()

		// Return `message` and `usage` back to the user
		//fmt.Printf("\nDEBUG1: %v\n\n", responseBody.Choices[0].Message.Content)
		//fmt.Printf("\nDEBUG2: %v\n\n", strings.Replace(strings.Replace(responseBody.Choices[0].Message.Content, "\n", "\\n", -1), "\"", "\\\"", -1))
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		fmt.Fprint(w, fmt.Sprintf(`{
	"message": "%v",
	"usage": {
		"prompt_tokens": %v,
		"completion_tokens": %v
	},
	"model": "%v"
}`,
			strings.Replace(strings.Replace(responseBody.Choices[0].Message.Content, "\n", "\\n", -1), "\"", "\\\"", -1),
			responseBody.Usage.PromptTokens,
			responseBody.Usage.CompletionTokens, GptModel))
		return
	}
}

func HandlerRouteChatGPT35_Turbo(s *server.Server) func(w http.ResponseWriter, r *http.Request) {
	return ChatGPTRequest(s, "gpt-3.5-turbo", 0.7)
}

func HandlerRouteChatGPT4(s *server.Server) func(w http.ResponseWriter, r *http.Request) {
	return ChatGPTRequest(s, "gpt-4-0314", 0.7)
}

func HandlerRouteAIBills(s *server.Server) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		logging.APIEndpoint(r, "GET", "/api/ai/bills")

		rows, err := s.DBPool.Query(context.Background(), "SELECT * FROM View_AI_Bills;")
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			fmt.Fprintf(w, `{
	"status": "failed",
	"message": "failed to get GPT bills"
}`)
			return
		}
		defer rows.Close()

		// Get the column names
		columns := make([]string, len(rows.FieldDescriptions()))
		for i, fd := range rows.FieldDescriptions() {
			columns[i] = fd.Name
		}

		rowDelimiter := "~~"
		bills := fmt.Sprintf("%s,%s,%s,%s,%s%s", columns[0], columns[1], columns[2], columns[3], columns[4], rowDelimiter)

		// Get the column values
		values := make([]string, len(columns))
		for rows.Next() {
			err := rows.Scan(&values[0], &values[1], &values[2], &values[3], &values[4])
			if err != nil {
				w.WriteHeader(http.StatusBadRequest)
				w.Header().Set("Content-Type", "application/json; charset=utf-8")
				fmt.Fprintf(w, `{
	"status": "failed",
	"message": "failed to get GPT bills during query"
}`)
				return
			}
			bills += fmt.Sprintf("%s,%s,%s,%s,%s%s", values[0], values[1], values[2], values[3], values[4], rowDelimiter)
		}
		bills = bills[:len(bills)-2]

		// Return the query
		w.WriteHeader(http.StatusOK)
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		fmt.Fprintf(w, `{
	"status": "success",
	"message": "got table of AI bills",
    "data": "%v"
}`, bills)
		return
	}
}
