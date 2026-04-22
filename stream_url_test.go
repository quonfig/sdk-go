package quonfig

import "testing"

func TestDeriveStreamURL(t *testing.T) {
	cases := []struct {
		name  string
		in    string
		want  string
		isErr bool
	}{
		{
			name: "production primary",
			in:   "https://primary.quonfig.com",
			want: "https://stream.primary.quonfig.com",
		},
		{
			name: "localhost with port preserves port",
			in:   "http://localhost:8080",
			want: "http://stream.localhost:8080",
		},
		{
			name: "single-label host with tld-less suffix",
			in:   "https://api-delivery.localhost",
			want: "https://stream.api-delivery.localhost",
		},
		{
			name: "preserves path and scheme",
			in:   "https://primary.quonfig.com/prefix",
			want: "https://stream.primary.quonfig.com/prefix",
		},
		{
			name: "http scheme preserved",
			in:   "http://primary.quonfig.com",
			want: "http://stream.primary.quonfig.com",
		},
		{
			name:  "empty",
			in:    "",
			isErr: true,
		},
		{
			name:  "missing scheme",
			in:    "primary.quonfig.com",
			isErr: true,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got, err := deriveStreamURL(tc.in)
			if tc.isErr {
				if err == nil {
					t.Fatalf("expected error for %q, got %q", tc.in, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error for %q: %v", tc.in, err)
			}
			if got != tc.want {
				t.Fatalf("deriveStreamURL(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}
