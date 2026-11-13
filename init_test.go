package lambroll_test

import (
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
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
