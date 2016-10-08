package wallet

import (
	"strings"
	"testing"

	"net/http/httptest"

	"github.com/stretchr/testify/assert"
)

func TestRequest_ReadUsername(t *testing.T) {
	cases := []struct {
		Name     string
		Body     string
		Expected string
		Err      string
	}{
		{
			Name:     "valid username",
			Body:     `{ "username" : "scott" }`,
			Expected: "scott",
			Err:      "",
		}, {
			Name:     "empty username",
			Body:     `{ "username" : "" }`,
			Expected: "",
			Err:      "body validation failed",
		},
		{
			Name:     "too short",
			Body:     `{ "username" : "s" }`,
			Expected: "",
			Err:      "body validation failed",
		},
		{
			Name:     "too long",
			Body:     `{ "username" : "` + strings.Repeat("a", 256) + `" }`,
			Expected: "",
			Err:      "body validation failed",
		},
		{
			Name:     "invalid json",
			Body:     `{ "username" : `,
			Expected: "",
			Err:      "decode body failed",
		},
	}

	for _, kase := range cases {
		t.Run(kase.Name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/", strings.NewReader(kase.Body))
			wreq := &Request{HTTP: req}

			actual, err := wreq.ReadUsername()
			if kase.Err != "" {
				if assert.Error(t, err) {
					assert.Contains(t, err.Error(), kase.Err, "error is wrong")
				}
				return
			}

			if assert.NoError(t, err) {
				assert.Equal(t, kase.Expected, actual, "body is wrong")
			}
		})
	}

	// valid

	// too short
	// too long
	// invalid json
}
