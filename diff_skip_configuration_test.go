package lambroll_test

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/lambda/types"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"testing"

	"github.com/fujiwara/lambroll"
)

func TestParseDiffSkipConfiguration(t *testing.T) {
	sub, options, _, err := lambroll.ParseCLI([]string{"diff", "--skip-configuration"})
	if err != nil {
		t.Fatal(err)
	}
	if sub != "diff" {
		t.Fatalf("unexpected subcommand: %s", sub)
	}
	data, err := json.Marshal(options.Diff)
	if err != nil {
		t.Fatal(err)
	}
	var parsed map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatal(err)
	}
	if parsed["skip_configuration"] != true {
		t.Fatalf("skip_configuration not enabled: %s", data)
	}
}

func TestDiffSkipConfiguration(t *testing.T) {
	tests := []struct {
		name                                                            string
		skip, code, skipFunction, exitCode                              bool
		changedCode, functionURL, image, localImage, missingSource      bool
		sameURL, matchingPermissions                                    bool
		getStatus, urlStatus                                            int
		wantDiff, wantCode, wantConfiguration, wantURL, wantPermissions bool
		wantError                                                       string
	}{
		{name: "same code ignores function configuration and tags", skip: true, exitCode: true},
		{name: "skip preserves URL configuration and permissions", skip: true, exitCode: true, functionURL: true, wantDiff: true, wantURL: true, wantPermissions: true},
		{name: "skip preserves permissions only differences", skip: true, exitCode: true, functionURL: true, sameURL: true, wantDiff: true, wantPermissions: true},
		{name: "same code and URL resources have no differences", skip: true, exitCode: true, functionURL: true, sameURL: true, matchingPermissions: true},
		{name: "URL exit code remains opt in", skip: true, functionURL: true, wantURL: true, wantPermissions: true},
		{name: "URL API error is returned when skipping configuration", skip: true, functionURL: true, urlStatus: http.StatusForbidden, wantError: "failed to get function url config"},
		{name: "changed code implies code comparison", skip: true, exitCode: true, changedCode: true, wantDiff: true, wantCode: true},
		{name: "explicit code combines with skip", skip: true, code: true, exitCode: true, changedCode: true, wantDiff: true, wantCode: true},
		{name: "exit code remains opt in", skip: true, changedCode: true, wantCode: true},
		{name: "default configuration diff is preserved", exitCode: true, wantDiff: true, wantConfiguration: true},
		{name: "code without skip still includes configuration", code: true, exitCode: true, changedCode: true, wantDiff: true, wantCode: true, wantConfiguration: true},
		{name: "default URL diff is preserved", functionURL: true, skipFunction: true, exitCode: true, wantDiff: true, wantURL: true, wantPermissions: true},
		{name: "container images are not supported", skip: true, image: true, localImage: true, wantError: "only supported for Zip"},
		{name: "local image cannot be compared as zip", skip: true, localImage: true, wantError: "only supported for Zip"},
		{name: "missing function is not unchanged code", skip: true, getStatus: http.StatusNotFound, wantError: "not found; cannot compare code"},
		{name: "get function error is returned", skip: true, getStatus: http.StatusForbidden, wantError: "failed to GetFunction"},
		{name: "missing source is returned", skip: true, missingSource: true, wantError: "is not found"},
		{name: "skip function conflicts with code only", skip: true, skipFunction: true, wantError: "cannot be combined"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			emptyConfig := filepath.Join(dir, "aws-config")
			if err := os.WriteFile(emptyConfig, nil, 0600); err != nil {
				t.Fatal(err)
			}
			for key, value := range map[string]string{
				"AWS_ACCESS_KEY_ID": "testing", "AWS_SECRET_ACCESS_KEY": "testing", "AWS_SESSION_TOKEN": "",
				"AWS_PROFILE": "", "AWS_CONFIG_FILE": emptyConfig, "AWS_SHARED_CREDENTIALS_FILE": emptyConfig,
				"AWS_EC2_METADATA_DISABLED": "true",
			} {
				t.Setenv(key, value)
			}

			var archive bytes.Buffer
			zw := zip.NewWriter(&archive)
			entry, err := zw.Create("index.js")
			if err != nil {
				t.Fatal(err)
			}
			if _, err := entry.Write([]byte("exports.handler = async () => 'hello';")); err != nil {
				t.Fatal(err)
			}
			if err := zw.Close(); err != nil {
				t.Fatal(err)
			}
			src := filepath.Join(dir, "function.zip")
			if err := os.WriteFile(src, archive.Bytes(), 0600); err != nil {
				t.Fatal(err)
			}
			hash := sha256.Sum256(archive.Bytes())
			remoteHash := base64.StdEncoding.EncodeToString(hash[:])
			if tt.changedCode {
				changed := sha256.Sum256([]byte("different function code"))
				remoteHash = base64.StdEncoding.EncodeToString(changed[:])
			}
			if tt.missingSource {
				src = filepath.Join(dir, "missing.zip")
			}

			local := map[string]any{
				"FunctionName": "diff-test", "Runtime": "nodejs22.x", "Handler": "index.handler",
				"Role": "arn:aws:iam::000000000000:role/test", "MemorySize": 256,
				"Tags": map[string]string{"environment": "local"},
			}
			if tt.localImage {
				local["PackageType"] = "Image"
				local["Code"] = map[string]any{"ImageUri": "registry.invalid/test:latest"}
			}
			function := filepath.Join(dir, "function.json")
			data, err := json.Marshal(local)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(function, data, 0600); err != nil {
				t.Fatal(err)
			}
			functionURL := ""
			if tt.functionURL {
				functionURL = filepath.Join(dir, "function_url.json")
				if err := os.WriteFile(functionURL, []byte(`{"Config":{"Qualifier":"live","AuthType":"NONE","Cors":{"AllowOrigins":["https://example.invalid"]}}}`), 0600); err != nil {
					t.Fatal(err)
				}
			}

			var mu sync.Mutex
			calls := map[string]int{}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				mu.Lock()
				calls[r.URL.Path]++
				mu.Unlock()
				w.Header().Set("Content-Type", "application/json")
				if r.Method != http.MethodGet {
					t.Errorf("unexpected mutating AWS request: %s %s", r.Method, r.URL.Path)
					w.WriteHeader(http.StatusForbidden)
					return
				}
				switch {
				case r.URL.Path == "/2015-03-31/functions/diff-test":
					if r.URL.Query().Get("Qualifier") != "live" {
						t.Errorf("qualifier not preserved: %s", r.URL.RawQuery)
					}
					if tt.getStatus != 0 {
						w.WriteHeader(tt.getStatus)
						errorType := "AccessDeniedException"
						if tt.getStatus == http.StatusNotFound {
							errorType = "ResourceNotFoundException"
						}
						_ = json.NewEncoder(w).Encode(map[string]string{"__type": errorType, "message": "test response"})
						return
					}
					packageType := "Zip"
					code := map[string]any{}
					if tt.image {
						packageType = "Image"
						code["ImageUri"] = "registry.invalid/test:latest"
					}
					_ = json.NewEncoder(w).Encode(map[string]any{"Configuration": map[string]any{
						"FunctionName": "diff-test", "Runtime": "nodejs22.x", "Handler": "index.handler", "MemorySize": 128,
						"Role": "arn:aws:iam::000000000000:role/test", "PackageType": packageType, "CodeSha256": remoteHash,
					}, "Code": code})
				case strings.HasPrefix(r.URL.Path, "/2017-03-31/tags/"):
					_ = json.NewEncoder(w).Encode(map[string]any{"Tags": map[string]string{"environment": "remote"}})
				case r.URL.Path == "/2021-10-31/functions/diff-test/url":
					if r.URL.Query().Get("Qualifier") != "live" {
						t.Errorf("URL qualifier not preserved: %s", r.URL.RawQuery)
					}
					if tt.urlStatus != 0 {
						w.WriteHeader(tt.urlStatus)
						_ = json.NewEncoder(w).Encode(map[string]string{"__type": "AccessDeniedException", "message": "test response"})
						return
					}
					auth, origin := "AWS_IAM", "https://remote.invalid"
					if tt.sameURL {
						auth, origin = "NONE", "https://example.invalid"
					}
					_ = json.NewEncoder(w).Encode(map[string]any{"AuthType": auth, "InvokeMode": "BUFFERED", "Cors": map[string]any{"AllowOrigins": []string{origin}}})
				case r.URL.Path == "/2015-03-31/functions/diff-test/policy":
					if r.URL.Query().Get("Qualifier") != "live" {
						t.Errorf("policy qualifier not preserved: %s", r.URL.RawQuery)
					}
					policy := lambroll.PolicyOutput{Version: "2012-10-17", Statement: []lambroll.PolicyStatement{}}
					if tt.matchingPermissions {
						config := &lambroll.FunctionURL{Config: &lambroll.FunctionURLConfig{FunctionName: new("diff-test"), Qualifier: new("live"), AuthType: types.FunctionUrlAuthTypeNone}}
						permission := &lambroll.FunctionURLPermission{Principal: new("*")}
						for _, input := range permission.AddPermissionInputs(config) {
							policy.Statement = append(policy.Statement, lambroll.PolicyStatement{Sid: aws.ToString(input.StatementId), Effect: "Allow", Principal: "*", Action: aws.ToString(input.Action)})
						}
					}
					encoded, err := json.Marshal(policy)
					if err != nil {
						t.Error(err)
						w.WriteHeader(http.StatusInternalServerError)
						return
					}
					_ = json.NewEncoder(w).Encode(map[string]string{"Policy": string(encoded)})
				default:
					t.Errorf("unexpected AWS request: %s", r.URL.Path)
					w.WriteHeader(http.StatusForbidden)
				}
			}))
			defer server.Close()
			ctx := context.Background()
			app, err := lambroll.New(ctx, &lambroll.Option{Function: function, Region: new("ap-northeast-1"), Endpoint: new(server.URL)})
			if err != nil {
				t.Fatal(err)
			}
			app.CallerIdentity().Resolver = func(context.Context) (*sts.GetCallerIdentityOutput, error) {
				return &sts.GetCallerIdentityOutput{Account: new("000000000000"), Arn: new("arn:aws:iam::000000000000:user/test"), UserId: new("test")}, nil
			}
			opt := &lambroll.DiffOption{Src: src, CodeSha256: tt.code, SkipConfiguration: tt.skip, SkipFunction: tt.skipFunction, ExitCode: tt.exitCode, Qualifier: new("live"), FunctionURL: functionURL}
			var output bytes.Buffer
			opt.SetWriter(&output)
			err = app.Diff(ctx, opt)
			if tt.wantDiff {
				if !errors.Is(err, lambroll.ErrDiff) {
					t.Fatalf("expected ErrDiff, got %v", err)
				}
				status, other := lambroll.ExtractExitCodeAndError(err)
				if status != 2 || other != nil {
					t.Errorf("unexpected CLI exit: %d, %v", status, other)
				}
			} else if tt.wantError != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantError) {
					t.Fatalf("expected error containing %q, got %v", tt.wantError, err)
				}
			} else if err != nil {
				t.Fatal(err)
			}
			if got := strings.Contains(output.String(), "CodeSha256:"); got != tt.wantCode {
				t.Errorf("code diff=%t, want %t: %s", got, tt.wantCode, output.String())
			}
			if got := strings.Contains(output.String(), "MemorySize"); got != tt.wantConfiguration {
				t.Errorf("configuration diff=%t, want %t: %s", got, tt.wantConfiguration, output.String())
			}
			if got := strings.Contains(output.String(), "AllowOrigins"); got != tt.wantURL {
				t.Errorf("URL diff=%t, want %t: %s", got, tt.wantURL, output.String())
			}
			if got := strings.Contains(output.String(), "lambda:InvokeFunctionUrl"); got != tt.wantPermissions {
				t.Errorf("permissions diff=%t, want %t: %s", got, tt.wantPermissions, output.String())
			}
			mu.Lock()
			defer mu.Unlock()
			if tt.skip {
				if strings.Contains(output.String(), "Tags") {
					t.Errorf("skipped tags appear in diff: %s", output.String())
				}
				for path, count := range calls {
					allowed := path == "/2015-03-31/functions/diff-test" || (tt.functionURL && (path == "/2021-10-31/functions/diff-test/url" || path == "/2015-03-31/functions/diff-test/policy"))
					if !allowed {
						t.Errorf("skipping configuration made %d unnecessary requests to %s", count, path)
					}
				}
			}
			if tt.skip && tt.skipFunction && len(calls) != 0 {
				t.Errorf("conflicting flags made AWS requests: %v", calls)
			}
			if tt.functionURL && tt.wantError == "" && (calls["/2021-10-31/functions/diff-test/url"] != 1 || calls["/2015-03-31/functions/diff-test/policy"] != 1) {
				t.Errorf("expected URL and policy requests: %v", calls)
			}
		})
	}
}

func TestDiffSkipConfigurationOptionFile(t *testing.T) {
	file := filepath.Join(t.TempDir(), "option.json")
	if err := os.WriteFile(file, []byte(`{"diff":{"skip_configuration":true}}`), 0600); err != nil {
		t.Fatal(err)
	}
	_, options, _, err := lambroll.ParseCLI([]string{"diff", "--option", file})
	if err != nil {
		t.Fatal(err)
	}
	if !options.Diff.SkipConfiguration {
		t.Fatal("option file did not enable skip_configuration")
	}
	_, options, _, err = lambroll.ParseCLI([]string{"diff", "--option", file, "--skip-configuration=false"})
	if err != nil {
		t.Fatal(err)
	}
	if options.Diff.SkipConfiguration {
		t.Fatal("explicit CLI false did not override option file")
	}
}
