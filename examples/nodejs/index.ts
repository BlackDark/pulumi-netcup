import * as pulumi from "@pulumi/pulumi";
import * as netcup from "@blackdark/pulumi-netcup";

const dnsRecord = new netcup.DnsRecord("example-dns-record", {
  domain: "example.com",
  name: "test",
  type: "A",
  value: "1.2.3.4",
});

export const fqdn = dnsRecord.fqdn;
export const recordId = dnsRecord.recordId;
