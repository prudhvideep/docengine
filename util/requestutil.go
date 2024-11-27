package util

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/gorilla/websocket"
)

type Response struct {
	Candidates []Candidate `json:"candidates"`
}

type Candidate struct {
	Content Content `json:"content"`
}

type Content struct {
	Parts []Part `json:"parts"`
	Role  string `json:"role"`
}

type Part struct {
	Text string `json:"text"`
}

func PostPrompt(conn *websocket.Conn,repoName string, url string, promptFilePath string) error {
  wd,e := os.Getwd()
	if e != nil {
		return e
	}

	docsPath := filepath.Join(wd,"docs",repoName+".md")
	
	file, err := os.Open(promptFilePath)
	if err != nil {
		return err
	}

	promptContent, err := io.ReadAll(file)
	if err != nil {
		return fmt.Errorf("could not read prompt file: %w", err)
	}

	requestBody := map[string]interface{}{
		"contents": []map[string]interface{}{
			{
				"parts": []map[string]string{
					{
						"text": string(promptContent),
					},
				},
			},
		},
	}

	jsonPayload, err := json.Marshal(requestBody)
	if err != nil {
		return fmt.Errorf("could not marshal JSON payload: %w", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonPayload))
	if err != nil {
		return fmt.Errorf("could not create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("could not send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("request failed with status %d: %s", resp.StatusCode, string(body))
	}

	conn.WriteMessage(websocket.TextMessage,[]byte("Aritfacts generated from Gemini"))

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("could not read response: %w", err)
	}

	// log.Println(string(responseBody))
	var parsedResponse Response
	if err := json.Unmarshal(responseBody, &parsedResponse); err != nil {
		return fmt.Errorf("could not parse response JSON: %w", err)
	}

	// log.Println("Parsed resp ", parsedResponse)

	if len(parsedResponse.Candidates) == 0 ||
		len(parsedResponse.Candidates[0].Content.Parts) == 0 {
		return fmt.Errorf("no content found in response")
	}

	// Get the text from the first part
	markdownContent := parsedResponse.Candidates[0].Content.Parts[0].Text

	err = os.WriteFile(docsPath, []byte(markdownContent), 0644)
	if err != nil {
		return fmt.Errorf("could not write to file %s: %w", docsPath, err)
	}

	log.Printf("Markdown saved to %s", docsPath)

	return nil

}
