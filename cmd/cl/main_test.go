package main

import "testing"

func TestParseArgs(t *testing.T) {
	tests := []struct {
		args  []string
		query string
		act   action
	}{
		{nil, "", actionPick},
		{[]string{"kafka"}, "kafka", actionPick},
		{[]string{"kafka", "consumer"}, "kafka consumer", actionPick},
		{[]string{"--help"}, "", actionHelp},
		{[]string{"-v"}, "", actionVersion},
	}
	for _, tc := range tests {
		q, a := parseArgs(tc.args)
		if q != tc.query || a != tc.act {
			t.Errorf("parseArgs(%q) = %q %v, want %q %v", tc.args, q, a, tc.query, tc.act)
		}
	}
}
