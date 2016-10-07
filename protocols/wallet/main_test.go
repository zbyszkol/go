package wallet

import (
	"math"
	"testing"
	"time"

	"github.com/icrowley/fake"
	"github.com/stellar/go/support/cryptotest"
	"github.com/stellar/go/support/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewS0(t *testing.T) {
	cryptotest.Randomness("NewS0", t, NewS0, nil)

	// test length
	s0, err := NewS0()
	if assert.NoError(t, err) {
		assert.Len(t, s0, 32)
	}
}

func TestMasterKey(t *testing.T) {
	// the entropy tests use small scrypt params to keep the test quick. These
	// tests are not testing the key streching properties of the algorithm.
	quickKdf := KdfParams{
		N:    8,
		R:    8,
		P:    1,
		Bits: 256,
	}

	cryptotest.Randomness("random s0, random name, random password", t, func() ([]byte, error) {
		s0, err := NewS0()
		if err != nil {
			return nil, errors.Wrap(err, "s0 failed")
		}
		u := fake.UserName()
		p := fake.Password(4, 10, true, true, true)

		master, err := MasterKey(s0, u, p, quickKdf)
		if err != nil {
			return nil, errors.Wrap(err, "master key failed")
		}
		return master, nil
	}, nil)

	s0, err := NewS0()
	require.NoError(t, err)
	cryptotest.Randomness("same s0, random name, random password", t, func() ([]byte, error) {
		u := fake.UserName()
		p := fake.Password(4, 10, true, true, true)

		master, err := MasterKey(s0, u, p, quickKdf)
		if err != nil {
			return nil, errors.Wrap(err, "master key failed")
		}
		return master, nil
	}, nil)

	u := fake.UserName()
	cryptotest.Randomness("random s0, same name, random password", t, func() ([]byte, error) {
		s0, err := NewS0()
		if err != nil {
			return nil, errors.Wrap(err, "s0 failed")
		}
		p := fake.Password(4, 10, true, true, true)

		master, err := MasterKey(s0, u, p, quickKdf)
		if err != nil {
			return nil, errors.Wrap(err, "master key failed")
		}
		return master, nil
	}, nil)

	p := fake.Password(4, 10, true, true, true)
	cryptotest.Randomness("random s0, random name, same password", t, func() ([]byte, error) {
		s0, err := NewS0()
		if err != nil {
			return nil, errors.Wrap(err, "s0 failed")
		}
		u := fake.UserName()

		master, err := MasterKey(s0, u, p, quickKdf)
		if err != nil {
			return nil, errors.Wrap(err, "master key failed")
		}
		return master, nil
	}, nil)

	cryptotest.Randomness("same s0, same name, random password", t, func() ([]byte, error) {

		p := fake.Password(4, 10, true, true, true)

		master, err := MasterKey(s0, u, p, quickKdf)
		if err != nil {
			return nil, errors.Wrap(err, "master key failed")
		}
		return master, nil
	}, nil)

	cryptotest.Randomness("same s0, random name, same password", t, func() ([]byte, error) {

		u := fake.UserName()

		master, err := MasterKey(s0, u, p, quickKdf)
		if err != nil {
			return nil, errors.Wrap(err, "master key failed")
		}
		return master, nil
	}, nil)

	cryptotest.Randomness("random s0, same name, same password", t, func() ([]byte, error) {
		s0, err := NewS0()
		if err != nil {
			return nil, errors.Wrap(err, "s0 failed")
		}

		master, err := MasterKey(s0, u, p, quickKdf)
		if err != nil {
			return nil, errors.Wrap(err, "master key failed")
		}
		return master, nil
	}, nil)

	// ensure determinism
	v1, err := MasterKey(s0, u, p, quickKdf)
	require.NoError(t, err)
	v2, err := MasterKey(s0, u, p, quickKdf)
	require.NoError(t, err)

	assert.Equal(t, v1, v2, "master key isn't deterministic")

	// test speed
	start := time.Now()
	_, err = MasterKey(s0, u, p, KdfParams{
		N:    int(math.Pow(2, 16)),
		R:    8,
		P:    1,
		Bits: 256,
	})
	speed := time.Since(start)
	require.NoError(t, err)
	t.Logf("master key speed: %s", speed)
	assert.True(t, speed > (100*time.Millisecond), "master key is too fast: %s", speed)
}

func TestMasterKeySalt(t *testing.T) {
	cryptotest.Randomness("random s0, random name", t, func() ([]byte, error) {
		u := fake.UserName()
		s0, err := NewS0()
		if err != nil {
			return nil, errors.Wrap(err, "s0 failed")
		}

		s := MasterKeySalt(s0, u)
		return s, nil
	}, nil)

	u := fake.UserName()
	cryptotest.Randomness("random s0, same name", t, func() ([]byte, error) {
		s0, err := NewS0()
		if err != nil {
			return nil, errors.Wrap(err, "s0 failed")
		}

		s := MasterKeySalt(s0, u)
		return s, nil
	}, nil)

	s0, err := NewS0()
	require.NoError(t, err)
	cryptotest.Randomness("same s0, random name", t, func() ([]byte, error) {
		u := fake.UserName()
		s := MasterKeySalt(s0, u)
		return s, nil
	}, nil)
}

func TestWalletID(t *testing.T) {
	makeMaster := cryptotest.Random(32)

	cryptotest.Randomness("random master key", t, func() ([]byte, error) {
		km, err := makeMaster()
		if err != nil {
			return nil, errors.Wrap(err, "make master failed")
		}

		return WalletID(km), nil
	}, nil)
}

func TestWalletKey(t *testing.T) {
	makeMaster := cryptotest.Random(32)

	cryptotest.Randomness("random master key", t, func() ([]byte, error) {
		km, err := makeMaster()
		if err != nil {
			return nil, errors.Wrap(err, "make master failed")
		}

		return WalletKey(km), nil
	}, nil)
}
