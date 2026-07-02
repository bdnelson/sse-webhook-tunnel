package main

import "testing"

// envFunc builds a getenv function from a map.
func envFunc(env map[string]string) func(string) string {
	return func(key string) string { return env[key] }
}

func TestParseConfig(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		env     map[string]string
		want    config
		wantErr bool
	}{
		{
			name: "flags provide required values",
			args: []string{"--source", "https://smee.io/abc", "--target", "http://localhost:9000/hook"},
			want: config{
				SourceURL: "https://smee.io/abc",
				TargetURL: "http://localhost:9000/hook",
				LogFile:   defaultLogFile,
			},
		},
		{
			name: "env fallback provides required values",
			args: []string{},
			env: map[string]string{
				"SOURCE_URL": "https://smee.io/env",
				"TARGET_URL": "http://localhost:8000/hook",
			},
			want: config{
				SourceURL: "https://smee.io/env",
				TargetURL: "http://localhost:8000/hook",
				LogFile:   defaultLogFile,
			},
		},
		{
			name: "flag overrides env",
			args: []string{"--source", "https://flag.example"},
			env: map[string]string{
				"SOURCE_URL": "https://env.example",
				"TARGET_URL": "http://localhost/hook",
			},
			want: config{
				SourceURL: "https://flag.example",
				TargetURL: "http://localhost/hook",
				LogFile:   defaultLogFile,
			},
		},
		{
			name: "insecure and log-file",
			args: []string{"--source", "s", "--target", "t", "--insecure", "--log-file", "/tmp/x.log"},
			want: config{SourceURL: "s", TargetURL: "t", LogFile: "/tmp/x.log", Insecure: true},
		},
		{
			name:    "missing source",
			args:    []string{"--target", "t"},
			wantErr: true,
		},
		{
			name:    "missing target",
			args:    []string{"--source", "s"},
			wantErr: true,
		},
		{
			name:    "invalid flag",
			args:    []string{"--nope"},
			wantErr: true,
		},
		{
			name: "insecure via env",
			args: []string{"--source", "s", "--target", "t"},
			env:  map[string]string{"INSECURE": "true"},
			want: config{SourceURL: "s", TargetURL: "t", LogFile: defaultLogFile, Insecure: true},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseConfig(tt.args, envFunc(tt.env))
			if tt.wantErr {
				if err == nil {
					t.Fatalf("parseConfig() expected error, got nil (config=%+v)", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseConfig() error = %v", err)
			}
			if got != tt.want {
				t.Errorf("parseConfig() = %+v, want %+v", got, tt.want)
			}
		})
	}
}
