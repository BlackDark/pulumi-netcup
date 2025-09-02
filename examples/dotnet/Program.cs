using System.Collections.Generic;
using Pulumi;
using Netcup = Blackdark.PulumiNetcup;

return await Deployment.RunAsync(() => 
{
    var dnsRecord = new Netcup.DnsRecord("example-dns-record", new()
    {
        Domain = "example.com",
        Name = "test",
        Type = "A",
        Value = "1.2.3.4",
    });

    return new Dictionary<string, object?>
    {
        ["fqdn"] = dnsRecord.Fqdn,
        ["recordId"] = dnsRecord.RecordId,
    };
});

