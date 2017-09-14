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

type AccountEntry struct {
	xdr.AccountEntry
	Keypair SeedProvider
}

type TrustLineEntry struct {
	xdr.TrustLineEntry
	Keypair keypair.Full
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
	return keypair.FromRawSeed(bytesData)
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
	seedBytes, rawErr := strkey.Decode(strkey.VersionByteSeed, rootSeed)
	if rawErr != nil {
		return database, rawErr
	}
	fullKP, keyErr := keypair.FromRawSeed(seedBytes)
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

type Size uint64

type MutatorGenerator func(Size, Database) (build.TransactionMutator, xdr.OperationMeta)

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

func (sampler *TransactionsSampler) Generate(size Size) (build.TransactionBuilder, xdr.TransactionMeta) {
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
		destinationKeypair := generateRandomKeypair()
		destination := build.Destination{destinationKeypair.Address()}
		startingBalance := rand.Int63n(int64(sourceAccount.Balance)) + 1
		amount := build.NativeAmount{strconv.FormatInt(startingBalance, 10)}

		var createData xdr.LedgerEntryData
		createdData.Type = xdr.LedgerEntryTypeAccount
		createdData.Account = &xdr.AccountEntry{}
		var createdChange xdr.LedgerEntryChange
		createdChange.Type = xdr.LedgerEntryChangeTypeLedgerEntryCreated
		createdChange.Created = xdr.LedgerEntry{Data: createdData}

		var updatedChange xdr.LedgerEntryChange
		changes := xdr.LedgerEntryChanges([]LedgerEntryChange{createdChange, updatedChange})
		opMeta := xdr.OperationMeta{Changes: changes}

		return build.CreateAccount(destination, amount), opMeta
	}
}

func generateRandomKeypair() *keypair.Full {
	keypair, _ := keypair.Random()
	return keypair
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
