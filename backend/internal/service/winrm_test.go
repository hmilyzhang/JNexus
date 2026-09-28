package service

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"jnexus/internal/model"
)

// a:Action element of WinRM requests: shell ops (Command / Receive / Signal)
// and the shell-open transfer (http://schemas.xmlsoap.org/ws/2004/09/transfer/Create)
var actionRe = regexp.MustCompile(`(?:windows/shell|ws/2004/09/transfer)/([A-Za-z]+)</a:Action>`)

// Mock WinRM endpoint modelled on the winrm library's own client test fixtures:
// requests carrying a Basic auth header get the full SOAP success flow
// (create shell -> command -> receive -> signal), anything else gets a 401 with
// a text/html body — the exact shape that surfaces as
// "http response error: 401 - invalid content type".

const mockCreateShell = `<s:Envelope xml:lang="en-US" xmlns:s="http://www.w3.org/2003/05/soap-envelope" xmlns:a="http://schemas.xmlsoap.org/ws/2004/08/addressing" xmlns:x="http://schemas.xmlsoap.org/ws/2004/09/transfer" xmlns:w="http://schemas.dmtf.org/wbem/wsman/1/wsman.xsd" xmlns:rsp="http://schemas.microsoft.com/wbem/wsman/1/windows/shell"><s:Header><a:Action>http://schemas.xmlsoap.org/ws/2004/09/transfer/CreateResponse</a:Action><a:MessageID>uuid:195078CF-804B-41F7-A246-9CB3C1A41A9A</a:MessageID><a:To>http://schemas.xmlsoap.org/ws/2004/08/addressing/role/anonymous</a:To><a:RelatesTo>uuid:D00059E8-57D6-4035-AD8D-3EDC495DA163</a:RelatesTo></s:Header><s:Body><x:ResourceCreated><a:Address>http://127.0.0.1/wsman</a:Address><a:ReferenceParameters><w:ResourceURI>http://schemas.microsoft.com/wbem/wsman/1/windows/shell/cmd</w:ResourceURI><w:SelectorSet><w:Selector Name="ShellId">67A74734-DD32-4F10-89DE-49A060483810</w:Selector></w:SelectorSet></a:ReferenceParameters><rsp:Shell><rsp:ShellId>67A74734-DD32-4F10-89DE-49A060483810</rsp:ShellId></rsp:Shell></x:ResourceCreated></s:Body></s:Envelope>`

const mockCommand = `<s:Envelope xml:lang="en-US" xmlns:s="http://www.w3.org/2003/05/soap-envelope" xmlns:a="http://schemas.xmlsoap.org/ws/2004/08/addressing" xmlns:x="http://schemas.xmlsoap.org/ws/2004/09/transfer" xmlns:w="http://schemas.dmtf.org/wbem/wsman/1/wsman.xsd" xmlns:rsp="http://schemas.microsoft.com/wbem/wsman/1/windows/shell"><s:Header><a:Action>http://schemas.microsoft.com/wbem/wsman/1/windows/shell/CommandResponse</a:Action><a:MessageID>uuid:D9E108AA-E32B-45E3-8601-E9C70999D3BA</a:MessageID><a:To>http://schemas.xmlsoap.org/ws/2004/08/addressing/role/anonymous</a:To><a:RelatesTo>uuid:F530804C-6D02-4FA9-AE78-1997750594BA</a:RelatesTo></s:Header><s:Body><rsp:CommandResponse><rsp:CommandId>1A6DEE6B-EC68-4DD6-87E9-030C0048ECC4</rsp:CommandId></rsp:CommandResponse></s:Body></s:Envelope>`

// stdout "That's all folks!!!" + Done + the given exit code
const mockReceiveTpl = `<s:Envelope xml:lang="en-US" xmlns:s="http://www.w3.org/2003/05/soap-envelope" xmlns:a="http://schemas.xmlsoap.org/ws/2004/08/addressing" xmlns:w="http://schemas.dmtf.org/wbem/wsman/1/wsman.xsd" xmlns:rsp="http://schemas.microsoft.com/wbem/wsman/1/windows/shell"><s:Header><a:Action>http://schemas.microsoft.com/wbem/wsman/1/windows/shell/ReceiveResponse</a:Action><a:MessageID>uuid:AAD46BD4-6315-4C3C-93D4-94A55773287D</a:MessageID><a:To>http://schemas.xmlsoap.org/ws/2004/08/addressing/role/anonymous</a:To><a:RelatesTo>uuid:18A52A06-9027-41DC-8850-3F244595AF62</a:RelatesTo></s:Header><s:Body><rsp:ReceiveResponse><rsp:Stream Name="stdout" CommandId="1A6DEE6B-EC68-4DD6-87E9-030C0048ECC4">VGhhdCdzIGFsbCBmb2xrcyEhIQ==</rsp:Stream><rsp:CommandState CommandId="1A6DEE6B-EC68-4DD6-87E9-030C0048ECC4" State="http://schemas.microsoft.com/wbem/wsman/1/windows/shell/CommandState/Done"><rsp:ExitCode>%d</rsp:ExitCode></rsp:CommandState></rsp:ReceiveResponse></s:Body></s:Envelope>`

const mockEmpty = `<s:Envelope xml:lang="en-US" xmlns:s="http://www.w3.org/2003/05/soap-envelope"><s:Body></s:Body></s:Envelope>`

type mockWinRM struct {
	srv          *httptest.Server
	basicHits    int // successful SOAP requests carrying Basic auth
	unauthHits   int // 401 rejections (NTLM attempts)
	receiveCount int
	exitCode     int // exit code reported by the Receive response
	basicUser    string
	basicPass    string
}

func newMockWinRM(t *testing.T, exitCode int, basicUser, basicPass string) *mockWinRM {
	t.Helper()
	m := &mockWinRM{exitCode: exitCode, basicUser: basicUser, basicPass: basicPass}
	m.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		user, pass, ok := r.BasicAuth()
		if ok && user == m.basicUser && pass == m.basicPass {
			m.basicHits++
			action := ""
			if mt := actionRe.FindSubmatch(body); mt != nil {
				action = string(mt[1])
			}
			w.Header().Set("Content-Type", "application/soap+xml;charset=UTF-8")
			switch {
			case action == "Create": // shell open
				fmt.Fprint(w, mockCreateShell)
			case strings.HasPrefix(action, "Command"):
				fmt.Fprint(w, mockCommand)
			case strings.HasPrefix(action, "Receive"):
				m.receiveCount++
				fmt.Fprintf(w, mockReceiveTpl, m.exitCode)
			default: // Signal / Close / anything else
				fmt.Fprint(w, mockEmpty)
			}
			return
		}
		m.unauthHits++
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprint(w, "<html><body>401 Unauthorized</body></html>")
	}))
	t.Cleanup(m.srv.Close)
	return m
}

func testWinHost(port int) *model.Host {
	return &model.Host{OSType: "windows", IP: "127.0.0.1", WinRMPort: port}
}

// Auth rejected on NTLM, accepted via the Basic-over-HTTP fallback rung.
func TestWinRMRunFallbackToBasicHTTP(t *testing.T) {
	m := newMockWinRM(t, 0, "svc", "Secret#1")
	port := strings.Split(strings.TrimPrefix(m.srv.URL, "http://"), ":")[1]
	h := testWinHost(atoi(t, port))
	out, code, err := WinRMRun(h, "svc", "Secret#1", "whoami", 30)
	if err != nil {
		t.Fatalf("expected success via fallback, got error: %v", err)
	}
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}
	if !strings.Contains(out, "That's all folks") {
		t.Fatalf("expected command output, got %q", out)
	}
	if m.basicHits == 0 {
		t.Fatalf("expected a successful Basic rung, got %d Basic requests", m.basicHits)
	}
}

// Every transport rejected: the final error must be the actionable aggregate,
// not a swallowed "[winrm] ..." line in the output.
func TestWinRMRunAllTransportsRejected(t *testing.T) {
	m := newMockWinRM(t, 0, "svc", "RightPassword")
	port := strings.Split(strings.TrimPrefix(m.srv.URL, "http://"), ":")[1]
	h := testWinHost(atoi(t, port))
	out, code, err := WinRMRun(h, "svc", "WrongPassword", "whoami", 30)
	if err == nil {
		t.Fatalf("expected aggregate auth error, got nil (out=%q code=%d)", out, code)
	}
	if !strings.Contains(err.Error(), "401") || !strings.Contains(err.Error(), "NTLM") {
		t.Fatalf("expected aggregate 401 error mentioning the transport chain, got: %v", err)
	}
	if strings.Contains(out, "[winrm]") {
		t.Fatalf("auth failure must not leak into output: %q", out)
	}
}

// A command that ran and exited non-zero keeps its output and its exit code
// (no transport fallback, no error return).
func TestWinRMRunBusinessErrorKeepsOutput(t *testing.T) {
	m := newMockWinRM(t, 123, "svc", "Secret#1")
	port := strings.Split(strings.TrimPrefix(m.srv.URL, "http://"), ":")[1]
	h := testWinHost(atoi(t, port))
	out, code, err := WinRMRun(h, "svc", "Secret#1", "whoami", 30)
	if err != nil {
		t.Fatalf("non-zero exit with output must not surface as error, got: %v", err)
	}
	if code != 123 {
		t.Fatalf("expected remote exit code 123, got %d", code)
	}
	if !strings.Contains(out, "That's all folks") || strings.Contains(out, "[winrm]") {
		t.Fatalf("expected clean command output, got %q", out)
	}
}

func atoi(t *testing.T, s string) int {
	t.Helper()
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			panic("non-digit in port: " + s)
		}
		n = n*10 + int(c-'0')
	}
	return n
}
