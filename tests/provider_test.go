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

package tests

import (
	"context"
	"testing"

	netcup "github.com/blackdark/pulumi-netcup/provider"
	"github.com/blang/semver"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/integration"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/property"
)

func TestDnsRecordCreateDryRun(t *testing.T) {
	t.Parallel()

	prov := provider(t)

	response, err := prov.Create(p.CreateRequest{
		Urn: urn("DnsRecord"),
		Properties: property.NewMap(map[string]property.Value{
			"domain": property.New("example.com"),
			"name":   property.New("test"),
			"type":   property.New("A"),
			"value":  property.New("1.2.3.4"),
		}),
		DryRun: true,
	})

	require.NoError(t, err)
	assert.True(t, response.Properties.Get("recordId").IsComputed())
	assert.True(t, response.Properties.Get("fqdn").IsComputed())
}

// urn is a helper function to build an urn for running integration tests.
func urn(typ string) resource.URN {
	return resource.NewURN("stack", "proj", "",
		tokens.Type("netcup:index:"+typ), "name")
}

// Create a test server.
func provider(t *testing.T) integration.Server {
	s, err := integration.NewServer(
		context.Background(),
		netcup.Name,
		semver.MustParse("1.0.0"),
		integration.WithProvider(netcup.Provider()),
	)
	require.NoError(t, err)
	return s
}
