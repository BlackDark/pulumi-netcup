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
	"strconv"
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
	Action         string      `json:"action"`
	Param          interface{} `json:"param"`
	APIKey         string      `json:"apikey"`
	APIPassword    string      `json:"apisessionid"`
	CustomerNumber string      `json:"customernumber"`
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
	ID           string  `json:"id"`
	Hostname     string  `json:"hostname"`
	Type         string  `json:"type"`
	Priority     *int    `json:"priority,omitempty"`
	Destination  string  `json:"destination"`
	DeleteRecord bool    `json:"deleterecord,omitempty"`
	State        string  `json:"state,omitempty"`
	Port         *int    `json:"port,omitempty"`
	Weight       *int    `json:"weight,omitempty"`
	Name         string  // Computed field
	Value        string  // Computed field
}

// LoginParams represents login parameters
type LoginParams struct {
	CustomerNumber string `json:"customernumber"`
	APIKey         string `json:"apikey"`
	APIPassword    string `json:"apipassword"`
}

// DnsRecordParams represents parameters for DNS record operations
type DnsRecordParams struct {
	DomainName string           `json:"domainname"`
	DNSRecords []*DnsRecordInfo `json:"dnsrecords,omitempty"`
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

// login authenticates with the Netcup API and returns a session ID
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
		return "", fmt.Errorf("login failed: %s", response.LongMessage)
	}

	sessionData, ok := response.ResponseData.(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("invalid login response format")
	}

	sessionID, ok := sessionData["apisessionid"].(string)
	if !ok {
		return "", fmt.Errorf("no session ID returned")
	}

	return sessionID, nil
}

// logout ends the API session
func (c *NetcupClient) logout(sessionID string) error {
	request := NetcupAPIRequest{
		Action:         "logout",
		APIKey:         c.apiKey,
		APIPassword:    sessionID,
		CustomerNumber: c.customerID,
	}

	_, err := c.makeAPICall(request)
	return err
}

// makeAPICall performs an HTTP request to the Netcup API
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

// CreateDnsRecord creates a new DNS record
func (c *NetcupClient) CreateDnsRecord(domain, name, recordType, value string, priority, port, weight *int, protocol, service *string) (string, error) {
	sessionID, err := c.login()
	if err != nil {
		return "", err
	}
	defer c.logout(sessionID)

	// Get existing records first
	existingRecords, err := c.getDnsRecords(sessionID, domain)
	if err != nil {
		return "", err
	}

	// Create new record
	newRecord := &DnsRecordInfo{
		Hostname:    name,
		Type:        recordType,
		Destination: value,
	}

	if priority != nil {
		newRecord.Priority = priority
	}
	if port != nil {
		newRecord.Port = port
	}
	if weight != nil {
		newRecord.Weight = weight
	}

	// Add new record to existing records
	allRecords := append(existingRecords, newRecord)

	params := DnsRecordParams{
		DomainName: domain,
		DNSRecords: allRecords,
	}

	request := NetcupAPIRequest{
		Action:         "updateDnsRecords",
		Param:          params,
		APIKey:         c.apiKey,
		APIPassword:    sessionID,
		CustomerNumber: c.customerID,
	}

	response, err := c.makeAPICall(request)
	if err != nil {
		return "", err
	}

	if response.Status != "success" {
		return "", fmt.Errorf("create DNS record failed: %s", response.LongMessage)
	}

	// Return a generated ID (Netcup doesn't return specific record IDs)
	return fmt.Sprintf("%s_%s_%s", domain, name, recordType), nil
}

// UpdateDnsRecord updates an existing DNS record
func (c *NetcupClient) UpdateDnsRecord(recordID, domain, name, recordType, value string, priority, port, weight *int, protocol, service *string) error {
	sessionID, err := c.login()
	if err != nil {
		return err
	}
	defer c.logout(sessionID)

	// Get existing records
	existingRecords, err := c.getDnsRecords(sessionID, domain)
	if err != nil {
		return err
	}

	// Find and update the specific record
	found := false
	for _, record := range existingRecords {
		if record.ID == recordID || (record.Hostname == name && record.Type == recordType) {
			record.Hostname = name
			record.Type = recordType
			record.Destination = value
			if priority != nil {
				record.Priority = priority
			}
			if port != nil {
				record.Port = port
			}
			if weight != nil {
				record.Weight = weight
			}
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("DNS record not found: %s", recordID)
	}

	params := DnsRecordParams{
		DomainName: domain,
		DNSRecords: existingRecords,
	}

	request := NetcupAPIRequest{
		Action:         "updateDnsRecords",
		Param:          params,
		APIKey:         c.apiKey,
		APIPassword:    sessionID,
		CustomerNumber: c.customerID,
	}

	response, err := c.makeAPICall(request)
	if err != nil {
		return err
	}

	if response.Status != "success" {
		return fmt.Errorf("update DNS record failed: %s", response.LongMessage)
	}

	return nil
}

// DeleteDnsRecord removes a DNS record
func (c *NetcupClient) DeleteDnsRecord(recordID, domain string) error {
	sessionID, err := c.login()
	if err != nil {
		return err
	}
	defer c.logout(sessionID)

	// Get existing records
	existingRecords, err := c.getDnsRecords(sessionID, domain)
	if err != nil {
		return err
	}

	// Find and mark record for deletion
	found := false
	for _, record := range existingRecords {
		if record.ID == recordID {
			record.DeleteRecord = true
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("DNS record not found: %s", recordID)
	}

	params := DnsRecordParams{
		DomainName: domain,
		DNSRecords: existingRecords,
	}

	request := NetcupAPIRequest{
		Action:         "updateDnsRecords",
		Param:          params,
		APIKey:         c.apiKey,
		APIPassword:    sessionID,
		CustomerNumber: c.customerID,
	}

	response, err := c.makeAPICall(request)
	if err != nil {
		return err
	}

	if response.Status != "success" {
		return fmt.Errorf("delete DNS record failed: %s", response.LongMessage)
	}

	return nil
}

// GetDnsRecord retrieves information about a specific DNS record
func (c *NetcupClient) GetDnsRecord(recordID, domain string) (*DnsRecordInfo, error) {
	sessionID, err := c.login()
	if err != nil {
		return nil, err
	}
	defer c.logout(sessionID)

	records, err := c.getDnsRecords(sessionID, domain)
	if err != nil {
		return nil, err
	}

	for _, record := range records {
		if record.ID == recordID {
			// Populate computed fields
			record.Name = record.Hostname
			record.Value = record.Destination
			return record, nil
		}
	}

	return nil, fmt.Errorf("DNS record not found: %s", recordID)
}

// getDnsRecords retrieves all DNS records for a domain
func (c *NetcupClient) getDnsRecords(sessionID, domain string) ([]*DnsRecordInfo, error) {
	params := DnsRecordParams{
		DomainName: domain,
	}

	request := NetcupAPIRequest{
		Action:         "infoDnsRecords",
		Param:          params,
		APIKey:         c.apiKey,
		APIPassword:    sessionID,
		CustomerNumber: c.customerID,
	}

	response, err := c.makeAPICall(request)
	if err != nil {
		return nil, err
	}

	if response.Status != "success" {
		return nil, fmt.Errorf("get DNS records failed: %s", response.LongMessage)
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

		record := &DnsRecordInfo{}
		if id, ok := recordMap["id"].(string); ok {
			record.ID = id
		}
		if hostname, ok := recordMap["hostname"].(string); ok {
			record.Hostname = hostname
		}
		if recordType, ok := recordMap["type"].(string); ok {
			record.Type = recordType
		}
		if destination, ok := recordMap["destination"].(string); ok {
			record.Destination = destination
		}
		if priority, ok := recordMap["priority"].(string); ok {
			if p, err := strconv.Atoi(priority); err == nil {
				record.Priority = &p
			}
		}

		records = append(records, record)
	}

	return records, nil
}