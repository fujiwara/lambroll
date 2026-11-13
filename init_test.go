package lambroll_test

import (
	"encoding/json"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fujiwara/lambroll"
)

func TestDownloadUsesAWSCABundle(t *testing.T) {
	const body = "zip content"
	ts := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(body))
	}))
	defer ts.Close()

	dir := t.TempDir()
	caFile := filepath.Join(dir, "ca.pem")
	caPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: ts.Certificate().Raw})
	if err := os.WriteFile(caFile, caPEM, 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("AWS_CA_BUNDLE", caFile)
	t.Setenv("AWS_REGION", "ap-northeast-1")
	t.Setenv("AWS_EC2_METADATA_DISABLED", "true")

	cfg, err := lambroll.NewAwsConfig(t.Context(), &lambroll.Option{})
	if err != nil {
		t.Fatal(err)
	}

	// The SDK's HTTP client trusts the CA bundle.
	dest := filepath.Join(dir, "function.zip")
	if err := lambroll.Download(t.Context(), cfg.HTTPClient, ts.URL, dest); err != nil {
		t.Fatalf("failed to download with the SDK HTTP client: %s", err)
	}
	b, err := os.ReadFile(dest)
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != body {
		t.Errorf("unexpected content: %q", b)
	}

	// The default client does not know AWS_CA_BUNDLE.
	if err := lambroll.Download(t.Context(), nil, ts.URL, filepath.Join(dir, "other.zip")); err == nil {
		t.Error("download with the default HTTP client must fail for an untrusted certificate")
	}
}

func TestInitFunctionNotFound(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/2015-03-31/functions/") {
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("X-Amzn-Errortype", "ResourceNotFoundException")
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte(`{"Type":"User","Message":"Function not found"}`))
			return
		}
		// STS GetCallerIdentity
		w.Header().Set("Content-Type", "text/xml")
		w.Write([]byte(`<GetCallerIdentityResponse xmlns="https://sts.amazonaws.com/doc/2011-06-15/"><GetCallerIdentityResult><Arn>arn:aws:iam::123456789012:user/test</Arn><UserId>AIDAEXAMPLE</UserId><Account>123456789012</Account></GetCallerIdentityResult></GetCallerIdentityResponse>`))
	}))
	defer ts.Close()

	t.Chdir(t.TempDir())
	t.Setenv("AWS_REGION", "ap-northeast-1")
	t.Setenv("AWS_ACCESS_KEY_ID", "dummy")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "dummy")
	t.Setenv("AWS_EC2_METADATA_DISABLED", "true")

	app, err := lambroll.New(t.Context(), &lambroll.Option{Endpoint: new(ts.URL)})
	if err != nil {
		t.Fatal(err)
	}
	if err := app.Init(t.Context(), &lambroll.InitOption{FunctionName: new("hello")}); err != nil {
		t.Fatal(err)
	}

	b, err := os.ReadFile("function.json")
	if err != nil {
		t.Fatal(err)
	}
	var fn struct {
		FunctionName string
		Handler      string
		Role         string
		Runtime      string
	}
	if err := json.Unmarshal(b, &fn); err != nil {
		t.Fatal(err)
	}
	if fn.FunctionName != "hello" {
		t.Errorf("unexpected FunctionName: %s", fn.FunctionName)
	}
	if fn.Runtime != "nodejs26.x" {
		t.Errorf("unexpected Runtime: %s", fn.Runtime)
	}
	if fn.Handler != "index.handler" {
		t.Errorf("unexpected Handler: %s", fn.Handler)
	}
	if fn.Role != "arn:aws:iam::123456789012:role/YOUR_LAMBDA_ROLE_NAME" {
		t.Errorf("unexpected Role: %s", fn.Role)
	}
}
