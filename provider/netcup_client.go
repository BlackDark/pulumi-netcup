// Copyright 2025, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package provider

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	NetcupAPIEndpoint = "https://ccp.netcup.net/run/webservice/servers/endpoint.php?JSON"
	APITimeout        = 30 * time.Second
)

// NetcupClient handles communication with the Netcup API
type NetcupClient struct {
	apiKey      string
	apiPassword string
	customerID  string
	httpClient  *http.Client
}

// NetcupAPIRequest represents the structure of API requests
type NetcupAPIRequest struct {
	Action string      `json:"action"`
	Param  interface{} `json:"param"`
}

// NetcupAPIResponse represents the structure of API responses
type NetcupAPIResponse struct {
	ServerRequestID string      `json:"serverrequestid"`
	ClientRequestID string      `json:"clientrequestid"`
	Action          string      `json:"action"`
	Status          string      `json:"status"`
	StatusCode      int         `json:"statuscode"`
	ShortMessage    string      `json:"shortmessage"`
	LongMessage     string      `json:"longmessage"`
	ResponseData    interface{} `json:"responsedata"`
}

// DnsRecordInfo represents DNS record information from the API
type DnsRecordInfo struct {
	ID           string `json:"id,omitempty"`
	Hostname     string `json:"hostname"`
	Type         string `json:"type"`
	Priority     string `json:"priority,omitempty"`
	Destination  string `json:"destination"`
	DeleteRecord bool   `json:"deleterecord,omitempty"`
	State        string `json:"state,omitempty"`
}

// LoginParams represents login parameters
type LoginParams struct {
	CustomerNumber string `json:"customernumber"`
	APIKey         string `json:"apikey"`
	APIPassword    string `json:"apipassword"`
}

// NewNetcupClient creates a new Netcup API client
func NewNetcupClient(apiKey, apiPassword, customerID string) *NetcupClient {
	return &NetcupClient{
		apiKey:      apiKey,
		apiPassword: apiPassword,
		customerID:  customerID,
		httpClient: &http.Client{
			Timeout: APITimeout,
		},
	}
}

func (c *NetcupClient) login() (string, error) {
	params := LoginParams{
		CustomerNumber: c.customerID,
		APIKey:         c.apiKey,
		APIPassword:    c.apiPassword,
	}

	request := NetcupAPIRequest{
		Action: "login",
		Param:  params,
	}

	response, err := c.makeAPICall(request)
	if err != nil {
		return "", fmt.Errorf("login failed: %w", err)
	}

	if response.Status != "success" {
		switch {
		case response.StatusCode == 2011:
			return "", fmt.Errorf("login failed: Invalid API credentials (API key or password incorrect)")
		case response.StatusCode == 2029:
			return "", fmt.Errorf("login failed: Customer account not found (check customer ID)")
		case response.StatusCode == 2057:
			return "", fmt.Errorf("login failed: Rate limit exceeded (more than 180 requests per minute). Please wait and retry later")
		default:
			return "", fmt.Errorf("login failed: %s (status code: %d)", response.LongMessage, response.StatusCode)
		}
	}

	sessionData, ok := response.ResponseData.(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("login failed: invalid response format")
	}

	sessionID, ok := sessionData["apisessionid"].(string)
	if !ok {
		return "", fmt.Errorf("login failed: no session ID returned in response")
	}

	return sessionID, nil
}

func (c *NetcupClient) logout(sessionID string) error {
	logoutParams := struct {
		CustomerNumber string `json:"customernumber"`
		APIKey         string `json:"apikey"`
		SessionID      string `json:"apisessionid"`
	}{
		CustomerNumber: c.customerID,
		APIKey:         c.apiKey,
		SessionID:      sessionID,
	}

	request := NetcupAPIRequest{
		Action: "logout",
		Param:  logoutParams,
	}

	_, err := c.makeAPICall(request)
	return err
}

func (c *NetcupClient) makeAPICall(request NetcupAPIRequest) (*NetcupAPIResponse, error) {
	jsonData, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	resp, err := c.httpClient.Post(NetcupAPIEndpoint, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var apiResponse NetcupAPIResponse
	if err := json.Unmarshal(body, &apiResponse); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &apiResponse, nil
}

func (c *NetcupClient) CreateDnsRecord(domain, name, recordType, value, priority string) (string, error) {
	sessionID, err := c.login()
	if err != nil {
		return "", err
	}
	defer c.logout(sessionID)

	existingRecords, err := c.getAllDnsRecords(sessionID, domain)
	if err != nil {
		return "", fmt.Errorf("failed to get existing DNS records: %w", err)
	}

	newRecord := &DnsRecordInfo{
		Hostname:    name,
		Type:        recordType,
		Destination: value,
	}

	if priority != "" {
		newRecord.Priority = priority
	}

	allRecords := append(existingRecords, newRecord)

	err = c.updateAllDnsRecords(sessionID, domain, allRecords)
	if err != nil {
		return "", fmt.Errorf("failed to create DNS record: %w", err)
	}

	updatedRecords, err := c.getAllDnsRecords(sessionID, domain)
	if err != nil {
		return "", fmt.Errorf("failed to get updated DNS records to find new record ID: %w", err)
	}

	for _, record := range updatedRecords {
		match := record.Hostname == name && record.Type == recordType && record.Destination == value
		if recordType == "MX" && priority != "" {
			match = match && record.Priority == priority
		}

		if match && record.ID != "" {
			return record.ID, nil
		}
	}

	return "", fmt.Errorf("DNS record was created successfully but no record ID was found. This may indicate an API issue")
}

func (c *NetcupClient) DeleteDnsRecord(recordID, domain string) error {
	sessionID, err := c.login()
	if err != nil {
		return err
	}
	defer c.logout(sessionID)

	existingRecords, err := c.getAllDnsRecords(sessionID, domain)
	if err != nil {
		return fmt.Errorf("failed to get existing DNS records: %w", err)
	}

	found := false
	for _, record := range existingRecords {
		if record.ID == recordID {
			record.DeleteRecord = true
			found = true
			break
		}
	}

	if !found {
		return nil
	}

	err = c.updateAllDnsRecords(sessionID, domain, existingRecords)
	if err != nil {
		return fmt.Errorf("failed to delete DNS record: %w", err)
	}

	return nil
}

func (c *NetcupClient) UpdateDnsRecord(recordID, domain, name, recordType, value, priority string) error {
	sessionID, err := c.login()
	if err != nil {
		return err
	}
	defer c.logout(sessionID)

	existingRecords, err := c.getAllDnsRecords(sessionID, domain)
	if err != nil {
		return fmt.Errorf("failed to get existing DNS records: %w", err)
	}

	found := false
	for _, record := range existingRecords {
		if record.ID == recordID {
			record.Hostname = name
			record.Type = recordType
			record.Destination = value
			if priority != "" {
				record.Priority = priority
			} else {
				record.Priority = ""
			}
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("DNS record not found: %s", recordID)
	}

	err = c.updateAllDnsRecords(sessionID, domain, existingRecords)
	if err != nil {
		return fmt.Errorf("failed to update DNS record: %w", err)
	}

	return nil
}

func (c *NetcupClient) GetDnsRecordById(recordID, domain string) (*DnsRecordInfo, error) {
	sessionID, err := c.login()
	if err != nil {
		return nil, err
	}
	defer c.logout(sessionID)

	params := struct {
		CustomerNumber string `json:"customernumber"`
		APIKey         string `json:"apikey"`
		SessionID      string `json:"apisessionid"`
		DomainName     string `json:"domainname"`
	}{
		CustomerNumber: c.customerID,
		APIKey:         c.apiKey,
		SessionID:      sessionID,
		DomainName:     domain,
	}

	request := NetcupAPIRequest{
		Action: "infoDnsRecords",
		Param:  params,
	}

	response, err := c.makeAPICall(request)
	if err != nil {
		return nil, err
	}

	if response.Status != "success" {
		return nil, fmt.Errorf("get DNS records failed: %s (status code: %d)", response.LongMessage, response.StatusCode)
	}

	responseData, ok := response.ResponseData.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid DNS records response format")
	}

	recordsData, ok := responseData["dnsrecords"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("no DNS records found in response (response structure: %+v)", responseData)
	}
	for _, recordData := range recordsData {
		recordMap, ok := recordData.(map[string]interface{})
		if !ok {
			continue
		}

		id, _ := recordMap["id"].(string)
		if id == recordID {
			hostname, _ := recordMap["hostname"].(string)
			recordType, _ := recordMap["type"].(string)
			destination, _ := recordMap["destination"].(string)

			record := &DnsRecordInfo{
				ID:          id,
				Hostname:    hostname,
				Type:        recordType,
				Destination: destination,
			}

			if priority, ok := recordMap["priority"].(string); ok {
				record.Priority = priority
			}
			if state, ok := recordMap["state"].(string); ok {
				record.State = state
			}
			return record, nil
		}
	}

	return nil, fmt.Errorf("DNS record not found: %s", recordID)
}

func (c *NetcupClient) getAllDnsRecords(sessionID, domain string) ([]*DnsRecordInfo, error) {
	params := struct {
		CustomerNumber string `json:"customernumber"`
		APIKey         string `json:"apikey"`
		SessionID      string `json:"apisessionid"`
		DomainName     string `json:"domainname"`
	}{
		CustomerNumber: c.customerID,
		APIKey:         c.apiKey,
		SessionID:      sessionID,
		DomainName:     domain,
	}

	request := NetcupAPIRequest{
		Action: "infoDnsRecords",
		Param:  params,
	}

	response, err := c.makeAPICall(request)
	if err != nil {
		return nil, err
	}

	if response.Status != "success" {
		return nil, fmt.Errorf("get DNS records failed: %s (status code: %d)", response.LongMessage, response.StatusCode)
	}

	responseData, ok := response.ResponseData.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid DNS records response format")
	}

	recordsData, ok := responseData["dnsrecords"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("no DNS records found in response")
	}

	var records []*DnsRecordInfo
	for _, recordData := range recordsData {
		recordMap, ok := recordData.(map[string]interface{})
		if !ok {
			continue
		}

		id, _ := recordMap["id"].(string)
		hostname, _ := recordMap["hostname"].(string)
		recordType, _ := recordMap["type"].(string)
		destination, _ := recordMap["destination"].(string)

		record := &DnsRecordInfo{
			ID:          id,
			Hostname:    hostname,
			Type:        recordType,
			Destination: destination,
		}

		if priority, ok := recordMap["priority"].(string); ok {
			record.Priority = priority
		}
		if state, ok := recordMap["state"].(string); ok {
			record.State = state
		}

		records = append(records, record)
	}

	return records, nil
}

func (c *NetcupClient) updateAllDnsRecords(sessionID, domain string, records []*DnsRecordInfo) error {
	params := struct {
		DomainName     string `json:"domainname"`
		CustomerNumber string `json:"customernumber"`
		APIKey         string `json:"apikey"`
		SessionID      string `json:"apisessionid"`
		DnsRecordSet   struct {
			DnsRecords []*DnsRecordInfo `json:"dnsrecords"`
		} `json:"dnsrecordset"`
	}{
		DomainName:     domain,
		CustomerNumber: c.customerID,
		APIKey:         c.apiKey,
		SessionID:      sessionID,
		DnsRecordSet: struct {
			DnsRecords []*DnsRecordInfo `json:"dnsrecords"`
		}{
			DnsRecords: records,
		},
	}

	request := NetcupAPIRequest{
		Action: "updateDnsRecords",
		Param:  params,
	}

	response, err := c.makeAPICall(request)
	if err != nil {
		return err
	}

	if response.Status != "success" {
		switch {
		case response.StatusCode == 4013:
			return fmt.Errorf("update DNS records failed: The DNS records are not in valid format. Check record type, hostname format, and destination value")
		case response.StatusCode == 2016:
			return fmt.Errorf("update DNS records failed: Domain not found or not accessible with current credentials")
		case response.StatusCode == 2057:
			return fmt.Errorf("update DNS records failed: Rate limit exceeded. Please wait and retry later")
		default:
			return fmt.Errorf("update DNS records failed: %s (status code: %d)", response.LongMessage, response.StatusCode)
		}
	}

	return nil
}
