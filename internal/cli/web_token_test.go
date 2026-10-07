package cli

import (
	"testing"

	"github.com/gookit/goutil/x/assert"
)

func TestWebResolveToken(t *testing.T) {
	lookup := func(values map[string]string) func(string) (string, bool) {
		return func(key string) (string, bool) {
			value, ok := values[key]
			return value, ok
		}
	}

	tests := []struct {
		name       string
		flag       string
		config     string
		env        map[string]string
		wantToken  string
		wantSource string
	}{
		{
			name: "the flag wins over the environment and config",
			flag: "from-flag", config: "from-config", env: map[string]string{webTokenEnv: "from-env"},
			wantToken: "from-flag", wantSource: webTokenFromFlag,
		},
		{
			name: "the environment is used when the flag is blank",
			flag: "  ", config: "from-config", env: map[string]string{webTokenEnv: " from-env "},
			wantToken: "from-env", wantSource: webTokenFromEnv,
		},
		{
			name:   "config fills in when the flag and environment are blank",
			config: " from-config ", env: map[string]string{webTokenEnv: "   "},
			wantToken: "from-config", wantSource: webTokenFromConfig,
		},
		{
			name:      "a blank environment is ignored",
			env:       map[string]string{webTokenEnv: "   "},
			wantToken: "", wantSource: webTokenFromNoSource,
		},
		{
			name:      "no source at all",
			wantToken: "", wantSource: webTokenFromNoSource,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token, source := webResolveToken(tt.flag, tt.config, lookup(tt.env))

			assert.Eq(t, tt.wantToken, token)
			assert.Eq(t, tt.wantSource, source)
		})
	}
}

func TestWebAllowHosts(t *testing.T) {
	got := webAllowHosts([]string{" 192.168.1.10 ", "MyHost", ""}, "myhost, build.local ,*")

	assert.Eq(t, []string{"192.168.1.10", "MyHost", "build.local", "*"}, got)
}
