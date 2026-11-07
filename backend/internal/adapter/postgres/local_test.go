package postgres

import "testing"

func TestLocalPort(t *testing.T) {
	for _, tc := range []struct{ value, want string }{
		{"", "5432"}, {"1", "1"}, {"65535", "65535"}, {"05432", "5432"},
		{"0", ""}, {"65536", ""}, {"-1", ""}, {"+5432", ""}, {" 5432", ""},
		{"5432 host=external", ""}, {"５４３２", ""}, {"999999999999999999999", ""},
	} {
		t.Run(tc.value, func(t *testing.T) {
			t.Setenv("DB_PORT", tc.value)
			got, err := localPort()
			if got != tc.want || (err != nil) != (tc.want == "") {
				t.Fatal("unexpected local port validation")
			}
		})
	}
}
