// +build go1.7

package cryptotest

import (
	"testing"
)

func runTest(scenario string, t *testing.T, fn func(t *testing.T)) {
	t.Run(scenario, fn)
}
