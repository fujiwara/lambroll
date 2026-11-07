package lambroll

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestDeployWaitsForPublishedVersion(t *testing.T) {
	originalPolicy := retryPolicy
	t.Cleanup(func() { retryPolicy = originalPolicy })
	// Exercise retry exhaustion without waiting through production backoff delays.
	retryPolicy.MinDelay = 0
	retryPolicy.MaxDelay = 0
	retryPolicy.MaxCount = 3
	t.Setenv("AWS_ACCESS_KEY_ID", "test")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "test")
	t.Setenv("AWS_EC2_METADATA_DISABLED", "true")
	for _, tt := range []struct {
		name      string
		create    bool
		state     string
		cancel    bool
		exhaust   bool
		noPublish bool
		dryRun    bool
		wantError bool
	}{
		{name: "update waits for snapshot", state: "Pending"},
		{name: "create waits for snapshot", create: true, state: "Pending"},
		{name: "already active", state: "Active"},
		{name: "failed snapshot preserves alias", state: "Failed", wantError: true},
		{name: "failed snapshot prevents alias creation", create: true, state: "Failed", wantError: true},
		{name: "cancellation preserves alias", state: "Pending", cancel: true, wantError: true},
		{name: "retry exhaustion preserves alias", state: "Pending", exhaust: true, wantError: true},
		{name: "retry exhaustion prevents alias creation", create: true, state: "Pending", exhaust: true, wantError: true},
		{name: "no publish", noPublish: true},
		{name: "dry run", dryRun: true},
		{name: "create dry run", create: true, dryRun: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			function := filepath.Join(t.TempDir(), "function.json")
			if err := os.WriteFile(function, []byte(`{"FunctionName":"snap-test","Runtime":"java21","Handler":"example.Handler","Role":"arn:aws:iam::000000000000:role/test","Code":{"S3Bucket":"test-bucket","S3Key":"function.zip"},"SnapStart":{"ApplyOn":"PublishedVersions"}}`), 0600); err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			var mu sync.Mutex
			created, active := false, false
			polls, aliases := 0, 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				mu.Lock()
				defer mu.Unlock()
				w.Header().Set("Content-Type", "application/json")
				switch {
				case r.Method == http.MethodGet && r.URL.Path == "/2015-03-31/functions/snap-test":
					qualifier := r.URL.Query().Get("Qualifier")
					if qualifier != "" {
						if qualifier != "42" {
							t.Errorf("polled version %q, want published version 42", qualifier)
						}
						polls++
						state := tt.state
						if polls > 1 && state == "Pending" && !tt.exhaust {
							state = "Active"
						}
						active = state == "Active"
						if tt.cancel {
							cancel()
						}
						// LastUpdateStatus can be Successful while SnapStart is Pending.
						_ = json.NewEncoder(w).Encode(map[string]any{"Configuration": map[string]string{"State": state, "LastUpdateStatus": "Successful", "Version": "42"}})
						return
					}
					if tt.create && !created {
						w.WriteHeader(http.StatusNotFound)
						_, _ = w.Write([]byte(`{"__type":"ResourceNotFoundException"}`))
						return
					}
					_, _ = w.Write([]byte(`{"Configuration":{"FunctionName":"snap-test","PackageType":"Zip","State":"Active","LastUpdateStatus":"Successful"},"Code":{}}`))
				case r.Method == http.MethodPost && r.URL.Path == "/2015-03-31/functions":
					created = true
					_, _ = w.Write([]byte(`{"Version":"42"}`))
				case r.Method == http.MethodPut && r.URL.Path == "/2015-03-31/functions/snap-test/code":
					_, _ = w.Write([]byte(`{"Version":"42"}`))
				case strings.HasPrefix(r.URL.Path, "/2015-03-31/functions/snap-test/aliases"):
					aliases++
					if !active {
						t.Error("alias changed before published version became Active")
					}
					var input struct{ FunctionVersion string }
					if err := json.NewDecoder(r.Body).Decode(&input); err != nil || input.FunctionVersion != "42" {
						t.Errorf("unexpected alias target: %+v (%v)", input, err)
					}
					_, _ = w.Write([]byte(`{}`))
				default:
					t.Errorf("unexpected request: %s %s", r.Method, r.URL)
					w.WriteHeader(http.StatusBadRequest)
				}
			}))
			defer server.Close()
			app, err := New(ctx, &Option{Function: function, Region: new("ap-northeast-1"), Endpoint: new(server.URL)})
			if err != nil {
				t.Fatal(err)
			}
			err = app.Deploy(ctx, &DeployOption{
				Publish: !tt.noPublish, AliasName: "current", SkipArchive: true, SkipConfiguration: true,
				DryRunOption: DryRunOption{DryRun: tt.dryRun},
			})
			if (err != nil) != tt.wantError {
				t.Fatalf("Deploy error = %v, wantError = %t", err, tt.wantError)
			}
			if tt.wantError && !strings.Contains(err.Error(), "version 42 to be Active") {
				t.Fatalf("error must identify the version whose readiness failed: %v", err)
			}
			if tt.cancel && !errors.Is(err, context.Canceled) {
				t.Fatalf("expected context cancellation, got %v", err)
			}
			if tt.exhaust && !strings.Contains(err.Error(), "max retries reached") {
				t.Fatalf("expected retry exhaustion, got %v", err)
			}
			mu.Lock()
			defer mu.Unlock()
			if tt.exhaust && polls != retryPolicy.MaxCount {
				t.Errorf("version checks = %d, want policy limit %d", polls, retryPolicy.MaxCount)
			}
			wantAliases := 1
			if tt.wantError || tt.noPublish || tt.dryRun {
				wantAliases = 0
			}
			if aliases != wantAliases {
				t.Errorf("alias mutations = %d, want %d", aliases, wantAliases)
			}
			if tt.noPublish || tt.dryRun {
				if polls != 0 {
					t.Errorf("unexpected version readiness checks: %d", polls)
				}
			} else if !tt.wantError && tt.state == "Pending" && polls < 2 {
				t.Errorf("expected polling through Pending to Active, got %d checks", polls)
			}
		})
	}
}
