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
	Name         string // Computed field
	Value        string // Computed field
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

// AuthenticatedRequest represents requests that need authentication
type AuthenticatedRequest struct {
	CustomerNumber string `json:"customernumber"`
	APIKey         string `json:"apikey"`
	SessionID      string `json:"apisessionid"`
	DomainName     string `json:"domainname"`
	DNSRecords     []*DnsRecordInfo `json:"dnsrecords,omitempty"`
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
func (c *NetcupClient) CreateDnsRecord(domain, name, recordType, value, priority string) (string, error) {
	sessionID, err := c.login()
	if err != nil {
		return "", err
	}
	defer c.logout(sessionID)

	// Create new record (Netcup requires updating the complete record set)
	newRecord := &DnsRecordInfo{
		Hostname:    name,
		Type:        recordType,
		Destination: value,
	}

	if priority != "" {
		newRecord.Priority = priority
	}

	params := AuthenticatedRequest{
		CustomerNumber: c.customerID,
		APIKey:         c.apiKey,
		SessionID:      sessionID,
		DomainName:     domain,
		DNSRecords:     []*DnsRecordInfo{newRecord},
	}

	request := NetcupAPIRequest{
		Action: "updateDnsRecords",
		Param:  params,
	}

	response, err := c.makeAPICall(request)
	if err != nil {
		return "", err
	}

	if response.Status != "success" {
		return "", fmt.Errorf("create DNS record failed: %s", response.LongMessage)
	}

	// Parse response to get the actual record ID
	responseData, ok := response.ResponseData.(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("invalid create response format")
	}

	recordsData, ok := responseData["dnsrecords"].([]interface{})
	if !ok {
		return "", fmt.Errorf("no DNS records found in create response")
	}

	// Find our created record by matching hostname, type, and destination
	for _, recordData := range recordsData {
		recordMap, ok := recordData.(map[string]interface{})
		if !ok {
			continue
		}

		hostname, hostnameOk := recordMap["hostname"].(string)
		recType, typeOk := recordMap["type"].(string)
		destination, destOk := recordMap["destination"].(string)
		
		if hostnameOk && typeOk && destOk {
			if hostname == name && recType == recordType && destination == value {
				if id, ok := recordMap["id"].(string); ok {
					return id, nil
				}
			}
		}
	}

	return "", fmt.Errorf("could not retrieve created record ID")
}


// DeleteDnsRecord removes a DNS record
func (c *NetcupClient) DeleteDnsRecord(recordID, domain string) error {
	sessionID, err := c.login()
	if err != nil {
		return err
	}
	defer c.logout(sessionID)

	// Get the record to delete
	record, err := c.GetDnsRecordById(recordID, domain)
	if err != nil {
		return err
	}

	// Mark record for deletion
	record.DeleteRecord = true

	params := AuthenticatedRequest{
		CustomerNumber: c.customerID,
		APIKey:         c.apiKey,
		SessionID:      sessionID,
		DomainName:     domain,
		DNSRecords:     []*DnsRecordInfo{record},
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
		return fmt.Errorf("delete DNS record failed: %s", response.LongMessage)
	}

	return nil
}

// GetDnsRecordById retrieves information about a specific DNS record by ID
func (c *NetcupClient) GetDnsRecordById(recordID, domain string) (*DnsRecordInfo, error) {
	sessionID, err := c.login()
	if err != nil {
		return nil, err
	}
	defer c.logout(sessionID)

	params := AuthenticatedRequest{
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

	for _, recordData := range recordsData {
		recordMap, ok := recordData.(map[string]interface{})
		if !ok {
			continue
		}

		if id, ok := recordMap["id"].(string); ok && id == recordID {
			record := &DnsRecordInfo{ID: id}
			if hostname, ok := recordMap["hostname"].(string); ok {
				record.Hostname = hostname
				record.Name = hostname
			}
			if recordType, ok := recordMap["type"].(string); ok {
				record.Type = recordType
			}
			if destination, ok := recordMap["destination"].(string); ok {
				record.Destination = destination
				record.Value = destination
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

