# Netcup DNS Example

This example demonstrates how to use the Pulumi Netcup provider to manage DNS records.

## Prerequisites

1. A Netcup account with API access enabled
2. Your Netcup API credentials:
   - API Key
   - API Password
   - Customer ID

You can obtain these credentials from your Netcup Customer Control Panel (CCP) at https://www.customercontrolpanel.de

## Configuration

Set the required configuration values:

```bash
pulumi config set pulumi-netcup:apiKey <your-api-key>
pulumi config set pulumi-netcup:apiPassword <your-api-password> --secret
pulumi config set pulumi-netcup:customerId <your-customer-id>
```

## Running the Example

1. Initialize a new Pulumi stack:
   ```bash
   pulumi stack init dev
   ```

2. Configure the provider settings as shown above.

3. Run the program:
   ```bash
   pulumi up
   ```

## Supported DNS Record Types

The provider supports all major DNS record types supported by Netcup:

- **A**: IPv4 address records
- **AAAA**: IPv6 address records  
- **MX**: Mail exchange records (requires priority)
- **CNAME**: Canonical name records
- **CAA**: Certificate Authority Authorization records
- **SRV**: Service records (requires priority, weight, port)
- **TXT**: Text records
- **TLSA**: TLS Certificate Association records
- **NS**: Name server records
- **DS**: Delegation Signer records
- **OPENPGPKEY**: OpenPGP public key records
- **SMIMEA**: S/MIME Certificate Association records
- **SSHFP**: SSH public key fingerprint records

## Example Usage

```go
// Create an A record
dnsRecord, err := netcup.NewDnsRecord(ctx, "www-record", &netcup.DnsRecordArgs{
    Domain: pulumi.String("example.com"),
    Name:   pulumi.String("www"),
    Type:   pulumi.String("A"),
    Value:  pulumi.String("1.2.3.4"),
})

// Create an MX record
mxRecord, err := netcup.NewDnsRecord(ctx, "mail-record", &netcup.DnsRecordArgs{
    Domain:   pulumi.String("example.com"),
    Name:     pulumi.String("@"),
    Type:     pulumi.String("MX"),
    Value:    pulumi.String("mail.example.com"),
    Priority: pulumi.IntPtr(10),
})

// Create an SRV record
srvRecord, err := netcup.NewDnsRecord(ctx, "sip-record", &netcup.DnsRecordArgs{
    Domain:   pulumi.String("example.com"),
    Name:     pulumi.String("_sip._tcp"),
    Type:     pulumi.String("SRV"),
    Value:    pulumi.String("sip.example.com"),
    Priority: pulumi.IntPtr(10),
    Weight:   pulumi.IntPtr(20),
    Port:     pulumi.IntPtr(5060),
})
```