//go:build yaml || all
// +build yaml all

package examples

import (
	"testing"

	"github.com/pulumi/providertest/pulumitest"
	"github.com/pulumi/providertest/pulumitest/opttest"
)

func TestYAMLExampleLifecycle(t *testing.T) {
	pt := pulumitest.NewPulumiTest(t, "yaml",
		opttest.AttachProviderServer("netcup", providerFactory),
		opttest.SkipInstall(),
		opttest.Env("NETCUP_API_KEY", "dummy"),
		opttest.Env("NETCUP_API_PASSWORD", "dummy"),
		opttest.Env("NETCUP_CUSTOMER_ID", "dummy"),
	)

	// Set configuration values as well
	pt.SetConfig(t, "netcup:apiKey", "dummy")
	pt.SetConfig(t, "netcup:apiPassword", "dummy")
	pt.SetConfig(t, "netcup:customerId", "dummy")

	// Only preview - we can't actually create resources with dummy credentials
	pt.Preview(t)
	// pt.Up(t) would fail with dummy credentials, which is expected
	// pt.Destroy(t)
}

func TestYAMLExampleUpgrade(t *testing.T) {
	pt := pulumitest.NewPulumiTest(t, "yaml",
		opttest.AttachProviderServer("netcup", providerFactory),
		opttest.SkipInstall(),
		opttest.Env("NETCUP_API_KEY", "dummy"),
		opttest.Env("NETCUP_API_PASSWORD", "dummy"),
		opttest.Env("NETCUP_CUSTOMER_ID", "dummy"),
	)

	// Set configuration values as well
	pt.SetConfig(t, "netcup:apiKey", "dummy")
	pt.SetConfig(t, "netcup:apiPassword", "dummy")
	pt.SetConfig(t, "netcup:customerId", "dummy")

	// Skip the upgrade test for now as it requires a different setup
	t.Skip("Provider upgrade test requires specific test data setup")
}
