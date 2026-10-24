package jira

import "testing"

func TestParseIssueRef(t *testing.T) {
	base := "https://jira.stil.dk"
	tests := []struct {
		name    string
		arg     string
		want    string
		wantErr bool
	}{
		{"bare key", "SKOLELOGIN-18312", "SKOLELOGIN-18312", false},
		{"browse url", "https://jira.stil.dk/browse/SKOLELOGIN-18312", "SKOLELOGIN-18312", false},
		{"browse url with query", "https://jira.stil.dk/browse/ABC-1?focusedId=5", "ABC-1", false},
		{"selectedIssue", "https://jira.stil.dk/secure/RapidBoard.jspa?rapidView=9&selectedIssue=ABC-77", "ABC-77", false},
		{"host case", "https://JIRA.stil.dk/browse/ABC-2", "ABC-2", false},
		{"wrong host", "https://other.example.com/browse/ABC-1", "", true},
		{"no key in url", "https://jira.stil.dk/projects/ABC", "", true},
		{"garbage", "hello", "", true},
		{"lowercase key", "abc-1", "", true},
		{"empty", "", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseIssueRef(tt.arg, base)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Fatalf("key = %q, want %q", got, tt.want)
			}
			if err != nil && !contains(err.Error(), "jirafs pull KEY --project <name>") {
				t.Fatalf("error %q lacks copy-pasteable hint", err)
			}
		})
	}
}
