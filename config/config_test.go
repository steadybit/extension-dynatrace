package config

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"github.com/steadybit/extension-dynatrace/types"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"
)

/********** helpers **********/

type reqCapture struct {
	Method string
	Path   string
	Query  url.Values
	Header http.Header
	Body   []byte
}

func newMockHTTPServer(t *testing.T, rc *reqCapture) *httptest.Server {
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		_ = r.Body.Close()
		if rc != nil {
			rc.Method = r.Method
			rc.Path = r.URL.Path
			rc.Query = r.URL.Query()
			rc.Header = r.Header.Clone()
			rc.Body = b
		}
		switch {
		case r.URL.Path == "/v2/events/ingest" && r.Method == http.MethodPost:
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"eventIngestResults":[{"correlationId":"corr-1","status":"OK"}],"reportCount":1}`))
		case r.URL.Path == "/v2/entities" && r.Method == http.MethodGet:
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"entities":[{"entityId":"HOST-1"}]}`))
		case r.URL.Path == "/v2/settings/objects" && r.Method == http.MethodPost:
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`[{"code":200,"objectId":"mw-123"}]`))
		case r.URL.Path == "/v2/settings/objects/mw-123" && r.Method == http.MethodDelete:
			w.WriteHeader(http.StatusNoContent)
		case r.URL.Path == "/v2/problems" && r.Method == http.MethodGet:
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"problems":[{"problemId":"p1"}]}`))
		default:
			w.WriteHeader(http.StatusBadRequest)
		}
	})
	return httptest.NewServer(h)
}

func genSelfSignedCert(t *testing.T, hosts []string) (certPEM, keyPEM []byte, pair tls.Certificate) {
	t.Helper()
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("key: %v", err)
	}
	serial, _ := rand.Int(rand.Reader, big.NewInt(1<<62))
	tpl := &x509.Certificate{
		SerialNumber:          serial,
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(365 * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}
	for _, h := range hosts {
		if ip := net.ParseIP(h); ip != nil {
			tpl.IPAddresses = append(tpl.IPAddresses, ip)
		} else {
			tpl.DNSNames = append(tpl.DNSNames, h)
		}
	}
	der, err := x509.CreateCertificate(rand.Reader, tpl, tpl, &priv.PublicKey, priv)
	if err != nil {
		t.Fatalf("cert: %v", err)
	}
	certPEM = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyPEM = pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(priv)})
	pair, err = tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		t.Fatalf("pair: %v", err)
	}
	return
}

func newTLSServerWithPair(t *testing.T, pair tls.Certificate, handler http.Handler) *httptest.Server {
	t.Helper()
	s := httptest.NewUnstartedServer(handler)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	s.Listener = ln
	s.TLS = &tls.Config{Certificates: []tls.Certificate{pair}}
	s.StartTLS()
	t.Cleanup(s.Close)
	return s
}

/********** tests for do() **********/

func Test_do_SendsHeadersAndBody(t *testing.T) {
	rc := &reqCapture{}
	srv := newMockHTTPServer(t, rc)
	defer srv.Close()

	s := Specification{ApiToken: "XYZ", InsecureSkipVerify: true}
	payload := map[string]string{"k": "v"}
	b, _ := json.Marshal(payload)

	_, resp, err := s.do(srv.URL+"/v2/events/ingest", http.MethodPost, b)
	if err != nil {
		t.Fatalf("do error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status=%d", resp.StatusCode)
	}
	if rc.Header.Get("Authorization") != "Api-Token XYZ" {
		t.Fatalf("missing auth header")
	}
	if rc.Header.Get("Content-Type") != "application/json; charset=UTF-8" {
		t.Fatalf("bad content-type: %s", rc.Header.Get("Content-Type"))
	}
	if !bytes.Contains(rc.Body, []byte(`"k":"v"`)) {
		t.Fatalf("missing body content")
	}
}

func Test_do_Loads_SSL_CERT_DIR_ForHTTPS(t *testing.T) {
	certPEM, _, pair := genSelfSignedCert(t, []string{"127.0.0.1"})
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "ok")
	})
	srv := newTLSServerWithPair(t, pair, h)

	// put cert in a dir and set SSL_CERT_DIR to include it
	d1 := t.TempDir()
	if err := os.WriteFile(filepath.Join(d1, "ca.crt"), certPEM, 0600); err != nil {
		t.Fatalf("write ca: %v", err)
	}
	// include an empty dir and the real dir to test colon-separated parsing
	d0 := t.TempDir()
	t.Setenv("SSL_CERT_DIR", d0+string(os.PathListSeparator)+d1)

	s := Specification{ApiToken: "X", InsecureSkipVerify: false}
	body, resp, err := s.do(srv.URL, http.MethodGet, nil)
	if err != nil {
		t.Fatalf("tls trust failed: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("status=%d", resp.StatusCode)
	}
	if !bytes.Contains(body, []byte("ok")) {
		t.Fatalf("unexpected body: %s", string(body))
	}
}

func Test_do_FailsWithoutCA_OnHTTPS(t *testing.T) {
	_, _, pair := genSelfSignedCert(t, []string{"127.0.0.1"})
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	})
	srv := newTLSServerWithPair(t, pair, h)

	t.Setenv("SSL_CERT_DIR", "")

	s := Specification{ApiToken: "X", InsecureSkipVerify: false}
	_, _, err := s.do(srv.URL, http.MethodGet, nil)
	if err == nil {
		t.Fatal("expected TLS verification error")
	}
}

/********** tests for high-level API methods **********/

func Test_PostEvent_Success(t *testing.T) {
	rc := &reqCapture{}
	srv := newMockHTTPServer(t, rc)
	defer srv.Close()

	spec := Specification{ApiBaseUrl: srv.URL, ApiToken: "X", InsecureSkipVerify: true}
	res, resp, err := spec.PostEvent(context.Background(), types.EventIngest{})
	if err != nil {
		t.Fatalf("PostEvent err: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status=%d", resp.StatusCode)
	}
	if rc.Path != "/v2/events/ingest" {
		t.Fatalf("path=%s", rc.Path)
	}
	if res == nil || len(res.EventIngestResults) != 1 {
		t.Fatalf("bad result: %+v", res)
	}
}

func Test_GetEntities_Success(t *testing.T) {
	rc := &reqCapture{}
	srv := newMockHTTPServer(t, rc)
	defer srv.Close()

	spec := Specification{ApiBaseUrl: srv.URL, ApiToken: "X"}
	res, resp, err := spec.GetEntities(context.Background(), "type(HOST)")
	if err != nil {
		t.Fatalf("GetEntities err: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status=%d", resp.StatusCode)
	}
	if rc.Path != "/v2/entities" {
		t.Fatalf("path=%s", rc.Path)
	}
	if res == nil || len(res.Entities) != 1 {
		t.Fatalf("bad entities: %+v", res)
	}
}

func Test_CreateMaintenanceWindow_Success(t *testing.T) {
	rc := &reqCapture{}
	srv := newMockHTTPServer(t, rc)
	defer srv.Close()

	spec := Specification{ApiBaseUrl: srv.URL, ApiToken: "X"}
	id, resp, err := spec.CreateMaintenanceWindow(context.Background(), types.CreateMaintenanceWindowRequest{})
	if err != nil {
		t.Fatalf("CreateMW err: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status=%d", resp.StatusCode)
	}
	if id == nil || *id != "mw-123" {
		t.Fatalf("id=%v", id)
	}
}

func Test_CreateMaintenanceWindow_Non200_Error(t *testing.T) {
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTeapot)
		_, _ = w.Write([]byte(`{"oops":true}`))
	})
	srv := httptest.NewServer(h)
	defer srv.Close()

	spec := Specification{ApiBaseUrl: srv.URL, ApiToken: "X"}
	id, resp, err := spec.CreateMaintenanceWindow(context.Background(), types.CreateMaintenanceWindowRequest{})
	if err == nil {
		t.Fatalf("expected error, got id=%v", id)
	}
	if resp.StatusCode != http.StatusTeapot {
		t.Fatalf("status=%d", resp.StatusCode)
	}
}

func Test_DeleteMaintenanceWindow_Success(t *testing.T) {
	rc := &reqCapture{}
	srv := newMockHTTPServer(t, rc)
	defer srv.Close()

	spec := Specification{ApiBaseUrl: srv.URL, ApiToken: "X"}
	resp, err := spec.DeleteMaintenanceWindow(context.Background(), "mw-123")
	if err != nil {
		t.Fatalf("DeleteMW err: %v", err)
	}
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("status=%d", resp.StatusCode)
	}
	if rc.Path != "/v2/settings/objects/mw-123" {
		t.Fatalf("path=%s", rc.Path)
	}
}

func Test_GetProblems_Success(t *testing.T) {
	rc := &reqCapture{}
	srv := newMockHTTPServer(t, rc)
	defer srv.Close()

	spec := Specification{ApiBaseUrl: srv.URL, ApiToken: "X"}
	problems, resp, err := spec.GetProblems(context.Background(), time.Now(), nil)
	if err != nil {
		t.Fatalf("GetProblems err: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status=%d", resp.StatusCode)
	}
	if rc.Path != "/v2/problems" {
		t.Fatalf("path=%s", rc.Path)
	}
	if problems == nil || len(problems) != 1 {
		t.Fatalf("bad problems: %+v", problems)
	}
}
