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

	"github.com/pulumi/pulumi-go-provider/infer"
)

// DnsRecord is the controller for the DNS record resource
type DnsRecord struct{}

// DnsRecordArgs are the inputs to the DNS record resource's constructor
type DnsRecordArgs struct {
	// Domain name for the DNS record
	Domain string `pulumi:"domain"`
	// Record name (subdomain or "@" for root)
	Name string `pulumi:"name"`
	// DNS record type (A, AAAA, MX, CNAME, CAA, SRV, TXT, TLSA, NS, DS, OPENPGPKEY, SMIMEA, SSHFP)
	Type string `pulumi:"type"`
	// Record value/destination
	Value string `pulumi:"value"`
	// Priority for MX/SRV records (optional)
	Priority *int `pulumi:"priority,optional"`
	// Additional parameters for complex record types (optional)
	Port     *int    `pulumi:"port,optional"`
	Weight   *int    `pulumi:"weight,optional"`
	Protocol *string `pulumi:"protocol,optional"`
	Service  *string `pulumi:"service,optional"`
}

// DnsRecordState is what's persisted in state
type DnsRecordState struct {
	DnsRecordArgs
	// The unique identifier returned by Netcup API
	RecordID string `pulumi:"recordId"`
	// Full qualified domain name
	FQDN string `pulumi:"fqdn"`
}

// Create creates a new DNS record
func (r *DnsRecord) Create(
	ctx context.Context,
	req infer.CreateRequest[DnsRecordArgs],
) (infer.CreateResponse[DnsRecordState], error) {
	name := req.Name
	input := req.Inputs
	preview := req.DryRun

	state := DnsRecordState{
		DnsRecordArgs: input,
		FQDN:         buildFQDN(input.Name, input.Domain),
	}

	if preview {
		state.RecordID = "preview-id"
		return infer.CreateResponse[DnsRecordState]{ID: name, Output: state}, nil
	}

	// Get provider config
	config := infer.GetConfig[Config](ctx)
	client := NewNetcupClient(config.ApiKey, config.ApiPassword, config.CustomerID)

	// Validate record type and parameters
	if err := validateDnsRecord(input); err != nil {
		return infer.CreateResponse[DnsRecordState]{}, err
	}

	// Create DNS record via Netcup API
	recordID, err := client.CreateDnsRecord(input.Domain, input.Name, input.Type, input.Value, input.Priority, input.Port, input.Weight, input.Protocol, input.Service)
	if err != nil {
		return infer.CreateResponse[DnsRecordState]{}, fmt.Errorf("failed to create DNS record: %w", err)
	}

	state.RecordID = recordID
	return infer.CreateResponse[DnsRecordState]{ID: name, Output: state}, nil
}


// Delete removes a DNS record
func (r *DnsRecord) Delete(
	ctx context.Context,
	req infer.DeleteRequest[DnsRecordState],
) error {
	state := req.State

	// Get provider config
	config := infer.GetConfig[Config](ctx)
	client := NewNetcupClient(config.ApiKey, config.ApiPassword, config.CustomerID)

	// Delete DNS record via Netcup API
	err := client.DeleteDnsRecord(state.RecordID, state.Domain)
	if err != nil {
		return fmt.Errorf("failed to delete DNS record: %w", err)
	}

	return nil
}


// validateDnsRecord validates DNS record parameters based on type
func validateDnsRecord(args DnsRecordArgs) error {
	validTypes := map[string]bool{
		"A": true, "AAAA": true, "MX": true, "CNAME": true, "CAA": true, "SRV": true,
		"TXT": true, "TLSA": true, "NS": true, "DS": true, "OPENPGPKEY": true, "SMIMEA": true, "SSHFP": true,
	}

	if !validTypes[args.Type] {
		return fmt.Errorf("unsupported DNS record type: %s", args.Type)
	}

	// Validate required fields for specific record types
	switch args.Type {
	case "MX":
		if args.Priority == nil {
			return fmt.Errorf("priority is required for MX records")
		}
	case "SRV":
		if args.Priority == nil || args.Weight == nil || args.Port == nil {
			return fmt.Errorf("priority, weight, and port are required for SRV records")
		}
	}

	return nil
}

// buildFQDN constructs the fully qualified domain name
func buildFQDN(name, domain string) string {
	if name == "@" || name == "" {
		return domain
	}
	return fmt.Sprintf("%s.%s", name, domain)
}