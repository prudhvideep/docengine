package util

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
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

func saveMarkDown(markdownContent string, repoUrl string) error {
	fmt.Println("Saving content")

	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		log.Fatalf("unable to load SDK config, %v", err)
	}

	client := s3.NewFromConfig(cfg)

	input := &s3.PutObjectInput{
		Bucket:      aws.String("docgen-markdown"),
		Key:         aws.String(repoUrl),
		Body:        strings.NewReader(markdownContent),
		ContentType: aws.String("text/markdown"),
	}

	_, err = client.PutObject(context.TODO(), input)
	if err != nil {
		return fmt.Errorf("failed to upload file: %v", err)
	}

	fmt.Printf("Successfully uploaded file to %s\n", repoUrl)

	return nil
}

func saveMermaid(mermaidContent string, repoUrl string) error {
	fmt.Println("Saving Mermaid content")

	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		log.Fatalf("unable to load SDK config, %v", err)
	}

	client := s3.NewFromConfig(cfg)

	input := &s3.PutObjectInput{
		Bucket:      aws.String("docs-overview"),
		Key:         aws.String(repoUrl),
		Body:        strings.NewReader(mermaidContent),
		ContentType: aws.String("text/plain"),
	}

	_, err = client.PutObject(context.TODO(), input)
	if err != nil {
		return fmt.Errorf("failed to upload file: %v", err)
	}

	fmt.Printf("Successfully uploaded the overview file to")

	return nil
}

func PostPrompt(conn *websocket.Conn, repoName string, url string, promptFilePath string) error {
	log.Println("Inside post prompt")

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

	conn.WriteMessage(websocket.TextMessage, []byte("Aritfacts generated"))

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("could not read response: %w", err)
	}

	// log.Println(string(responseBody))
	var parsedResponse Response
	if err := json.Unmarshal(responseBody, &parsedResponse); err != nil {
		return fmt.Errorf("could not parse response JSON: %w", err)
	}

	// // log.Println("Parsed resp ", parsedResponse)

	if len(parsedResponse.Candidates) == 0 ||
		len(parsedResponse.Candidates[0].Content.Parts) == 0 {
		return fmt.Errorf("no content found in response")
	}

	// Get the text from the first part
	markdownContent := parsedResponse.Candidates[0].Content.Parts[0].Text

	fmt.Println(" repoName ---> ", repoName)

	err = saveMarkDown(markdownContent, repoName)
	if err != nil {
		conn.WriteMessage(websocket.TextMessage, []byte("Error uploading the content"))
		return err
	}

	conn.WriteMessage(websocket.TextMessage, []byte("File saved successfully"))

	conn.WriteMessage(websocket.TextMessage, []byte("Generating System Overview"))

	err = generateSystemOverview(conn, markdownContent, repoName)
	if err != nil {
		conn.WriteMessage(websocket.TextMessage, []byte("Error Generating System Overview"))
	}
	return nil

}

func generateSystemOverview(conn *websocket.Conn, markdownContent string, repoUrl string) error {
	geminiKey := os.Getenv("GEMINI_API_KEY")
	if geminiKey == "" {
		log.Println("Api key missing in the environment ")
		return fmt.Errorf("Api key missing in the environment")
	}

	geminiUrl := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/gemini-1.5-flash-latest:generateContent?key=%s", geminiKey)

	err := conn.WriteMessage(websocket.TextMessage, []byte("Preparing the artifacts"))
	if err != nil {
		log.Println("Error getting the summary")
	}

	markdownContent = `This is the overview markdown documentation of my application. I want to get mermaid code for a system oveview for this application so that a user can get a good understanding of the system
	
	Ensure that the Mermaid code does not include invalid characters (e.g., extra semicolons, incorrect indentation, or syntax issues). 
	Give me simple mermaid code which can be rendered right away without much processing
	Don't add things like file name mongo.py in this example in subgraphs as it is giving me errors H[mongo.py (Data Access Layer)]
	Don't try to add any alternate names inside the barckets like main.go here [Main Application (main.go)]. jsut a single name would be fine.
	Also please done add any sepecial characters for example dont add / here D[/generate] and no () in  A[main()]. just give simple names.
	` + markdownContent

	requestBody := map[string]interface{}{
		"contents": []map[string]interface{}{
			{
				"parts": []map[string]string{
					{
						"text": string(markdownContent),
					},
				},
			},
		},
	}

	jsonPayload, err := json.Marshal(requestBody)
	if err != nil {
		return fmt.Errorf("could not marshal JSON payload: %w", err)
	}

	req, err := http.NewRequest("POST", geminiUrl, bytes.NewBuffer(jsonPayload))
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

	conn.WriteMessage(websocket.TextMessage, []byte("Aritfacts generated"))

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("could not read response: %w", err)
	}

	var parsedResponse Response
	if err := json.Unmarshal(responseBody, &parsedResponse); err != nil {
		return fmt.Errorf("could not parse response JSON: %w", err)
	}

	if len(parsedResponse.Candidates) == 0 ||
		len(parsedResponse.Candidates[0].Content.Parts) == 0 {
		return fmt.Errorf("no content found in response")
	}

	mermaidContent := parsedResponse.Candidates[0].Content.Parts[0].Text

	err = saveMermaid(mermaidContent, repoUrl)
	if err != nil {
		conn.WriteMessage(websocket.TextMessage, []byte("Error uploading the content"))
		return err
	}

	return nil
}
