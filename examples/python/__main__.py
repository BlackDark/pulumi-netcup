import pulumi
import pulumi_netcup as netcup

dns_record = netcup.DnsRecord("example-dns-record",
    domain="example.com",
    name="test",
    type="A",
    value="1.2.3.4"
)

pulumi.export("fqdn", dns_record.fqdn)
pulumi.export("recordId", dns_record.record_id)
