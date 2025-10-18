package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func withEnv(k, v string, fn func()) {
	old, had := os.LookupEnv(k)
	_ = os.Setenv(k, v)
	defer func() {
		if had {
			_ = os.Setenv(k, old)
		} else {
			_ = os.Unsetenv(k)
		}
	}()
	fn()
}

func Test_firstNonEmpty(t *testing.T) {
	tests := []struct {
		name string
		in   []string
		want string
	}{
		{"all empty", []string{"", "", ""}, ""},
		{"first non-empty", []string{"a", "b"}, "a"},
		{"later non-empty", []string{"", "b"}, "b"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := firstNonEmpty(tt.in...)
			if got != tt.want {
				t.Errorf("firstNonEmpty() got=%#v want=%#v", got, tt.want)
			}
		})
	}
}

func Test_getEnv(t *testing.T) {
	tests := []struct {
		name string
		setK string
		setV string
		key  string
		def  string
		want string
	}{
		{"no env uses default non-empty", "", "", "FOO", "bar", "bar"},
		{"env overrides", "FOO", "baz", "FOO", "bar", "baz"},
		{"default empty stays empty", "", "", "FOO", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setK != "" {
				withEnv(tt.setK, tt.setV, func() {
					got := getEnv(tt.key, tt.def)
					if got != tt.want {
						t.Errorf("getEnv() got=%#v want=%#v", got, tt.want)
					}
				})
				return
			}
			got := getEnv(tt.key, tt.def)
			if got != tt.want {
				t.Errorf("getEnv() got=%#v want=%#v", got, tt.want)
			}
		})
	}
}

func Test_getEnvInt(t *testing.T) {
	tests := []struct {
		name string
		set  string
		def  int
		want int
	}{
		{"no env -> default", "", 7, 7},
		{"valid int", "42", 7, 42},
		{"invalid int -> default", "abc", 9, 9},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.set == "" {
				_ = os.Unsetenv("XINT")
			} else {
				_ = os.Setenv("XINT", tt.set)
				defer os.Unsetenv("XINT")
			}
			got := getEnvInt("XINT", tt.def)
			if got != tt.want {
				t.Errorf("getEnvInt() got=%#v want=%#v", got, tt.want)
			}
		})
	}
}

func Test_Config_HTTPAddr(t *testing.T) {
	tests := []struct {
		name string
		port int
		want string
	}{
		{"default", 8080, "0.0.0.0:8080"},
		{"custom", 9090, "0.0.0.0:9090"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Config{MetricsPort: tt.port}
			if got := c.HTTPAddr(); got != tt.want {
				t.Errorf("HTTPAddr() got=%#v want=%#v", got, tt.want)
			}
		})
	}
}

func Test_Config_Redacted(t *testing.T) {
	c := &Config{GoogleProjectID: "pid", Subscription: "sub", PubsubTopic: "topic", TargetNamespace: "ns", MetricsPort: 8081, LogLevel: "debug", CredentialsFile: "creds.json"}
	got := c.Redacted()
	want := map[string]any{
		"projectID":           "pid",
		"requestSubscription": "sub",
		"resultTopic":         "topic",
		"targetNamespace":     "ns",
		"metricsPort":         8081,
		"logLevel":            "debug",
		"credentialsProvided": true,
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Redacted()\n got=%#v\nwant=%#v", got, want)
	}
}

func Test_projectIDFromCredentials(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "creds.json")
	content := []byte(`{"project_id":"my-proj"}`)
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatalf("write temp creds: %#v", err)
	}
	pid, err := projectIDFromCredentials(path)
	if err != nil || pid != "my-proj" {
		t.Errorf("projectIDFromCredentials() pid=%#v err=%#v", pid, err)
	}

	// invalid json returns empty id, no error
	if err := os.WriteFile(path, []byte(`{"nope":1}`), 0o600); err != nil {
		t.Fatalf("write temp creds: %#v", err)
	}
	pid2, err2 := projectIDFromCredentials(path)
	if err2 != nil || pid2 != "" {
		t.Errorf("projectIDFromCredentials(invalid) pid=%#v err=%#v", pid2, err2)
	}
}

func Test_getGoogleProjectID(t *testing.T) {
	unset := func(keys ...string) {
		for _, k := range keys {
			_ = os.Unsetenv(k)
		}
	}
	// ensure clean env
	unset("GOOGLE_APPLICATION_CREDENTIALS", "ALLOCATOR_PUBSUB_PROJECT_ID", "GOOGLE_PROJECT_ID", "GOOGLE_CLOUD_PROJECT", "GCLOUD_PROJECT", "GCP_PROJECT")

	dir := t.TempDir()
	credFile := filepath.Join(dir, "creds.json")
	_ = os.WriteFile(credFile, []byte(`{"project_id":"file-proj"}`), 0o600)

	tests := []struct {
		name     string
		setEnv   map[string]string
		creds    string
		explicit string
		want     string
	}{
		{"from GOOGLE_APPLICATION_CREDENTIALS", map[string]string{"GOOGLE_APPLICATION_CREDENTIALS": credFile}, "", "", "file-proj"},
		{"from explicit ALLOCATOR_PUBSUB_PROJECT_ID", map[string]string{}, "", "explicit-proj", "explicit-proj"},
		{"from GOOGLE_PROJECT_ID", map[string]string{"GOOGLE_PROJECT_ID": "env-proj"}, "", "", "env-proj"},
		{"from common env", map[string]string{"GOOGLE_CLOUD_PROJECT": "common-proj"}, "", "", "common-proj"},
		{"from provided credsFile path", map[string]string{}, credFile, "", "file-proj"},
		{"none -> empty", map[string]string{}, "", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// reset env
			unset("GOOGLE_APPLICATION_CREDENTIALS", "ALLOCATOR_PUBSUB_PROJECT_ID", "GOOGLE_PROJECT_ID", "GOOGLE_CLOUD_PROJECT", "GCLOUD_PROJECT", "GCP_PROJECT")
			for k, v := range tt.setEnv {
				_ = os.Setenv(k, v)
			}
			got := getGoogleProjectID(tt.creds, tt.explicit)
			if got != tt.want {
				t.Errorf("getGoogleProjectID() got=%#v want=%#v", got, tt.want)
			}
		})
	}
}

func Test_Load(t *testing.T) {
	// Use only environment inputs to load; avoid panics
	unset := func(keys ...string) {
		for _, k := range keys {
			_ = os.Unsetenv(k)
		}
	}
	unset("ALLOCATION_REQUEST_SUBSCRIPTION", "ALLOCATION_RESULT_TOPIC", "TARGET_NAMESPACE", "ALLOCATOR_METRICS_PORT", "ALLOCATOR_LOG_LEVEL", "GOOGLE_APPLICATION_CREDENTIALS", "ALLOCATOR_GSA_CREDENTIALS", "ALLOCATOR_PUBSUB_PROJECT_ID")

	os.Setenv("ALLOCATION_REQUEST_SUBSCRIPTION", "sub")
	os.Setenv("ALLOCATION_RESULT_TOPIC", "topic")
	os.Setenv("TARGET_NAMESPACE", "ns")
	os.Setenv("ALLOCATOR_METRICS_PORT", "7777")
	os.Setenv("ALLOCATOR_LOG_LEVEL", "warn")
	defer unset("ALLOCATION_REQUEST_SUBSCRIPTION", "ALLOCATION_RESULT_TOPIC", "TARGET_NAMESPACE", "ALLOCATOR_METRICS_PORT", "ALLOCATOR_LOG_LEVEL")

	cfg := Load()
	if cfg == nil {
		t.Fatalf("Load() returned nil")
	}
	if cfg.Subscription != "sub" || cfg.PubsubTopic != "topic" || cfg.TargetNamespace != "ns" || cfg.MetricsPort != 7777 || cfg.LogLevel != "warn" {
		b, _ := json.Marshal(cfg)
		t.Errorf("Load() unexpected cfg: %#v", string(b))
	}
}
