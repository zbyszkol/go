package wallet

import (
	"crypto/hmac"
	"crypto/sha256"
	"math"
	"time"

	"crypto/rand"

	"github.com/stellar/go/support/db"
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

// DefaultKdfParams represents the default kdf params to use when deriving the
// master key.  It presently targets a ~250ms scrypt runtime on a mid 2012
// 2.7ghz i7 macbook pro.
var DefaultKdfParams = KdfParams{
	N:    int(math.Pow(2, 16)),
	R:    8,
	P:    1,
	Bits: 256,
}

// Client represents a type that can act as the client role in the Wallet
// protocol. Clients provide authentication information (usernames, passwords,
// totp codes) used to download and unlock wallets.
type Client interface {
	// ReadUsername reads the username that the client wishes to login as.
	ReadUsername() (string, error)
}

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

type LocalServer struct {
	DB *db.Repo
}

// LoginParams represents the login parameters that can be used to finish a
// login request.  When combined with the password, the master key can be
// derived for Username.
type LoginParams struct {
	Username  string
	Salt      []byte
	KdfParams KdfParams
}

// Protocol is the manager for the wallet protocol.  It implements the various
// interactions between client and server.
type Protocol struct {
	Client Client
	Server Server
}

// Server represents a type that conforms to the "server" role in the wallet
// protocol.  It protects access to a locked wallet, only releasing the wallet
// data when the client has proved it is access to the wallet.
type Server interface {
	GetLoginParams(username string) (*LoginParams, error)
	GetByWalletID(username string, walletid string) (*Wallet, error)
}

type Storage interface {
}

type LockedWallet struct {
	LockVersion  int
	MainData     []byte
	KeychainData []byte
	UpdatedAt    time.Time
}

type Wallet struct {
	Username      string
	WalletID      []byte
	Salt          []byte
	KdfParams     KdfParams
	PublicKey     string
	MainData      []byte
	KeychainData  []byte
	UsernameProof []byte
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

// Unlock unlocks the provided locked wallet using the WalletKey derived from
// the provided master key.
func Unlock(lw LockedWallet, master []byte) ([]byte, error) {
	return nil, nil
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
