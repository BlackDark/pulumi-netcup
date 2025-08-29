package main

import (
	"github.com/blackdark/pulumi-netcup/sdk/go/pulumi-netcup"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		// Get configuration
		conf := config.New(ctx, "")
		
		// Create DNS A record
		dnsRecord, err := netcup.NewDnsRecord(ctx, "example-a-record", &netcup.DnsRecordArgs{
			Domain: pulumi.String("example.com"),
			Name:   pulumi.String("www"),
			Type:   pulumi.String("A"),
			Value:  pulumi.String("1.2.3.4"),
		})
		if err != nil {
			return err
		}

		// Export the DNS record FQDN
		ctx.Export("fqdn", dnsRecord.FQDN)
		ctx.Export("recordId", dnsRecord.RecordID)

		// Create MX record
		mxRecord, err := netcup.NewDnsRecord(ctx, "example-mx-record", &netcup.DnsRecordArgs{
			Domain:   pulumi.String("example.com"),
			Name:     pulumi.String("@"),
			Type:     pulumi.String("MX"),
			Value:    pulumi.String("mail.example.com"),
			Priority: pulumi.StringPtr("10"),
		})
		if err != nil {
			return err
		}

		ctx.Export("mxRecordId", mxRecord.RecordID)

		// Create SRV record (Note: SRV records have complex format requirements in Netcup)
		srvRecord, err := netcup.NewDnsRecord(ctx, "example-srv-record", &netcup.DnsRecordArgs{
			Domain:   pulumi.String("example.com"),
			Name:     pulumi.String("_sip._tcp"),
			Type:     pulumi.String("SRV"),
			Value:    pulumi.String("10 20 5060 sip.example.com"), // priority weight port target
		})
		if err != nil {
			return err
		}

		ctx.Export("srvRecordId", srvRecord.RecordID)

		// Create TXT record
		txtRecord, err := netcup.NewDnsRecord(ctx, "example-txt-record", &netcup.DnsRecordArgs{
			Domain: pulumi.String("example.com"),
			Name:   pulumi.String("@"),
			Type:   pulumi.String("TXT"),
			Value:  pulumi.String("v=spf1 include:_spf.google.com ~all"),
		})
		if err != nil {
			return err
		}

		ctx.Export("txtRecordId", txtRecord.RecordID)

		return nil
	})
}