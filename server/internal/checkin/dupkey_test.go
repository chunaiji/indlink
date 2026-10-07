package checkin

import (
	"errors"
	"fmt"
	"testing"

	"github.com/go-sql-driver/mysql"
)

func TestIsDuplicateKey(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"dup entry 1062", &mysql.MySQLError{Number: 1062}, true},
		{"deadlock 1213", &mysql.MySQLError{Number: 1213}, false},
		{"generic error", errors.New("some other error"), false},
		{"wrapped 1062", fmt.Errorf("wrap: %w", &mysql.MySQLError{Number: 1062}), true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := isDuplicateKey(tc.err)
			if got != tc.want {
				t.Fatalf("isDuplicateKey(%v) = %v, want %v", tc.err, got, tc.want)
			}
		})
	}
}
