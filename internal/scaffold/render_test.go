package scaffold

import "testing"

func TestRender(t *testing.T) {
	values := Values{"APP_NAME": "demoapp", "PORT": "8080"}

	cases := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "substitutes a known placeholder",
			in:   "module {{APP_NAME}}-api",
			want: "module demoapp-api",
		},
		{
			name: "substitutes multiple placeholders",
			in:   "{{APP_NAME}} listens on {{PORT}}",
			want: "demoapp listens on 8080",
		},
		{
			name: "leaves shell/compose ${VAR} syntax untouched",
			in:   `environment: ["DATABASE_URL=${DATABASE_URL}", "TAG=${API_TAG:-latest}"]`,
			want: `environment: ["DATABASE_URL=${DATABASE_URL}", "TAG=${API_TAG:-latest}"]`,
		},
		{
			name: "leaves an unresolved placeholder intact",
			in:   "secret: {{JWT_SECRET}}",
			want: "secret: {{JWT_SECRET}}",
		},
		{
			name: "mixes resolved, unresolved, and dollar syntax in one file",
			in:   "{{APP_NAME}}: ${UNRELATED} and {{MISSING}}",
			want: "demoapp: ${UNRELATED} and {{MISSING}}",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := string(Render([]byte(tc.in), values))
			if got != tc.want {
				t.Errorf("Render(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestRenderString(t *testing.T) {
	values := Values{"APP_NAME": "demoapp"}
	got := RenderString("files/{{APP_NAME}}-api/main.go", values)
	want := "files/demoapp-api/main.go"
	if got != want {
		t.Errorf("RenderString = %q, want %q", got, want)
	}
}

func TestHasUnresolved(t *testing.T) {
	if HasUnresolved([]byte("all good, ${FINE} too")) {
		t.Error("HasUnresolved false positive on ${VAR} syntax")
	}
	if !HasUnresolved([]byte("missing {{THIS_ONE}}")) {
		t.Error("HasUnresolved false negative on a real placeholder")
	}
}
