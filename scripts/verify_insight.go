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
	// Wait for server to start
	time.Sleep(2 * time.Second)

	fmt.Println("Starting Insight verification...")

	// 1. Register User
	registerUser()

	// 2. Login User
	token := loginUser()
	if token == "" {
		fmt.Println("Failed to login, exiting.")
		os.Exit(1)
	}
	fmt.Println("Login successful.")

	// 3. Create Mock DataSource
	dsID := createMockDataSource(token)
	if dsID == 0 {
		fmt.Println("Failed to create mock datasource, exiting.")
		os.Exit(1)
	}
	fmt.Println("Mock DataSource created.")

	// 4. Test Insight API
	testInsight(token, dsID)
	
	fmt.Println("Insight verification completed successfully.")
}

func registerUser() {
	payload := map[string]string{
		"username": "admin",
		"password": "password123",
	}
	jsonData, _ := json.Marshal(payload)
	http.Post(baseURL+"/register", "application/json", bytes.NewBuffer(jsonData))
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
		return ""
	}

	var result map[string]string
	json.NewDecoder(resp.Body).Decode(&result)
	return result["token"]
}

func createMockDataSource(token string) int {
	timestamp := time.Now().UnixNano()
	payload := map[string]interface{}{
		"name":        fmt.Sprintf("test-mock-%d", timestamp),
		"type":        "mock",
		"url":         "http://mock",
		"description": "Test Mock DataSource",
	}
	jsonData, _ := json.Marshal(payload)
	req, _ := http.NewRequest("POST", baseURL+"/api/datasources", bytes.NewBuffer(jsonData))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Printf("Create DataSource failed: %v\n", err)
		return 0
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		fmt.Printf("Create DataSource failed with status: %d, body: %s\n", resp.StatusCode, string(body))
		return 0
	}

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	return int(result["id"].(float64))
}

func testInsight(token string, dsID int) {
	payload := map[string]interface{}{
		"datasource_id": dsID,
		"message":       "Analyze the following data result", // Use specific message to trigger mock response
	}
	jsonData, _ := json.Marshal(payload)
	req, _ := http.NewRequest("POST", baseURL+"/api/ai/insight", bytes.NewBuffer(jsonData))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Printf("Insight request failed: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		fmt.Printf("Insight request failed with status: %d, body: %s\n", resp.StatusCode, string(body))
		os.Exit(1)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		fmt.Printf("Failed to parse response: %v\n", err)
		os.Exit(1)
	}

	if result["summary"] == "" {
		fmt.Println("Response missing summary")
		os.Exit(1)
	}
	if result["chart_config"] == nil {
		fmt.Println("Response missing chart_config")
		os.Exit(1)
	}
	
	fmt.Printf("Insight Response:\nSummary: %s\nChart Config: %v\n", result["summary"], result["chart_config"])
}
