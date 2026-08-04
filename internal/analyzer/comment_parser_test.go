package analyzer

import "testing"

func TestHumanizeSummary(t *testing.T) {
	tests := []struct {
		name     string
		summary  string
		funcName string
		want     string
	}{
		{
			name:     "strips the identifier Go doc convention requires",
			summary:  "ListBooks returns the catalogue.",
			funcName: "ListBooks",
			want:     "Returns the catalogue.",
		},
		{
			name:     "capitalises the remaining prose",
			summary:  "GetUser fetches a single user by ID.",
			funcName: "GetUser",
			want:     "Fetches a single user by ID.",
		},
		{
			name:     "leaves a summary that does not start with the name",
			summary:  "Create a new address",
			funcName: "CreateAddress",
			want:     "Create a new address",
		},
		{
			name:     "leaves an already-capitalised next word alone",
			summary:  "GetUser JSON-encodes the user.",
			funcName: "GetUser",
			want:     "JSON-encodes the user.",
		},
		{
			name:     "does not strip a name that is only a prefix of the first word",
			summary:  "GetUserByID returns a user.",
			funcName: "GetUser",
			want:     "GetUserByID returns a user.",
		},
		{
			name:     "handles a comment that is only the identifier",
			summary:  "Health",
			funcName: "Health",
			want:     "Health",
		},
		{
			name:     "empty summary stays empty",
			summary:  "",
			funcName: "Health",
			want:     "",
		},
		{
			name:     "empty func name is a no-op",
			summary:  "Returns the catalogue.",
			funcName: "",
			want:     "Returns the catalogue.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := HumanizeSummary(tt.summary, tt.funcName); got != tt.want {
				t.Errorf("HumanizeSummary(%q, %q) = %q, want %q",
					tt.summary, tt.funcName, got, tt.want)
			}
		})
	}
}
