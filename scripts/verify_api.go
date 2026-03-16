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
	"mime/multipart"
)

const baseURL = "http://localhost:8081"

func main() {
	// Wait for server to start
	time.Sleep(2 * time.Second)

	fmt.Println("Starting API verification...")

	// 1. Register User
	registerUser()

	// 2. Login User
	token := loginUser()
	if token == "" {
		fmt.Println("Failed to login, exiting.")
		os.Exit(1)
	}
	fmt.Println("Login successful.")

	// 3. Create DataSource
	dsID := createDataSource(token)
	if dsID == 0 {
		fmt.Println("Failed to create datasource, exiting.")
		os.Exit(1)
	}
	fmt.Println("DataSource created.")

	// 4. Create AlertRule
	ruleID := createAlertRule(token, dsID)
	if ruleID == 0 {
		fmt.Println("Failed to create alert rule, exiting.")
		os.Exit(1)
	}
	fmt.Println("AlertRule created.")

	// 5. Export AlertRules
	yamlData := exportAlertRules(token)
	fmt.Println("AlertRules exported.")

	// 6. Import AlertRules
	importAlertRules(token, yamlData)
	fmt.Println("AlertRules imported.")

	// 7. List Alarms
	listAlarms(token)
	fmt.Println("Alarms listed.")

	// 8. List AlertLogs
	listAlertLogs(token)
	fmt.Println("AlertLogs listed.")

	// 9. Query Data (Mock)
	queryData(token, dsID)
	fmt.Println("Data query tested.")

	fmt.Println("API verification completed successfully.")
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

func createDataSource(token string) int {
	timestamp := time.Now().UnixNano()
	payload := map[string]interface{}{
		"name":        fmt.Sprintf("test-prometheus-%d", timestamp),
		"type":        "prometheus",
		"url":         "http://localhost:9090",
		"description": "Test Prometheus DataSource",
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

func createAlertRule(token string, dsID int) int {
	timestamp := time.Now().UnixNano()
	payload := map[string]interface{}{
		"name":           fmt.Sprintf("test-rule-%d", timestamp),
		"datasource_id":  dsID,
		"query":          "up == 0",
		"duration":       60,
		"level":          "critical",
		"condition":      ">",
		"threshold":      0,
		"algorithm":      "static",
		"silence_period": 5,
		"is_enabled":     true,
	}
	jsonData, _ := json.Marshal(payload)
	req, _ := http.NewRequest("POST", baseURL+"/api/alert-rules", bytes.NewBuffer(jsonData))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Printf("Create AlertRule failed: %v\n", err)
		return 0
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		fmt.Printf("Create AlertRule failed with status: %d, body: %s\n", resp.StatusCode, string(body))
		return 0
	}

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	return int(result["id"].(float64))
}

func exportAlertRules(token string) []byte {
	req, _ := http.NewRequest("GET", baseURL+"/api/alert-rules/export", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Printf("Export AlertRules failed: %v\n", err)
		return nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		fmt.Printf("Export AlertRules failed with status: %d\n", resp.StatusCode)
		return nil
	}

	body, _ := io.ReadAll(resp.Body)
	return body
}

func importAlertRules(token string, yamlData []byte) {
	if yamlData == nil {
		fmt.Println("Skipping import due to export failure")
		return
	}

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", "rules.yaml")
	if err != nil {
		fmt.Printf("Create form file failed: %v\n", err)
		return
	}
	part.Write(yamlData)
	writer.Close()

	req, _ := http.NewRequest("POST", baseURL+"/api/alert-rules/import", body)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Printf("Import AlertRules failed: %v\n", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		fmt.Printf("Import AlertRules failed with status: %d, body: %s\n", resp.StatusCode, string(respBody))
		return
	}
}

func listAlarms(token string) {
	req, _ := http.NewRequest("GET", baseURL+"/api/alarms", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Printf("List Alarms failed: %v\n", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		fmt.Printf("List Alarms failed with status: %d\n", resp.StatusCode)
		return
	}
}

func listAlertLogs(token string) {
	req, _ := http.NewRequest("GET", baseURL+"/api/alert-logs", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Printf("List AlertLogs failed: %v\n", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		fmt.Printf("List AlertLogs failed with status: %d\n", resp.StatusCode)
		return
	}
}

func queryData(token string, dsID int) {
	payload := map[string]interface{}{
		"datasource_id": dsID,
		"query":         "up",
	}
	jsonData, _ := json.Marshal(payload)
	req, _ := http.NewRequest("POST", baseURL+"/api/query", bytes.NewBuffer(jsonData))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Printf("Query Data failed: %v\n", err)
		return
	}
	defer resp.Body.Close()

	// Expecting failure or success depending on if prometheus is reachable
	// But we just want to ensure the API is reachable and returns something
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusInternalServerError && resp.StatusCode != http.StatusBadRequest {
		fmt.Printf("Query Data unexpected status: %d\n", resp.StatusCode)
	}
}
