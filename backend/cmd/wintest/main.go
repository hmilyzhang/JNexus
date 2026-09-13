// WinRM diagnostic probe: reproduces the raw WinRM connection outside the platform.
// Usage:
//
//	wintest <host> <user> <pass>              HTTPS 5986 Basic (self-signed certs accepted)
//	wintest <host> <user> <pass> krb <realm> [spn] [krb5conf]   Kerberos (domain) over HTTPS 5986
package main

import (
	"fmt"
	"os"

	"github.com/masterzen/winrm"
)

func main() {
	if len(os.Args) < 4 {
		fmt.Println("usage: wintest <host> <user> <pass> [krb <realm> [spn] [krb5conf]]")
		os.Exit(2)
	}
	host, user, pass := os.Args[1], os.Args[2], os.Args[3]
	krbMode := len(os.Args) > 4 && os.Args[4] == "krb"

	params := winrm.DefaultParameters
	var endpoint *winrm.Endpoint
	if krbMode {
		realm, spn, conf := "GLBANK.COM", "", "/etc/krb5.conf"
		if len(os.Args) > 5 {
			realm = os.Args[5]
		}
		if len(os.Args) > 6 {
			spn = os.Args[6]
		}
		if len(os.Args) > 7 {
			conf = os.Args[7]
		}
		if spn == "" {
			spn = "WSMAN/" + host
		}
		params.TransportDecorator = func() winrm.Transporter {
			return winrm.NewClientKerberos(&winrm.Settings{
				WinRMUsername: user, WinRMPassword: pass,
				KrbRealm: realm, KrbSpn: spn, KrbConfig: conf,
				WinRMProto: "https", WinRMPort: 5986, WinRMHost: host, WinRMInsecure: true,
			})
		}
		endpoint = winrm.NewEndpoint(host, 5986, true, true, nil, nil, nil, 0)
		fmt.Printf("mode=kerberos realm=%s spn=%s conf=%s\n", realm, spn, conf)
	} else {
		endpoint = winrm.NewEndpoint(host, 5986, true, true, nil, nil, nil, 0)
		fmt.Println("mode=basic(https)")
	}
	client, err := winrm.NewClientWithParameters(endpoint, user, pass, params)
	if err != nil {
		fmt.Println("client error:", err)
		os.Exit(1)
	}
	out, _, code, err := client.RunWithString("echo ok", "")
	fmt.Printf("code=%d err=%v out=%q\n", code, err, out)
}
