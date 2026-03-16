//go:build ignore
// +build ignore

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

const baseURL = "http://localhost:8081"

func main() {
	// Wait for server to start (if running in parallel, but we will run this manually)
	time.Sleep(2 * time.Second)

	fmt.Println("Starting AI Query verification...")

	// 1. Register/Login User
	registerUser()
	token := loginUser()
	if token == "" {
		fmt.Println("Failed to login, exiting.")
		os.Exit(1)
	}
	fmt.Println("Login successful.")

	// 2. Test Text-to-Query (PromQL)
	testTextToQuery(token, "prometheus", "Show me the request rate over the last 5 minutes")

	// 3. Test Text-to-Query (Elasticsearch)
	testTextToQuery(token, "elasticsearch", "Show me all logs where level is error")

	fmt.Println("AI Query verification completed.")
}

func registerUser() {
	payload := map[string]string{
		"username": "admin",
		"password": "password123",
	}
	jsonData, _ := json.Marshal(payload)
	resp, err := http.Post(baseURL+"/register", "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		fmt.Printf("Register failed: %v\n", err)
		return
	}
	defer resp.Body.Close()
	// Ignore error if user already exists
}

func loginUser() string {
	payload := map[string]string{
		"username": "admin",
		"password": "password123",
	}
	jsonData, _ := json.Marshal(payload)
	resp, err := http.Post(baseURL+"/api/login", "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		fmt.Printf("Login failed: %v\n", err)
		return ""
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		fmt.Printf("Login failed with status: %d\n", resp.StatusCode)
		return ""
	}

	var result map[string]string
	json.NewDecoder(resp.Body).Decode(&result)
	return result["token"]
}

func testTextToQuery(token, dsType, text string) {
	fmt.Printf("Testing Text-to-Query for %s: '%s'\n", dsType, text)
	payload := map[string]string{
		"datasource_type": dsType,
		"query_text":      text,
	}
	jsonData, _ := json.Marshal(payload)
	req, _ := http.NewRequest("POST", baseURL+"/api/ai/text-to-query", bytes.NewBuffer(jsonData))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Printf("Request failed: %v\n", err)
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		fmt.Printf("Failed with status: %d, body: %s\n", resp.StatusCode, string(body))
		return
	}

	fmt.Printf("Success! Response: %s\n", string(body))
}
