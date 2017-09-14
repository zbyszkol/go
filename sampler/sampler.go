package sampler

import (
	"bytes"
	"encoding/binary"
	"github.com/stellar/go/build"
	"github.com/stellar/go/keypair"
	"github.com/stellar/go/strkey"
	"github.com/stellar/go/xdr"
	"math"
	"math/rand"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// NOTE: we are storing private keys in AccountId (expects public key)
type AccountEntry struct {
	xdr.AccountEntry
	// seed    uint64
	Keypair SeedProvider
}

func (entry *AccountEntry) SetAccountEntry(xdrEntry *xdr.AccountEntry) *AccountEntry {
	entry.AccountEntry = *xdrEntry
	return entry
}

func (entry *AccountEntry) GetAccountEntry() *xdr.AccountEntry {
	return &entry.AccountEntry
}

type TrustLineEntry struct {
	xdr.TrustLineEntry
	Keypair SeedProvider
}

// type TransactionsChanges struct {
// 	OperationChanges []OperationChange
// }

// type OperationChange struct {

// }

type SeedProvider interface {
	GetSeed() *keypair.Full
}

type Uint64 uint64

func (seed Uint64) GetSeed() *keypair.Full {
	var bytesData []byte = make([]byte, 32)
	binary.LittleEndian.PutUint64(bytesData, seed)
	return fromRawSeed(byteData)
}

func (full *keypair.Full) GetSeed() *keypair.Full {
	return full
}

type Database interface {
	GetAccountByOrder(int) *AccountEntry
	GetAccountByAddress(keypair.KP) *AccountEntry
	AddAccount(*AccountEntry) *Database
	GetAccountsCount() int
	GetTrustLineByOrder(int) *TrustLineEntry
	// GetTrustLineById(accountAddress keypair.KP, issuer keypair.KP, assetcode string) *TrustLineEntry
	GetTrustLineCount() int
	AddTrustLine(*TrustLineEntry) *Database
}

func AddRootAccount(database Database, sequenceProvider SequenceProvider) (Database, error) {
	rootSeed := "SDHOAMBNLGCE2MV5ZKIVZAQD3VCLGP53P3OBSBI6UN5L5XZI5TKHFQL4"
	seedBytes := seedStringToBytes(rootSeed)
	fullKP, keyErr := fromRawSeed(seedBytes)
	if keyErr != nil {
		return database, keyErr
	}
	rootSequenceNumber, seqErr := sequenceProvider.FetchSequenceNumber(fullKP.Address())
	if seqErr != nil {
		return database, seqErr
	}
	rootAccount := &AccountEntry{Keypair: fullKP, Balance: 1000000000000000000, SeqNum: rootSequenceNumber + 1}
	return database.AddAccount(rootAccount), nil
}

func seedStringToBytes(seed string) []byte {
	return strkey.MustDecode(strkey.VersionByteSeed, seed)
}

type InMemoryDatabase struct {
	orderedData       *CircularBuffer
	mappedData        map[string]*AccountEntry
	orderedTrustlines *CircularBuffer
	// mappedTrustlines  map[string]*TrustLineEntry
}

func (data *InMemoryDatabase) GetAccountByOrder(order int) *AccountEntry {
	return data.orderedData.Get(order).(*AccountEntry)
}

func (data *InMemoryDatabase) GetAccountsCount() int {
	return data.orderedData.Count()
}

func (data *InMemoryDatabase) AddAccount(account *AccountEntry) {
	data.orderedData.Add(account)
}

func (data *InMemoryDatabase) GetTrustLineByOrder(ix int) *TrustLineEntry {
	return data.orderedTrustlines.Get(ix).(*TrustLineEntry)
}

func (data *InMemoryDatabase) GetTrustLineCount() int {
	return data.orderedTrustlines.Count()
}

func (data *InMemoryDatabase) AddTrustLine(trustline *TrustLineEntry) {
	data.orderedTrustlines.Add(trustline)
}

func (data *InMemoryDatabase) Map(publicKey [32]byte) SeedProvider {
	publicKeyString := rawKeyToString(publicKey)
	data := data.mappedData[publicKeyString]
	return data.Keypair
}

type Size uint64

type MutatorGenerator func(Size, Database) build.TransactionMutator // , xdr.OperationMeta)

type PublicToSeedMapper interface {
	Map(publicKey [32]byte) SeedProvider
}

type generatorsList []struct {
	generator func(*AccountEntry) MutatorGenerator
	bias      uint32
}

type TransactionsSampler struct {
	database   Database
	generators func(*AccountEntry) MutatorGenerator
}

func CreateTransactionsGenerator() TransactionGenerator {
	return nil
}

// type TransactionMeta struct {
// 	V          int32
// 	Operations *[]OperationMeta
// }
// type OperationMeta struct {
// 	Changes LedgerEntryChanges
// }
// type LedgerEntryChanges []LedgerEntryChange
// type LedgerEntryChange struct {
// 	Type    LedgerEntryChangeType
// 	Created *LedgerEntry
// 	Updated *LedgerEntry
// 	Removed *LedgerKey
// 	State   *LedgerEntry
// }
// type LedgerEntry struct {
// 	LastModifiedLedgerSeq Uint32
// 	Data                  LedgerEntryData
// 	Ext                   LedgerEntryExt
// }
// type LedgerEntryData struct {
// 	Type      LedgerEntryType
// 	Account   *AccountEntry
// 	TrustLine *TrustLineEntry
// 	Offer     *OfferEntry
// 	Data      *DataEntry
// }

func (sampler *TransactionsSampler) Generate(size Size) (build.TransactionBuilder, PublicToSeedMapper) { // , xdr.TransactionMeta) {
	sourceAccount := getRandomAccount(sampler.database)
	transaction := build.Transaction(
		build.SourceAccount{sourceAccount.Keypair.Address()},
		build.Sequence{sourceAccount.SeqNum + 1},
		build.TestNetwork,
	)
	generator := sampler.generators(sourceAccount)
	for it := Size(0); it < size; it++ {
		operation := generator(1, sampler.database)
		transaction.Mutate(operation)
	}
	return transaction, sourceAccount
}

func (sampler *TransactionsSampler) ApplyChanges(changes *xdr.TransactionMeta) *TransactionsSampler {
	applyTransactionChanges(changes, sampler.database, sampler.database)
	return sampler
}

func applyTransactionChanges(changes xdr.TransactionMeta, database Database, mapper PublicToSeedMapper) Database {
	for _, operation := range changes.Operations {
		for _, change := range operation.Changes {
			switch change.Type {
			case xdr.LedgerEntryChangeTypeLedgerEntryCreated:
				database = handleEntryCreated(change.Created.Data, database)
			case xdr.LedgerEntryChangeTypeLedgerEntryUpdated:
				database = handleEntryUpdated(change.Updated.Data, database)
			case xdr.LedgerEntryChangeTypeLedgerEntryRemoved:
			case xdr.LedgerEntryChangeTypeLedgerEntryState:
			}
		}
	}
	return database
}

func handleEntryCreated(data LedgerEntryData, database Database, mapper PublicToSeedMapper) Database {
	switch data.Type {
	case xdr.LedgerEntryTypeAccount:
		accountEntry := &AccountEntry{}
		accountEntry.SetAccountEntry(data.Account)
		accountEntry.Keypair = mapper(accountEntry.AccountEntry.AccountId.Ed25519)
		database = database.AddAccount(accountEntry)
	case xdr.LedgerEntryTypeTrustline:
		// TODO
	}
	return database
}

func fromRawSeed(seed [32]byte) *keypair.Full {
	return keypair.FromRawSeed(seed)
}

func handleEntryUpdated(data LedgerEntryData, database Database, mapper PublicToSeedMapper) Database {
	switch data.Type {
	case xdr.LedgerEntryTypeAccount:
		seed := mapper(accountEntry.AccountEntry.AccountId.Ed25519)
		oldAccount := database.GetAccountByAddress(seed)
		oldAccount.SetAccountEntry(data.Account)
	case xdr.LedgerEntryTypeTrustline:
		// TODO
	}
	return database
}

func getRandomGeneratorWrapper(generatorsList generatorsList) func(*AccountEntry) MutatorGenerator {
	return func(sourceAccount *AccountEntry) MutatorGenerator {
		return getRandomGenerator(generatorsList, sourceAccount)
	}
}

func getRandomGenerator(generators generatorsList, sourceAccount *AccountEntry) MutatorGenerator {
	var result MutatorGenerator
	whole := 100
	var randomCdf uint32 = uint32(rand.Intn(whole) + 1)
	var cdf uint32 = 0
	for _, value := range generators {
		cdf += value.bias
		if cdf >= randomCdf {
			result = value.generator(sourceAccount)
			break
		}
	}
	return result
}

func getValidCreateAccountMutator(sourceAccount *AccountEntry) MutatorGenerator {
	return func(size Size, database Database) build.TransactionMutator {
		destinationKeypair := getNextKeypair() // generateRandomKeypair()
		destination := build.Destination{destinationKeypair.GetSeed().Address()}
		startingBalance := rand.Int63n(int64(sourceAccount.Balance)) + 1
		amount := build.NativeAmount{strconv.FormatInt(startingBalance, 10)}

		// TODO forget about validation?
		// rawSeed := fullKeypairToRawBytes(destinationKeypair)
		// destAccountId := xdr.AccountId(xdr.PublicKey{Type: xdr.PublicKeyTypePublicKeyTypeEd25519, Ed25519: rawSeed})
		// var createData xdr.LedgerEntryData
		// createdData.Type = xdr.LedgerEntryTypeAccount
		// createdData.Account = &xdr.AccountEntry{AccountId: destAccountId, Balance: startingBalance}
		// var createdChange xdr.LedgerEntryChange
		// createdChange.Type = xdr.LedgerEntryChangeTypeLedgerEntryCreated
		// createdChange.Created = xdr.LedgerEntry{Data: createdData}

		// sourceAccount.Balance -= startingBalance + fee
		// var updatedData xdr.LedgerEntryData
		// updatedData.Type = xdr.LedgerEntryTypeAccount
		// updatedData.Account = sourceAccount.AccountEntry
		// var updatedChange xdr.LedgerEntryChange
		// updatedChange.Type = xdr.LedgerEntryChangeTypeLedgerEntryUpdated
		// updatedChange.Updated = xdr.LedgerEntry{Data: updatedData}
		// changes := xdr.LedgerEntryChanges([]LedgerEntryChange{createdChange, updatedChange})
		// opMeta := xdr.OperationMeta{Changes: changes}

		// TODO add stub to database
		database.AddAccount(&AccountEntry{Keypair: destinationKeypair})

		return build.CreateAccount(destination, amount) // , opMeta
	}
}

var privateKeyIndex uint64 = 0

func getNextKeypair() SeedProvider {
	privateKeyIndex++
	return privateKeyIndex
}

func generateRandomKeypair() *keypair.Full {
	keypair, _ := keypair.Random()
	return keypair
}

func fullKeypairToRawBytes(full *keypair.Full) [32]byte {
	return strkey.MustDecode(strkey.VersionByteSeed, full.Seed())
}

func rawKeyToString(key [32]byte) string {
	return strkey.MustEncode(strkey.VersionByteSeed, key)
}

func getValidPaymentMutator(sourceAccount *AccountEntry) MutatorGenerator {
	return func(size Size, database Database) build.TransactionMutator {
		destinationAccount := getRandomAccount(database)
		trustLine, destTrustLine := getRandomTrustLine(sourceAccount, destinationAccount, database)
		amount := rand.Int63n(min(int64(trustLine.Balance), int64(destTrustLine.Limit)))
		amountString := strconv.FormatInt(amount, 10)
		var paymentMut build.PaymentMutator
		if trustLine.Asset.Native {
			paymentMut = build.NativeAmount{amountString}
		} else {
			paymentMut = build.CreditAmount{Code: trustLine.Asset.Code, Issuer: trustLine.Asset.Issuer, Amount: amountString}
		}
		return build.Payment(build.Destination{destinationAccount.AccountId.Address()}, paymentMut)
	}
}

func min(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}

// TODO: now it returns only native assets
func getRandomTrustLine(sourceAccount, destinationAccount *AccountEntry, database Database) (*TrustLineEntry, *TrustLineEntry) {
	myTrustLine := &TrustLineEntry{
		AccountId: sourceAccount.AccountId,
		Asset:     build.NativeAsset(),
		Balance:   sourceAccount.Balance,
		Limit:     math.MaxInt64,
	}
	destinationTrustline := &TrustLineEntry{
		AccountId: destinationAccount.AccountId,
		Asset:     build.NativeAsset(),
		Balance:   destinationAccount.Balance,
		Limit:     math.MaxInt64,
	}
	return &myTrustLine, &destinationTrustline
}

func getRandomAccount(database Database) *AccountEntry {
	return database.GetAccountByOrder(rand.Intn(database.GetAccountsCount()))
}
