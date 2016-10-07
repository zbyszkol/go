// +build go1.5,go1.6,!go1.7

package cryptotest

import "testing"

func runTest(scenario string, t *testing.T, fn func(t *testing.T)) {
	fn(t)
}
