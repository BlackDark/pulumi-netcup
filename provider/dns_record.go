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
	"context"
	"fmt"
	"strings"

	"github.com/pulumi/pulumi-go-provider/infer"
)

// DnsRecord represents a DNS record resource in Netcup
type DnsRecord struct{}

// Annotate provides metadata about the DnsRecord resource
func (r *DnsRecord) Annotate(a infer.Annotator) {
	a.Describe(&r, "A DNS record managed by Netcup DNS service")
}

type DnsRecordArgs struct {
	Domain   string  `pulumi:"domain"`
	Name     string  `pulumi:"name"`
	Type     string  `pulumi:"type"`
	Value    string  `pulumi:"value"`
	Priority *string `pulumi:"priority,optional"`
}

// Annotate provides metadata about the DnsRecordArgs
func (args *DnsRecordArgs) Annotate(a infer.Annotator) {
	a.Describe(&args.Domain, "The domain name for the DNS record")
	a.Describe(&args.Name, "The hostname for the DNS record (use '@' for root domain)")
	a.Describe(&args.Type, "The DNS record type (A, AAAA, CNAME, MX, TXT, etc.)")
	a.Describe(&args.Value, "The value/destination for the DNS record")
	a.Describe(&args.Priority, "The priority for MX and SRV records (optional for other types)")
}

type DnsRecordState struct {
	DnsRecordArgs
	RecordID string `pulumi:"recordId"`
	FQDN     string `pulumi:"fqdn"`
}

// Annotate provides metadata about the DnsRecordState
func (state *DnsRecordState) Annotate(a infer.Annotator) {
	a.Describe(&state.Domain, "The domain name for the DNS record")
	a.Describe(&state.Name, "The hostname for the DNS record")
	a.Describe(&state.Type, "The DNS record type")
	a.Describe(&state.Value, "The value/destination for the DNS record")
	a.Describe(&state.Priority, "The priority for the DNS record")
	a.Describe(&state.RecordID, "The unique identifier for the DNS record")
	a.Describe(&state.FQDN, "The fully qualified domain name")
}

func (r *DnsRecord) Create(
	ctx context.Context,
	req infer.CreateRequest[DnsRecordArgs],
) (infer.CreateResponse[DnsRecordState], error) {
	input := req.Inputs

	if req.DryRun {
		// Generate a temporary composite ID for preview
		tempID := createCompositeID(input.Domain, "preview-id")
		state := DnsRecordState{
			DnsRecordArgs: input,
			RecordID:      "preview-id",
			FQDN:          buildFQDN(input.Name, input.Domain),
		}
		return infer.CreateResponse[DnsRecordState]{ID: tempID, Output: state}, nil
	}

	if err := validateDnsRecord(input); err != nil {
		return infer.CreateResponse[DnsRecordState]{}, err
	}

	config := infer.GetConfig[Config](ctx)
	client := NewNetcupClient(config.ApiKey, config.ApiPassword, config.CustomerID)

	var priority string
	if input.Priority != nil {
		priority = *input.Priority
	}

	recordID, err := client.CreateDnsRecord(input.Domain, input.Name, input.Type, input.Value, priority)
	if err != nil {
		return infer.CreateResponse[DnsRecordState]{}, fmt.Errorf("failed to create DNS record: %w", err)
	}

	// Create composite ID for the resource
	compositeID := createCompositeID(input.Domain, recordID)

	state := DnsRecordState{
		DnsRecordArgs: input,
		RecordID:      recordID,
		FQDN:          buildFQDN(input.Name, input.Domain),
	}

	return infer.CreateResponse[DnsRecordState]{ID: compositeID, Output: state}, nil
}

func (r *DnsRecord) Read(
	ctx context.Context,
	req infer.ReadRequest[DnsRecordArgs, DnsRecordState],
) (infer.ReadResponse[DnsRecordArgs, DnsRecordState], error) {
	// Parse the composite ID to get domain and record ID
	domain, recordID, err := parseCompositeID(req.ID)
	if err != nil {
		return infer.ReadResponse[DnsRecordArgs, DnsRecordState]{},
			fmt.Errorf("invalid resource ID format: %w", err)
	}

	config := infer.GetConfig[Config](ctx)
	client := NewNetcupClient(config.ApiKey, config.ApiPassword, config.CustomerID)

	currentRecord, err := client.GetDnsRecordById(recordID, domain)
	if err != nil {
		// If record not found, return empty response to indicate resource should be recreated
		return infer.ReadResponse[DnsRecordArgs, DnsRecordState]{}, nil
	}

	// Convert priority to pointer (handling empty/zero values)
	var priority *string
	if currentRecord.Priority != "" && currentRecord.Priority != "0" {
		priority = &currentRecord.Priority
	}

	// Create inputs and state from current record data
	inputs := DnsRecordArgs{
		Domain:   domain,
		Name:     currentRecord.Hostname,
		Type:     currentRecord.Type,
		Value:    currentRecord.Destination,
		Priority: priority,
	}

	state := DnsRecordState{
		DnsRecordArgs: inputs,
		RecordID:      recordID,
		FQDN:          buildFQDN(currentRecord.Hostname, domain),
	}

	return infer.ReadResponse[DnsRecordArgs, DnsRecordState]{
		ID:     req.ID, // Return the same composite ID
		Inputs: inputs,
		State:  state,
	}, nil
}

func (r *DnsRecord) Update(
	ctx context.Context,
	req infer.UpdateRequest[DnsRecordArgs, DnsRecordState],
) (infer.UpdateResponse[DnsRecordState], error) {
	if req.DryRun {
		return infer.UpdateResponse[DnsRecordState]{}, nil
	}

	inputs := req.Inputs

	if err := validateDnsRecord(inputs); err != nil {
		return infer.UpdateResponse[DnsRecordState]{}, err
	}

	// Parse the composite ID to get domain and record ID
	domain, recordID, err := parseCompositeID(req.ID)
	if err != nil {
		return infer.UpdateResponse[DnsRecordState]{}, fmt.Errorf("invalid resource ID: %w", err)
	}

	config := infer.GetConfig[Config](ctx)
	client := NewNetcupClient(config.ApiKey, config.ApiPassword, config.CustomerID)

	// Verify the record exists before updating
	_, err = client.GetDnsRecordById(recordID, domain)
	if err != nil {
		return infer.UpdateResponse[DnsRecordState]{}, fmt.Errorf("failed to find existing DNS record for update: %w", err)
	}

	var priority string
	if inputs.Priority != nil {
		priority = *inputs.Priority
	}

	err = client.UpdateDnsRecord(recordID, domain, inputs.Name, inputs.Type, inputs.Value, priority)
	if err != nil {
		return infer.UpdateResponse[DnsRecordState]{}, fmt.Errorf("failed to update DNS record: %w", err)
	}

	newState := DnsRecordState{
		DnsRecordArgs: inputs,
		RecordID:      recordID,
		FQDN:          buildFQDN(inputs.Name, inputs.Domain),
	}

	return infer.UpdateResponse[DnsRecordState]{Output: newState}, nil
}

func (r *DnsRecord) Diff(
	ctx context.Context,
	req infer.DiffRequest[DnsRecordArgs, DnsRecordState],
) (infer.DiffResponse, error) {
	hasChanges := false
	deleteBeforeReplace := false

	// Check for changes that require replacement (domain, name, type)
	if req.Inputs.Domain != req.State.Domain {
		hasChanges = true
		deleteBeforeReplace = true
	}

	if req.Inputs.Name != req.State.Name {
		hasChanges = true
		deleteBeforeReplace = true
	}

	if req.Inputs.Type != req.State.Type {
		hasChanges = true
		deleteBeforeReplace = true
	}

	// Check for changes that can be updated in place
	if req.Inputs.Value != req.State.Value {
		hasChanges = true
	}

	// Handle priority comparison with normalization
	inputPriority := ""
	if req.Inputs.Priority != nil {
		inputPriority = *req.Inputs.Priority
	}
	statePriority := ""
	if req.State.Priority != nil {
		statePriority = *req.State.Priority
	}

	// Normalize priority values - treat "0" and "" as equivalent for non-priority record types
	if (inputPriority == "" && statePriority == "0") || (inputPriority == "0" && statePriority == "") {
		// These are equivalent - no change
	} else if inputPriority != statePriority {
		hasChanges = true
	}

	return infer.DiffResponse{
		DeleteBeforeReplace: deleteBeforeReplace,
		HasChanges:          hasChanges,
	}, nil
}

func (r *DnsRecord) Delete(
	ctx context.Context,
	req infer.DeleteRequest[DnsRecordState],
) (infer.DeleteResponse, error) {
	// Parse the composite ID to get domain and record ID
	domain, recordID, err := parseCompositeID(req.ID)
	if err != nil {
		return infer.DeleteResponse{}, fmt.Errorf("invalid resource ID: %w", err)
	}

	config := infer.GetConfig[Config](ctx)
	client := NewNetcupClient(config.ApiKey, config.ApiPassword, config.CustomerID)

	err = client.DeleteDnsRecord(recordID, domain)
	if err != nil {
		return infer.DeleteResponse{}, fmt.Errorf("failed to delete DNS record %s: %w", recordID, err)
	}

	return infer.DeleteResponse{}, nil
}

// Check validates and normalizes the resource inputs
func (r *DnsRecord) Check(ctx context.Context, req infer.CheckRequest) (infer.CheckResponse[DnsRecordArgs], error) {
	// Use default check for standard validation and type conversion
	args, failures, err := infer.DefaultCheck[DnsRecordArgs](ctx, req.NewInputs)
	if err != nil {
		return infer.CheckResponse[DnsRecordArgs]{
			Inputs:   args,
			Failures: failures,
		}, err
	}

	// Perform additional validation using the existing validateDnsRecord function
	if validationErr := validateDnsRecord(args); validationErr != nil {
		// Convert validation error to check failure
		// For now, we'll just return the error - the infer package will handle it
		return infer.CheckResponse[DnsRecordArgs]{
			Inputs:   args,
			Failures: failures,
		}, validationErr
	}

	return infer.CheckResponse[DnsRecordArgs]{
		Inputs:   args,
		Failures: failures,
	}, nil
}

func validateDnsRecord(args DnsRecordArgs) error {
	validTypes := map[string]bool{
		"A": true, "AAAA": true, "MX": true, "CNAME": true, "CAA": true, "SRV": true,
		"TXT": true, "TLSA": true, "NS": true, "DS": true, "OPENPGPKEY": true, "SMIMEA": true, "SSHFP": true,
	}

	if !validTypes[args.Type] {
		return fmt.Errorf("unsupported DNS record type: %s", args.Type)
	}

	if args.Domain == "" {
		return fmt.Errorf("domain is required")
	}
	if args.Name == "" {
		return fmt.Errorf("name is required")
	}
	if args.Value == "" {
		return fmt.Errorf("value is required")
	}

	switch args.Type {
	case "MX":
		if args.Priority == nil || *args.Priority == "" {
			return fmt.Errorf("priority is required for MX records")
		}
	case "SRV":
		if args.Priority == nil || *args.Priority == "" {
			return fmt.Errorf("priority is required for SRV records")
		}
	case "CNAME":
		if args.Name == "@" {
			return fmt.Errorf("CNAME records cannot be created for the root domain (@)")
		}
	}

	return nil
}

// WireDependencies defines the dependency relationships between inputs and outputs
func (r *DnsRecord) WireDependencies(f infer.FieldSelector, args *DnsRecordArgs, state *DnsRecordState) {
	f.OutputField(&state.Domain).DependsOn(f.InputField(&args.Domain))
	f.OutputField(&state.Name).DependsOn(f.InputField(&args.Name))
	f.OutputField(&state.Type).DependsOn(f.InputField(&args.Type))
	f.OutputField(&state.Value).DependsOn(f.InputField(&args.Value))
	f.OutputField(&state.Priority).DependsOn(f.InputField(&args.Priority))
	f.OutputField(&state.FQDN).DependsOn(f.InputField(&args.Name), f.InputField(&args.Domain))
}

// createCompositeID creates a composite ID in the format "domain:recordID"
func createCompositeID(domain, recordID string) string {
	return fmt.Sprintf("%s:%s", domain, recordID)
}

// parseCompositeID parses a composite ID and returns domain and recordID
func parseCompositeID(compositeID string) (domain, recordID string, err error) {
	if !strings.Contains(compositeID, ":") {
		return "", "", fmt.Errorf("composite ID must be in format 'domain:recordID', got: %s", compositeID)
	}

	parts := strings.SplitN(compositeID, ":", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("invalid composite ID format: %s", compositeID)
	}

	return parts[0], parts[1], nil
}

func buildFQDN(name, domain string) string {
	if name == "@" || name == "" {
		return domain
	}
	return fmt.Sprintf("%s.%s", name, domain)
}
