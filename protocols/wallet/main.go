package wallet

import (
	"crypto/hmac"
	"crypto/sha256"

	"crypto/rand"

	"github.com/stellar/go/support/errors"
	"golang.org/x/crypto/scrypt"
)

const (
	// WalletIDBase is the constant value that is HMAC-256'ed using a master key
	// to produce a WalletID value
	WalletIDBase = "WALLET_ID"

	// WalletKeyBase is the constant value that is HMAC-256'ed using a master key
	// to produce a WalletKey value
	WalletKeyBase = "WALLET_KEY"
)

const (
	// VersionByte is the current version byte for this algorithm
	VersionByte = 0x01
)

// KdfParams represents a configuration of the parameters used when deriving the
// master key from a user's username and password.
type KdfParams struct {
	// N is the scrypt N parameter
	N int
	// P is the scrypt r parameter
	R int
	// P is the scrypt p parameter
	P int

	// Bits is the desired key output length in bits
	Bits int
}

// NewS0 returns a new cryptographically random 256-bit byte slice suitable for
// use as the s0 parameter for deriving the master key salt value.
func NewS0() ([]byte, error) {
	ret := make([]byte, 32)
	_, err := rand.Read(ret)
	if err != nil {
		return nil, errors.Wrap(err, "read random source faled")
	}

	return ret, nil
}

// MasterKey derives the K_m (the master key) according to the wallet v2 protocol.
func MasterKey(s0 []byte, username, password string, kdf KdfParams) ([]byte, error) {
	salt := MasterKeySalt(s0, username)
	master, err := scrypt.Key([]byte(password), salt, kdf.N, kdf.R, kdf.P, kdf.Bits/8)
	if err != nil {
		return nil, errors.Wrap(err, "scrypt failed")
	}

	return master, nil
}

// MasterKeySalt derives the salt used as part of the scrypt operation to derive
// the master key.
func MasterKeySalt(s0 []byte, username string) []byte {
	saltBase := []byte{VersionByte}
	saltBase = append(saltBase, s0...)
	saltBase = append(saltBase, []byte(username)...)

	hash := sha256.Sum256(saltBase)

	return hash[:]
}

// WalletID derives a value capable of locating an remote, encrypted wallet
// according to the wallet v2 protocol. WALLET_ID is hmac-sha256'ed using the
// master key.
func WalletID(master []byte) []byte {
	if master == nil {
		return nil
	}

	mac := hmac.New(sha256.New, master)
	mac.Write([]byte(WalletIDBase))
	id := mac.Sum(nil)
	return id
}

// WalletKey derives the cryptographic key capable of unlocking a ciphertext
// encrypted according to the wallet v2 protocol. WALLET_KEY is hmac-sha256'ed
// using the master key.
func WalletKey(master []byte) []byte {
	if master == nil {
		return nil
	}

	mac := hmac.New(sha256.New, master)
	mac.Write([]byte(WalletKeyBase))
	id := mac.Sum(nil)
	return id
}
