package types_test

import (
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/nitsugaro/go-journey/types"
)

func TestNewJourneyRequestCreatesTypedIndependentSnapshot(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, "https://api.example.test:8443/customers?a=1&a=2", nil)
	request.Header.Add("X-Trace", "one")
	request.Header.Add("X-Trace", "two")
	request.Header.Set("Content-Type", "application/json; charset=utf-8")
	request.Header.Set("Content-Encoding", "gzip")
	request.Header.Set("Origin", "https://client.example.test")
	request.AddCookie(&http.Cookie{Name: "session", Value: "abc", Secure: true, HttpOnly: true, SameSite: http.SameSiteLaxMode})
	request.RemoteAddr = "192.0.2.1:4000"
	request.TLS = &tls.ConnectionState{Version: tls.VersionTLS13, PeerCertificates: []*x509.Certificate{{
		Raw: []byte{1, 2, 3}, Subject: pkix.Name{CommonName: "client"}, Issuer: pkix.Name{CommonName: "issuer"},
		SerialNumber: big.NewInt(42), DNSNames: []string{"client.example.test"},
	}}}
	body := []byte(`{"name":"Ada"}`)
	snapshot := types.NewJourneyRequest(request, body)
	if snapshot.Method != http.MethodPost || snapshot.Path != "/customers" || snapshot.Protocol != "https" ||
		snapshot.BaseURL != "https://api.example.test" || snapshot.Port != 8443 || snapshot.HTTPVersion == "" {
		t.Fatalf("request identity snapshot=%#v", snapshot)
	}
	if len(snapshot.QueryParameters["a"]) != 2 || len(snapshot.Headers["X-Trace"]) != 2 {
		t.Fatalf("query=%v headers=%v", snapshot.QueryParameters, snapshot.Headers)
	}
	if snapshot.Body.MediaType != "application/json" || snapshot.Body.Parameters["charset"] != "utf-8" ||
		snapshot.Body.ContentEncoding != "gzip" || string(snapshot.Body.Data) != string(body) {
		t.Fatalf("body=%#v", snapshot.Body)
	}
	if snapshot.Origin != "https://client.example.test" || snapshot.RemoteAddress != "192.0.2.1:4000" ||
		snapshot.TLSVersion != "TLS1.3" || len(snapshot.Certificates) != 1 || snapshot.Certificates[0].SerialNumber != "42" ||
		len(snapshot.Cookies) != 1 || snapshot.Cookies[0].Name != "session" || snapshot.Cookies[0].Value != "abc" {
		t.Fatalf("transport metadata snapshot=%#v", snapshot)
	}
	request.Header.Set("X-Trace", "changed")
	body[0] = 'x'
	if snapshot.Headers["X-Trace"][0] != "one" || snapshot.Body.Data[0] != '{' {
		t.Fatal("snapshot retained aliases to mutable request data")
	}
}
