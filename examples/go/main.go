package main

import (
	netcup "github.com/blackdark/pulumi-netcup/sdk/go/pulumi-netcup"

	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		// Create a DNS record
		// Note: This will require valid Netcup credentials to be configured via provider config
		dnsRecord, err := netcup.NewDnsRecord(ctx, "example-dns-record", &netcup.DnsRecordArgs{
			Domain: pulumi.String("example.com"),
			Name:   pulumi.String("test"),
			Type:   pulumi.String("A"),
			Value:  pulumi.String("1.2.3.4"),
		})
		if err != nil {
			return err
		}

		ctx.Export("fqdn", dnsRecord.Fqdn)
		ctx.Export("recordId", dnsRecord.RecordId)
		return nil
	})
}
