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

type DnsRecord struct{}

type DnsRecordArgs struct {
	Domain   string  `pulumi:"domain"`
	Name     string  `pulumi:"name"`
	Type     string  `pulumi:"type"`
	Value    string  `pulumi:"value"`
	Priority *string `pulumi:"priority,optional"`
}

type DnsRecordState struct {
	DnsRecordArgs
	RecordID string `pulumi:"recordId"`
	FQDN     string `pulumi:"fqdn"`
}

func (r *DnsRecord) Create(
	ctx context.Context,
	req infer.CreateRequest[DnsRecordArgs],
) (infer.CreateResponse[DnsRecordState], error) {
	input := req.Inputs
	state := DnsRecordState{
		DnsRecordArgs: input,
		FQDN:          buildFQDN(input.Name, input.Domain),
	}

	if req.DryRun {
		state.RecordID = "preview-id"
		return infer.CreateResponse[DnsRecordState]{ID: req.Name, Output: state}, nil
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

	state.RecordID = recordID
	return infer.CreateResponse[DnsRecordState]{ID: req.Name, Output: state}, nil
}

func (r *DnsRecord) Read(
	ctx context.Context,
	req infer.ReadRequest[DnsRecordArgs, DnsRecordState],
) (infer.ReadResponse[DnsRecordArgs, DnsRecordState], error) {
	state := req.State

	if state.Domain == "" || state.Name == "" || state.Type == "" || state.Value == "" {
		return infer.ReadResponse[DnsRecordArgs, DnsRecordState]{}, nil
	}

	config := infer.GetConfig[Config](ctx)
	client := NewNetcupClient(config.ApiKey, config.ApiPassword, config.CustomerID)

	currentRecord, err := client.GetDnsRecordById(state.RecordID, state.Domain)
	if err != nil {
		return infer.ReadResponse[DnsRecordArgs, DnsRecordState]{}, nil
	}

	state.Name = currentRecord.Name
	state.Type = currentRecord.Type
	state.Value = currentRecord.Value
	if currentRecord.Priority != "" {
		state.Priority = &currentRecord.Priority
	} else {
		state.Priority = nil
	}

	return infer.ReadResponse[DnsRecordArgs, DnsRecordState]{
		ID:    req.ID,
		State: state,
	}, nil
}

func (r *DnsRecord) Update(
	ctx context.Context,
	req infer.UpdateRequest[DnsRecordArgs, DnsRecordState],
) (infer.UpdateResponse[DnsRecordState], error) {
	state := req.State
	inputs := req.Inputs

	if err := validateDnsRecord(inputs); err != nil {
		return infer.UpdateResponse[DnsRecordState]{}, err
	}

	config := infer.GetConfig[Config](ctx)
	client := NewNetcupClient(config.ApiKey, config.ApiPassword, config.CustomerID)

	_, err := client.GetDnsRecordById(state.RecordID, state.Domain)
	if err != nil {
		return infer.UpdateResponse[DnsRecordState]{}, fmt.Errorf("failed to find existing DNS record for update: %w", err)
	}

	var priority string
	if inputs.Priority != nil {
		priority = *inputs.Priority
	}

	err = client.UpdateDnsRecord(state.RecordID, inputs.Domain, inputs.Name, inputs.Type, inputs.Value, priority)
	if err != nil {
		return infer.UpdateResponse[DnsRecordState]{}, fmt.Errorf("failed to update DNS record: %w", err)
	}

	newState := DnsRecordState{
		DnsRecordArgs: inputs,
		RecordID:      state.RecordID,
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

	if req.Inputs.Value != req.State.Value {
		hasChanges = true
	}

	inputPriority := ""
	if req.Inputs.Priority != nil {
		inputPriority = *req.Inputs.Priority
	}
	statePriority := ""
	if req.State.Priority != nil {
		statePriority = *req.State.Priority
	}

	if inputPriority != statePriority {
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
	state := req.State

	config := infer.GetConfig[Config](ctx)
	client := NewNetcupClient(config.ApiKey, config.ApiPassword, config.CustomerID)

	err := client.DeleteDnsRecord(state.RecordID, state.Domain)
	if err != nil {
		return infer.DeleteResponse{}, fmt.Errorf("failed to delete DNS record %s: %w", state.RecordID, err)
	}

	return infer.DeleteResponse{}, nil
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

func buildFQDN(name, domain string) string {
	if name == "@" || name == "" {
		return domain
	}
	return fmt.Sprintf("%s.%s", name, domain)
}
