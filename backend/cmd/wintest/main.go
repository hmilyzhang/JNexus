// Isolated WinRM auth probe with request/response dump.
package main

import (
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/masterzen/winrm"
	"github.com/masterzen/winrm/soap"
)

type dumpRT struct {
	std  *http.Client
	url  string
	user string
	pass string
}

func (d *dumpRT) Post(client *winrm.Client, msg *soap.SoapMessage) (string, error) {
	req, _ := http.NewRequest("POST", d.url, strings.NewReader(msg.String()))
	req.Header.Set("Content-Type", "application/soap+xml;charset=UTF-8")
	req.SetBasicAuth(d.user, d.pass)
	fmt.Printf(">> POST %s\n", d.url)
	for k, v := range req.Header {
		val := v[0]
		if k == "Authorization" && len(val) > 30 {
			val = val[:28] + "...(masked)"
		}
		fmt.Printf(">> %s: %s\n", k, val)
	}
	resp, err := d.std.Do(req)
	if err != nil {
		fmt.Println("<< transport error:", err)
		return "", err
	}
	fmt.Printf("<< %s | WWW-Authenticate: %q\n", resp.Status, resp.Header.Get("WWW-Authenticate"))
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 300))
	resp.Body.Close()
	fmt.Printf("<< body: %s\n", string(body))
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("http response error: %d - invalid content type", resp.StatusCode)
	}
	return string(body), nil
}

func (d *dumpRT) Transport(*winrm.Endpoint) error { return nil }

func main() {
	host, user, pass := os.Args[1], os.Args[2], os.Args[3]
	params := winrm.DefaultParameters
	params.TransportDecorator = func() winrm.Transporter {
		return &dumpRT{
			std: &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
				Dial: (&net.Dialer{Timeout: 10 * time.Second}).Dial}},
			url:  "https://" + host + ":5986/wsman",
			user: user, pass: pass,
		}
	}
	client, err := winrm.NewClientWithParameters(
		winrm.NewEndpoint(host, 5986, true, true, nil, nil, nil, 0), user, pass, params)
	if err != nil {
		fmt.Println("client error:", err)
		return
	}
	out, _, code, err := client.RunWithString("echo ok", "")
	fmt.Printf("code=%d err=%v out=%q\n", code, err, out)
}
