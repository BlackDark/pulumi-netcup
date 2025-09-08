// Copyright 2025, BlackDark.
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

	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/infer"
)

// DNSRecord represents a DNS record managed by Netcup DNS service.
type DNSRecord struct{}

// Annotate provides metadata about the DNSRecord resource.
func (r *DNSRecord) Annotate(a infer.Annotator) {
	a.Describe(&r, "A DNS record managed by Netcup DNS service")
}

// DNSRecordArgs contains the input arguments for a DNS record resource.
type DNSRecordArgs struct {
	Domain   string  `pulumi:"domain"`
	Name     string  `pulumi:"name"`
	Type     string  `pulumi:"type"`
	Value    string  `pulumi:"value"`
	Priority *string `pulumi:"priority,optional"`
}

// Annotate provides metadata about the DNSRecordArgs.
func (args *DNSRecordArgs) Annotate(a infer.Annotator) {
	a.Describe(&args.Domain, "The domain name for the DNS record (e.g., 'example.com')")
	a.Describe(
		&args.Name,
		"The hostname for the DNS record. Use '@' for root domain, or specify subdomain (e.g., 'www', 'mail')",
	)
	a.Describe(
		&args.Type,
		"The DNS record type. Supported types: A, AAAA, CNAME, MX, TXT, SRV, CAA, TLSA, NS, DS, OPENPGPKEY, SMIMEA, SSHFP",
	)
	a.Describe(
		&args.Value,
		"The value/destination for the DNS record (e.g., IP address for A records, hostname for CNAME)",
	)
	a.Describe(&args.Priority, "The priority for MX and SRV records (required for these types, ignored for others)")
}

// DNSRecordState contains the state of a DNS record resource.
type DNSRecordState struct {
	DNSRecordArgs
	RecordID string `pulumi:"recordId"`
	FQDN     string `pulumi:"fqdn"`
}

// Annotate provides metadata about the DNSRecordState.
func (state *DNSRecordState) Annotate(a infer.Annotator) {
	a.Describe(&state.Domain, "The domain name for the DNS record")
	a.Describe(&state.Name, "The hostname for the DNS record")
	a.Describe(&state.Type, "The DNS record type")
	a.Describe(&state.Value, "The value/destination for the DNS record")
	a.Describe(&state.Priority, "The priority for the DNS record")
	a.Describe(&state.RecordID, "The unique identifier for the DNS record")
	a.Describe(&state.FQDN, "The fully qualified domain name")
}

// Create creates a new DNS record resource in Netcup.
func (r *DNSRecord) Create(
	ctx context.Context,
	req infer.CreateRequest[DNSRecordArgs],
) (infer.CreateResponse[DNSRecordState], error) {
	input := req.Inputs

	if req.DryRun {
		// Generate a temporary composite ID for preview
		tempID := createCompositeID(input.Domain, "preview-id")
		state := DNSRecordState{
			DNSRecordArgs: input,
			RecordID:      "preview-id",
			FQDN:          buildFQDN(input.Name, input.Domain),
		}
		return infer.CreateResponse[DNSRecordState]{ID: tempID, Output: state}, nil
	}

	// Additional validation (should have been caught in Check, but double-check)
	if failures := validateDNSRecordWithFailures(input); len(failures) > 0 {
		return infer.CreateResponse[DNSRecordState]{},
			fmt.Errorf("validation failed: %v", failures)
	}

	config := infer.GetConfig[Config](ctx)
	client := NewNetcupClient(config.APIKey, config.APIPassword, config.CustomerID)

	var priority string
	if input.Priority != nil {
		priority = *input.Priority
	}

	recordID, err := client.CreateDNSRecord(input.Domain, input.Name, input.Type, input.Value, priority)
	if err != nil {
		return infer.CreateResponse[DNSRecordState]{}, fmt.Errorf("failed to create DNS record: %w", err)
	}

	// Create composite ID for the resource
	compositeID := createCompositeID(input.Domain, recordID)

	state := DNSRecordState{
		DNSRecordArgs: input,
		RecordID:      recordID,
		FQDN:          buildFQDN(input.Name, input.Domain),
	}

	return infer.CreateResponse[DNSRecordState]{ID: compositeID, Output: state}, nil
}

func (r *DNSRecord) Read(
	// Read reads the current state of a DNS record resource in Netcup.
	ctx context.Context,
	req infer.ReadRequest[DNSRecordArgs, DNSRecordState],
) (infer.ReadResponse[DNSRecordArgs, DNSRecordState], error) {
	// Parse the composite ID to get domain and record ID
	domain, recordID, err := parseCompositeID(req.ID)
	if err != nil {
		return infer.ReadResponse[DNSRecordArgs, DNSRecordState]{},
			fmt.Errorf("invalid resource ID format: %w", err)
	}

	config := infer.GetConfig[Config](ctx)
	client := NewNetcupClient(config.APIKey, config.APIPassword, config.CustomerID)

	currentRecord, err := client.GetDNSRecordByID(recordID, domain)
	if err != nil {
		// Check if this is a "not found" error specifically
		if isNotFoundError(err) {
			// Return empty response to indicate resource should be recreated
			return infer.ReadResponse[DNSRecordArgs, DNSRecordState]{}, nil
		}
		// For other errors, return the error to indicate a real problem
		return infer.ReadResponse[DNSRecordArgs, DNSRecordState]{},
			fmt.Errorf("failed to read DNS record %s: %w", recordID, err)
	}

	// Convert priority to pointer (handling empty/zero values)
	var priority *string
	if currentRecord.Priority != "" && currentRecord.Priority != "0" {
		priority = &currentRecord.Priority
	}

	// Create inputs and state from current record data
	inputs := DNSRecordArgs{
		Domain:   domain,
		Name:     currentRecord.Hostname,
		Type:     currentRecord.Type,
		Value:    currentRecord.Destination,
		Priority: priority,
	}

	state := DNSRecordState{
		DNSRecordArgs: inputs,
		RecordID:      recordID,
		FQDN:          buildFQDN(currentRecord.Hostname, domain),
	}

	return infer.ReadResponse[DNSRecordArgs, DNSRecordState]{
		ID:     req.ID, // Return the same composite ID
		Inputs: inputs,
		State:  state,
	}, nil
}

// Update updates an existing DNS record resource in Netcup.
func (r *DNSRecord) Update(
	ctx context.Context,
	req infer.UpdateRequest[DNSRecordArgs, DNSRecordState],
) (
	infer.UpdateResponse[DNSRecordState],
	error,
) {
	inputs := req.Inputs

	// Validate inputs (should have been caught in Check, but double-check)
	if failures := validateDNSRecordWithFailures(inputs); len(failures) > 0 {
		return infer.UpdateResponse[DNSRecordState]{},
			fmt.Errorf("validation failed: %v", failures)
	}

	// Parse the composite ID to get domain and record ID
	domain, recordID, err := parseCompositeID(req.ID)
	if err != nil {
		return infer.UpdateResponse[DNSRecordState]{}, fmt.Errorf("invalid resource ID: %w", err)
	}
	config := infer.GetConfig[Config](ctx)
	client := NewNetcupClient(config.APIKey, config.APIPassword, config.CustomerID)

	// Verify the record exists before updating
	_, err = client.GetDNSRecordByID(recordID, domain)
	if err != nil {
		return infer.UpdateResponse[DNSRecordState]{},
			fmt.Errorf("failed to find existing DNS record for update: %w", err)
	}

	var priority string
	if inputs.Priority != nil {
		priority = *inputs.Priority
	}

	err = client.UpdateDNSRecord(recordID, domain, inputs.Name, inputs.Type, inputs.Value, priority)
	if err != nil {
		return infer.UpdateResponse[DNSRecordState]{}, fmt.Errorf("failed to update DNS record: %w", err)
	}
	newState := DNSRecordState{
		DNSRecordArgs: inputs,
		RecordID:      recordID,
		FQDN:          buildFQDN(inputs.Name, inputs.Domain),
	}

	return infer.UpdateResponse[DNSRecordState]{Output: newState}, nil
}

// Diff computes the differences between the desired and current state of a DNS record resource.
func (r *DNSRecord) Diff(
	ctx context.Context,
	req infer.DiffRequest[DNSRecordArgs, DNSRecordState],
) (
	infer.DiffResponse,
	error,
) {
	hasChanges := false
	deleteBeforeReplace := false
	detailedDiff := make(map[string]p.PropertyDiff)

	// Check for changes that require replacement (domain, name, type)
	if req.Inputs.Domain != req.State.Domain {
		hasChanges = true
		deleteBeforeReplace = true
		detailedDiff["domain"] = p.PropertyDiff{
			Kind:      p.UpdateReplace,
			InputDiff: true,
		}
	}

	if req.Inputs.Name != req.State.Name {
		hasChanges = true
		deleteBeforeReplace = true
		detailedDiff["name"] = p.PropertyDiff{
			Kind:      p.UpdateReplace,
			InputDiff: true,
		}
	}

	if req.Inputs.Type != req.State.Type {
		hasChanges = true
		deleteBeforeReplace = true
		detailedDiff["type"] = p.PropertyDiff{
			Kind:      p.UpdateReplace,
			InputDiff: true,
		}
	}

	// Check for changes that can be updated in place
	if req.Inputs.Value != req.State.Value {
		hasChanges = true
		detailedDiff["value"] = p.PropertyDiff{
			Kind:      p.Update,
			InputDiff: true,
		}
	}

	// Handle priority comparison with normalization
	if priorityChanged(req.Inputs.Priority, req.State.Priority, req.Inputs.Type) {
		hasChanges = true
		detailedDiff["priority"] = p.PropertyDiff{
			Kind:      p.Update,
			InputDiff: true,
		}
	}

	// Add computed field diffs
	newFQDN := buildFQDN(req.Inputs.Name, req.Inputs.Domain)
	if newFQDN != req.State.FQDN {
		detailedDiff["fqdn"] = p.PropertyDiff{
			Kind:      p.Update,
			InputDiff: false, // This is a computed field, not an input
		}
	}

	return infer.DiffResponse{
		DeleteBeforeReplace: deleteBeforeReplace,
		HasChanges:          hasChanges,
		DetailedDiff:        detailedDiff,
	}, nil
}

// Delete deletes a DNS record resource in Netcup.
func (r *DNSRecord) Delete(ctx context.Context, req infer.DeleteRequest[DNSRecordState]) (infer.DeleteResponse, error) {
	// Parse the composite ID to get domain and record ID
	domain, recordID, err := parseCompositeID(req.ID)
	if err != nil {
		return infer.DeleteResponse{}, fmt.Errorf("invalid resource ID: %w", err)
	}

	config := infer.GetConfig[Config](ctx)
	client := NewNetcupClient(config.APIKey, config.APIPassword, config.CustomerID)

	err = client.DeleteDNSRecord(recordID, domain)
	if err != nil {
		return infer.DeleteResponse{}, fmt.Errorf("failed to delete DNS record %s: %w", recordID, err)
	}

	return infer.DeleteResponse{}, nil
}

// Check validates and normalizes the resource inputs.
func (r *DNSRecord) Check(ctx context.Context, req infer.CheckRequest) (infer.CheckResponse[DNSRecordArgs], error) {
	// Use default check for standard validation and type conversion
	args, failures, err := infer.DefaultCheck[DNSRecordArgs](ctx, req.NewInputs)
	if err != nil {
		return infer.CheckResponse[DNSRecordArgs]{
			Inputs:   args,
			Failures: failures,
		}, err
	}

	// Normalize inputs
	args = normalizeInputs(args)

	// Add custom validation failures
	additionalFailures := validateDNSRecordWithFailures(args)
	failures = append(failures, additionalFailures...)

	return infer.CheckResponse[DNSRecordArgs]{
		Inputs:   args,
		Failures: failures,
	}, nil
}

// WireDependencies defines the dependency relationships between inputs and outputs.
func (r *DNSRecord) WireDependencies(f infer.FieldSelector, args *DNSRecordArgs, state *DNSRecordState) {
	f.OutputField(&state.Domain).DependsOn(f.InputField(&args.Domain))
	f.OutputField(&state.Name).DependsOn(f.InputField(&args.Name))
	f.OutputField(&state.Type).DependsOn(f.InputField(&args.Type))
	f.OutputField(&state.Value).DependsOn(f.InputField(&args.Value))
	f.OutputField(&state.Priority).DependsOn(f.InputField(&args.Priority))
	f.OutputField(&state.FQDN).DependsOn(f.InputField(&args.Name), f.InputField(&args.Domain))
}

// normalizeInputs normalizes and cleans up input values
func normalizeInputs(args DNSRecordArgs) DNSRecordArgs {
	// Normalize DNS record type to uppercase
	args.Type = strings.ToUpper(strings.TrimSpace(args.Type))

	// Normalize domain to lowercase
	args.Domain = strings.ToLower(strings.TrimSpace(args.Domain))

	// Normalize name
	args.Name = sanitizeRecordName(args.Name)

	// Normalize value
	args.Value = strings.TrimSpace(args.Value)

	// Normalize priority if present
	if args.Priority != nil {
		priority := strings.TrimSpace(*args.Priority)
		args.Priority = &priority
	}

	return args
}

// validateDnsRecordWithFailures performs validation and returns field-specific failures
func validateDNSRecordWithFailures(args DNSRecordArgs) []p.CheckFailure {
	var failures []p.CheckFailure

	validTypes := getValidTypesMap()

	if !validTypes[strings.ToUpper(args.Type)] {
		failures = append(failures, p.CheckFailure{
			Property: "type",
			Reason: fmt.Sprintf(
				"Unsupported DNS record type: %s. Valid types are: %v",
				args.Type,
				getValidTypesList(),
			),
		})
	}

	if args.Domain == "" {
		failures = append(failures, p.CheckFailure{
			Property: "domain",
			Reason:   "Domain is required",
		})
	} else if !isValidDomain(args.Domain) {
		failures = append(failures, p.CheckFailure{
			Property: "domain",
			Reason:   "Domain format is invalid",
		})
	}

	if args.Name == "" {
		failures = append(failures, p.CheckFailure{
			Property: "name",
			Reason:   "Name is required",
		})
	}

	if args.Value == "" {
		failures = append(failures, p.CheckFailure{
			Property: "value",
			Reason:   "Value is required",
		})
	}

	// Type-specific validations
	normalizedType := strings.ToUpper(args.Type)
	switch normalizedType {
	case "MX", "SRV":
		if args.Priority == nil || *args.Priority == "" {
			failures = append(failures, p.CheckFailure{
				Property: "priority",
				Reason:   fmt.Sprintf("Priority is required for %s records", normalizedType),
			})
		}
	case "CNAME":
		if args.Name == "@" {
			failures = append(failures, p.CheckFailure{
				Property: "name",
				Reason:   "CNAME records cannot be created for the root domain (@)",
			})
		}
	}

	return failures
}

// Legacy validation function for backward compatibility
func validateDNSRecord(args DNSRecordArgs) error {
	failures := validateDNSRecordWithFailures(args)
	if len(failures) > 0 {
		var messages []string
		for _, failure := range failures {
			messages = append(messages, fmt.Sprintf("%s: %s", failure.Property, failure.Reason))
		}
		return fmt.Errorf("validation failed: %s", strings.Join(messages, "; "))
	}
	return nil
}

// Helper functions
func getValidTypesMap() map[string]bool {
	return map[string]bool{
		"A": true, "AAAA": true, "MX": true, "CNAME": true, "CAA": true, "SRV": true,
		"TXT": true, "TLSA": true, "NS": true, "DS": true, "OPENPGPKEY": true, "SMIMEA": true, "SSHFP": true,
	}
}

func getValidTypesList() []string {
	return []string{
		"A",
		"AAAA",
		"MX",
		"CNAME",
		"CAA",
		"SRV",
		"TXT",
		"TLSA",
		"NS",
		"DS",
		"OPENPGPKEY",
		"SMIMEA",
		"SSHFP",
	}
}

func priorityChanged(inputPriority, statePriority *string, recordType string) bool {
	inputVal := ""
	if inputPriority != nil {
		inputVal = *inputPriority
	}

	stateVal := ""
	if statePriority != nil {
		stateVal = *statePriority
	}

	// For record types that don't use priority, normalize empty and "0" values
	if !requiresPriority(recordType) {
		inputNormalized := inputVal == "" || inputVal == "0"
		stateNormalized := stateVal == "" || stateVal == "0"
		return inputNormalized != stateNormalized && inputVal != stateVal
	}

	return inputVal != stateVal
}

func requiresPriority(recordType string) bool {
	switch strings.ToUpper(recordType) {
	case "MX", "SRV":
		return true
	default:
		return false
	}
}

func isValidDomain(domain string) bool {
	if domain == "" {
		return false
	}
	// Basic domain validation - can be enhanced as needed
	return !strings.Contains(domain, " ") && strings.Contains(domain, ".")
}

func sanitizeRecordName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return "@"
	}
	return name
}

func isNotFoundError(err error) bool {
	if err == nil {
		return false
	}

	errMsg := strings.ToLower(err.Error())

	// Check for various "not found" indicators from the Netcup API
	return strings.Contains(errMsg, "dns record not found") ||
		strings.Contains(errMsg, "not found") ||
		strings.Contains(errMsg, "record not found") ||
		strings.Contains(errMsg, "domain not found") ||
		// Check for specific Netcup status codes that indicate "not found"
		strings.Contains(errMsg, "status code: 2016") // Domain not found
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

// GetID returns the resource ID in a consistent format.
func (r *DNSRecord) GetID(state DNSRecordState) string {
	return createCompositeID(state.Domain, state.RecordID)
}
