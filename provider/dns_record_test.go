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
	"testing"

	"github.com/blang/semver"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	integration "github.com/pulumi/pulumi-go-provider/integration"
	presource "github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/property"
)

func TestDnsRecordValidation(t *testing.T) {
	tests := []struct {
		name        string
		args        DnsRecordArgs
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid A record",
			args: DnsRecordArgs{
				Domain: "example.com",
				Name:   "test",
				Type:   "A",
				Value:  "1.2.3.4",
			},
			expectError: false,
		},
		{
			name: "valid MX record with priority",
			args: DnsRecordArgs{
				Domain:   "example.com",
				Name:     "@",
				Type:     "MX",
				Value:    "mail.example.com",
				Priority: stringPtr("10"),
			},
			expectError: false,
		},
		{
			name: "invalid MX record without priority",
			args: DnsRecordArgs{
				Domain: "example.com",
				Name:   "@",
				Type:   "MX",
				Value:  "mail.example.com",
			},
			expectError: true,
			errorMsg:    "priority is required for MX records",
		},
		{
			name: "invalid CNAME for root domain",
			args: DnsRecordArgs{
				Domain: "example.com",
				Name:   "@",
				Type:   "CNAME",
				Value:  "target.example.com",
			},
			expectError: true,
			errorMsg:    "CNAME records cannot be created for the root domain",
		},
		{
			name: "invalid record type",
			args: DnsRecordArgs{
				Domain: "example.com",
				Name:   "test",
				Type:   "INVALID",
				Value:  "1.2.3.4",
			},
			expectError: true,
			errorMsg:    "unsupported DNS record type",
		},
		{
			name: "missing domain",
			args: DnsRecordArgs{
				Name:  "test",
				Type:  "A",
				Value: "1.2.3.4",
			},
			expectError: true,
			errorMsg:    "domain is required",
		},
		{
			name: "missing name",
			args: DnsRecordArgs{
				Domain: "example.com",
				Type:   "A",
				Value:  "1.2.3.4",
			},
			expectError: true,
			errorMsg:    "name is required",
		},
		{
			name: "missing value",
			args: DnsRecordArgs{
				Domain: "example.com",
				Name:   "test",
				Type:   "A",
			},
			expectError: true,
			errorMsg:    "value is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateDnsRecord(tt.args)
			if tt.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestBuildFQDN(t *testing.T) {
	tests := []struct {
		name     string
		hostname string
		domain   string
		expected string
	}{
		{
			name:     "root domain with @",
			hostname: "@",
			domain:   "example.com",
			expected: "example.com",
		},
		{
			name:     "root domain with empty string",
			hostname: "",
			domain:   "example.com",
			expected: "example.com",
		},
		{
			name:     "subdomain",
			hostname: "www",
			domain:   "example.com",
			expected: "www.example.com",
		},
		{
			name:     "complex subdomain",
			hostname: "mail.internal",
			domain:   "example.com",
			expected: "mail.internal.example.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := buildFQDN(tt.hostname, tt.domain)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestDnsRecordLifecycle tests the complete CRUD lifecycle for DNS records
// Note: This test requires valid Netcup credentials and will be skipped in CI
func TestDnsRecordLifecycle(t *testing.T) {
	t.Skip("Integration test requiring valid Netcup credentials - enable manually for testing")

	server, err := integration.NewServer(t.Context(),
		"netcup",
		semver.Version{Minor: 1},
		integration.WithProvider(Provider()),
	)
	require.NoError(t, err)

	// Test configuration - replace with valid test credentials when running manually
	// config := map[string]interface{}{
	//	"apiKey":     "test-api-key",
	//	"apiPassword": "test-api-password",
	//	"customerId": "test-customer-id",
	// }

	integration.LifeCycleTest{
		Resource: "netcup:index:DnsRecord",
		Create: integration.Operation{
			Inputs: presource.FromResourcePropertyMap(presource.NewPropertyMapFromMap(map[string]interface{}{
				"domain": "test-domain.com",
				"name":   "test",
				"type":   "A",
				"value":  "1.2.3.4",
			})),
			Hook: func(inputs, output property.Map) {
				t.Logf("Create Outputs: %v", output)

				// Verify required fields are present
				assert.NotEmpty(t, output.Get("recordId").AsString(), "recordId should be set")
				assert.Equal(t, "test-domain.com", output.Get("domain").AsString())
				assert.Equal(t, "test", output.Get("name").AsString())
				assert.Equal(t, "A", output.Get("type").AsString())
				assert.Equal(t, "1.2.3.4", output.Get("value").AsString())
				assert.Equal(t, "test.test-domain.com", output.Get("fqdn").AsString())
			},
		},
		Updates: []integration.Operation{
			{
				// Test updating the value
				Inputs: presource.FromResourcePropertyMap(presource.NewPropertyMapFromMap(map[string]interface{}{
					"domain": "test-domain.com",
					"name":   "test",
					"type":   "A",
					"value":  "5.6.7.8", // Changed value
				})),
				Hook: func(inputs, output property.Map) {
					t.Logf("Update Outputs: %v", output)

					// Verify the value was updated
					assert.Equal(t, "5.6.7.8", output.Get("value").AsString())
					assert.NotEmpty(t, output.Get("recordId").AsString(), "recordId should still be set")
				},
			},
		},
	}.Run(t, server)
}

// TestDnsRecordMXWithPriority tests MX record creation with priority
func TestDnsRecordMXWithPriority(t *testing.T) {
	t.Skip("Integration test requiring valid Netcup credentials - enable manually for testing")

	server, err := integration.NewServer(t.Context(),
		"netcup",
		semver.Version{Minor: 1},
		integration.WithProvider(Provider()),
	)
	require.NoError(t, err)

	// config := map[string]interface{}{
	//	"apiKey":     "test-api-key",
	//	"apiPassword": "test-api-password",
	//	"customerId": "test-customer-id",
	// }

	integration.LifeCycleTest{
		Resource: "netcup:index:DnsRecord",
		Create: integration.Operation{
			Inputs: presource.FromResourcePropertyMap(presource.NewPropertyMapFromMap(map[string]interface{}{
				"domain":   "test-domain.com",
				"name":     "@",
				"type":     "MX",
				"value":    "mail.test-domain.com",
				"priority": "10",
			})),
			Hook: func(inputs, output property.Map) {
				t.Logf("MX Create Outputs: %v", output)

				// Verify MX record fields
				assert.Equal(t, "MX", output.Get("type").AsString())
				assert.Equal(t, "mail.test-domain.com", output.Get("value").AsString())
				assert.Equal(t, "10", output.Get("priority").AsString())
				assert.Equal(t, "test-domain.com", output.Get("fqdn").AsString()) // Root domain
			},
		},
	}.Run(t, server)
}

// stringPtr is a helper function to get a pointer to a string
func stringPtr(s string) *string {
	return &s
}
