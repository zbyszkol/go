// Package cryptotest implement test helpers for testing the the properties of
// of a cryptographic function.
package cryptotest

import (
	"math/rand"
	"testing"

	"github.com/lazybeaver/entropy"
	"github.com/stellar/go/support/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var DefaultConfig = &Config{
	MaxCount: 10000,
}

type Config struct {
	MaxCount int
}

// Generator represents a function that generates a slice of bytes from no input
type Generator func() ([]byte, error)

// Entropy calculates the shannon entropy of the output of a single call to the
// provided generator.
func Entropy(fn Generator) (float64, error) {
	e := entropy.NewShannonEstimator()
	out, err := fn()
	if err != nil {
		return 0.0, errors.Wrap(err, "call fn failed")
	}

	e.Write(out)
	return e.Value(), nil
}

// Random returns a new random generator that produces random byte slice of
// the provided length
func Random(bytes int) Generator {
	return func() ([]byte, error) {
		ret := make([]byte, bytes)
		_, err := rand.Read(ret)
		if err != nil {
			return nil, errors.Wrap(err, "read random failed")
		}

		return ret, nil
	}
}

// Randomness is a test that asserts that the shannon entropy of the provided
// random function is near the shannon entropy of a cryptographically random
// byte stream.  This output of the provided function is fed into the entropy
// intstrument n times. NOTE(scott): The delta threshold is subjective, as I'm a
// bit out of my depths with regards to information theory.  If anyone has
// guidance or feedback about the quality of this method of testing, please
// contact me.
func Randomness(scenario string, t *testing.T, fn Generator, cfg *Config) {
	runTest("randomness: "+scenario, t, func(t *testing.T) {
		if cfg == nil {
			cfg = DefaultConfig
		}
		rande, err := Entropy(Random(cfg.MaxCount))
		require.NoError(t, err)
		gene := entropy.NewShannonEstimator()

		for i := 0; i < cfg.MaxCount; i++ {

			gen, err := fn()
			if !assert.NoError(t, err) {
				return
			}

			_, err = gene.Write(gen)
			require.NoError(t, err)
		}

		assert.InDelta(t, rande, gene.Value(), 0.1, "randomness is low")
	})
}
