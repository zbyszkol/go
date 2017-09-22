package sampler

import (
	"encoding/binary"
	"github.com/stellar/go/amount"
	"github.com/stellar/go/build"
	"github.com/stellar/go/keypair"
	"github.com/stellar/go/strkey"
	"github.com/stellar/go/utils/circularBuffer"
	"github.com/stellar/go/xdr"
	"math"
	"math/rand"
)

const logTag string = "[sampler.go] "

type AccountEntry struct {
	xdr.AccountEntry
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

type SeedProvider interface {
	GetSeed() *Full
}

type Uint64 uint64

func (seed Uint64) GetSeed() *Full {
	var bytesData [32]byte
	binary.LittleEndian.PutUint64(bytesData[:], uint64(seed))
	result := Full(*fromRawSeed(bytesData))
	return &result
}

type Full struct{ keypair.Full }

func (full *Full) GetSeed() *Full {
	return full
}

type Database interface {
	BeginTransaction()
	RejectTransaction()
	EndTransaction()
	GetAccountByOrder(int) *AccountEntry
	GetAccountByAddress(string) *AccountEntry
	AddAccount(*AccountEntry) Database
	GetAccountsCount() int
	GetTrustLineByOrder(int) *TrustLineEntry
	// GetTrustLineById(accountAddress keypair.KP, issuer keypair.KP, assetcode string) *TrustLineEntry
	GetTrustLineCount() int
	AddTrustLine(*TrustLineEntry)
}

func AddRootAccount(database Database, accountFetcher AccountFetcher, sequenceProvider SequenceNumberFetcher) (Database, error) {
	rootSeed := "SDHOAMBNLGCE2MV5ZKIVZAQD3VCLGP53P3OBSBI6UN5L5XZI5TKHFQL4"
	fullKP := fromRawSeed(seedStringToBytes(rootSeed))
	coreAccount, fetchError := accountFetcher.FetchAccount(fullKP)
	if fetchError != nil {
		return database, fetchError
		// errors.New("error while fetching the sequence number for the root account: ")
	}
	rootSequenceNumber, seqError := sequenceProvider.FetchSequenceNumber(fullKP)
	if seqError != nil {
		return database, seqError
	}
	rootAccount := &AccountEntry{Keypair: fullKP}
	rootAccount.Balance = coreAccount.Balance
	rootAccount.SeqNum = xdr.SequenceNumber(rootSequenceNumber.Sequence)
	return database.AddAccount(rootAccount), nil
}

func seedStringToBytes(seed string) [32]byte {
	return sliceToFixedArray(strkey.MustDecode(strkey.VersionByteSeed, seed))
}

func sliceToFixedArray(data []byte) [32]byte {
	var resultArray [32]byte
	copy(resultArray[:], data)
	return resultArray
}

type tuple struct {
	old AccountEntry
	new *AccountEntry
}

type InMemoryDatabase struct {
	orderedData       *circularBuffer.CircularBuffer
	mappedData        map[string]*AccountEntry
	orderedTrustlines *circularBuffer.CircularBuffer
	// backup            []tuple
	// added             []*AccountEntry
	// mappedTrustlines  map[string]*TrustLineEntry
}

func NewInMemoryDatabase() Database {
	dataMap := make(map[string]*AccountEntry)
	return &InMemoryDatabase{orderedData: circularBuffer.NewCircularBuffer(1000), mappedData: dataMap}
}

func (data *InMemoryDatabase) BeginTransaction() {
	// data.backup = []AccountEntry{}
}

func (data *InMemoryDatabase) EndTransaction() {
	// data.backup = []AccountEntry{}
}

func (data *InMemoryDatabase) RejectTransaction() {
	// for _, tuple := range data.backup {
	// 	*tuple.new = tuple.old
	// }
	// if len(data.added) >= len(data.orderedData) {
	// 	data.added = []*AccountEntry{}
	// 	// TODO clear rest
	// } else {
	// 	for _, added := range data.added {
	// 		delete(data.mappedData, added.Keypair.GetSeed().Address())
	// 	}
	// }
}

func (data *InMemoryDatabase) GetAccountByOrder(order int) *AccountEntry {
	value := data.orderedData.Get(order).(*AccountEntry)
	// data.backup = append(data.backup, tuple{old: *value, new: value})
	return value
}

func (data *InMemoryDatabase) GetAccountsCount() int {
	return data.orderedData.Count()
}

func (data *InMemoryDatabase) AddAccount(account *AccountEntry) Database {
	_, removed, wasRemoved := data.orderedData.Add(account)
	data.mappedData[account.Keypair.GetSeed().Address()] = account
	if wasRemoved {
		removedAccount := removed.(*AccountEntry)
		delete(data.mappedData, removedAccount.Keypair.GetSeed().Address())
	}
	// TODO add data to remove account
	return data
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

func (data *InMemoryDatabase) GetAccountByAddress(address string) *AccountEntry {
	return data.mappedData[address]
}

type TransactionGenerator func(uint64, Database) (*build.TransactionBuilder, *AccountEntry)

type MutatorGenerator func(uint64, Database) build.TransactionMutator

type generatorsListEntry struct {
	generator func(*AccountEntry) MutatorGenerator
	bias      uint32
}

type generatorsList []generatorsListEntry

type TransactionsSampler struct {
	generators func(*AccountEntry) MutatorGenerator
}

func NewTransactionGenerator() TransactionGenerator {
	accountGenerator := generatorsListEntry{generator: getValidCreateAccountMutator, bias: 50}
	paymentGenerator := generatorsListEntry{generator: getValidPaymentMutator, bias: 50}
	generatorsList := []generatorsListEntry{accountGenerator, paymentGenerator}
	sampler := &TransactionsSampler{generators: getRandomGeneratorWrapper(generatorsList)}
	return func(size uint64, database Database) (*build.TransactionBuilder, *AccountEntry) {
		return sampler.Generate(size, database)
	}
}

const (
	minimalBalance uint64 = 20
	minimalFee     uint64 = 100
)

func (sampler *TransactionsSampler) Generate(size uint64, database Database) (*build.TransactionBuilder, *AccountEntry) {
	sourceAccount := getRandomAccount(database)
	sourceBalance := uint64(sourceAccount.Balance)
	accountsCount := database.GetAccountsCount()
	count := 0
	for ; sourceBalance <= minimalBalance+size*minimalFee && count < accountsCount; count++ {
		Logger.Printf(logTag + "sampling an account")
		sourceAccount = database.GetAccountByOrder(count)
		sourceBalance = uint64(sourceAccount.Balance)
		Logger.Printf(logTag+"sampled account: %+v", sourceAccount)
	}
	if count >= accountsCount {
		// no account is able to fulfill constraints
		Logger.Printf("no account able to fulfill fee constraints, returning nil")
		return nil, nil
	}
	sourceAccount.Balance = xdr.Int64(sourceBalance - size*minimalFee)

	operations := []build.TransactionMutator{
		build.SourceAccount{sourceAccount.Keypair.GetSeed().Address()},
		build.Sequence{uint64(sourceAccount.SeqNum + 1)},
		build.TestNetwork,
	}
	for it := uint64(0); it < size; it++ {
		generator := sampler.generators(sourceAccount)
		mutator := generator(1, database)
		if mutator == nil {
			Logger.Printf("sampled a nil transaction mutator")
			continue
		}
		operations = append(operations, mutator)
	}

	transaction := build.Transaction(
		operations...,
	)
	return transaction, sourceAccount
}

func ApplyChanges(changes *xdr.TransactionMeta, database Database) Database {
	for _, operation := range *changes.Operations {
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

func handleEntryCreated(data xdr.LedgerEntryData, database Database) Database {
	switch data.Type {
	case xdr.LedgerEntryTypeAccount:
		publicKey := RawKeyToString(*data.Account.AccountId.Ed25519)
		accountEntry := database.GetAccountByAddress(publicKey)
		accountEntry.SetAccountEntry(data.Account)
	case xdr.LedgerEntryTypeTrustline:
		// TODO
	}
	return database
}

func fromRawSeed(seed [32]byte) *Full {
	result, _ := keypair.FromRawSeed(seed)
	return &Full{*result}
}

func handleEntryUpdated(data xdr.LedgerEntryData, database Database) Database {
	switch data.Type {
	case xdr.LedgerEntryTypeAccount:
		publicKey := RawKeyToString(*data.Account.AccountId.Ed25519)
		accountEntry := database.GetAccountByAddress(publicKey)
		accountEntry.SetAccountEntry(data.Account)
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
	return func(size uint64, database Database) build.TransactionMutator {
		destinationKeypair := getNextKeypair() // generateRandomKeypair()
		destination := build.Destination{destinationKeypair.GetSeed().Address()}
		startingBalance := rand.Int63n(int64(sourceAccount.Balance))
		amount := build.NativeAmount{amount.String(xdr.Int64(startingBalance))}

		newAccount := &AccountEntry{Keypair: destinationKeypair}
		newAccount.Balance = xdr.Int64(startingBalance)
		database.AddAccount(newAccount)
		sourceAccount.Balance -= xdr.Int64(startingBalance)
		result := build.CreateAccount(destination, amount)
		Logger.Printf(logTag+"created CreateAccount tx: %+v", result)
		return &result
	}
}

var privateKeyIndex uint64 = 0

func getNextKeypair() SeedProvider {
	privateKeyIndex++
	return Uint64(privateKeyIndex)
}

func generateRandomKeypair() *keypair.Full {
	keypair, _ := keypair.Random()
	return keypair
}

func fullKeypairToRawBytes(full *keypair.Full) [32]byte {
	return sliceToFixedArray(strkey.MustDecode(strkey.VersionByteSeed, full.Seed()))
}

func RawKeyToString(key [32]byte) string {
	return strkey.MustEncode(strkey.VersionByteSeed, key[:])
}

func bytesToString(data []byte) string {
	return strkey.MustEncode(strkey.VersionByteSeed, data)
}

func getValidPaymentMutator(sourceAccount *AccountEntry) MutatorGenerator {
	return func(size uint64, database Database) build.TransactionMutator {
		Logger.Printf("sampling a valid payment")
		destinationAccount := getRandomAccount(database)
		// for destinationAccount.Keypair.GetSeed().Address() == sourceAccount.Keypair.GetSeed().Address() {
		// 	destinationAccount = getRandomAccount(database)
		// }
		trustLine, destTrustLine := getRandomTrustLine(sourceAccount, destinationAccount, database)
		availableAmount := min(int64(trustLine.Balance), int64(destTrustLine.Limit))
		Logger.Printf(logTag+"amount of assets available for tx: %d", availableAmount)
		payment := rand.Int63n(availableAmount)
		amountString := amount.String(xdr.Int64(payment))
		var paymentMut build.PaymentMutator
		if trustLine.Asset.Type == xdr.AssetTypeAssetTypeNative {
			paymentMut = build.NativeAmount{amountString}

			destinationAccount.Balance += xdr.Int64(payment)
			sourceAccount.Balance -= xdr.Int64(payment)
		} else {
			asset := trustLine.Asset
			var code, issuer string
			if asset.Type == xdr.AssetTypeAssetTypeCreditAlphanum4 {
				code = bytesToString(asset.AlphaNum4.AssetCode[:])
				issuer = RawKeyToString(*asset.AlphaNum4.Issuer.Ed25519)
			} else if asset.Type == xdr.AssetTypeAssetTypeCreditAlphanum12 {
				code = bytesToString(asset.AlphaNum4.AssetCode[:])
				issuer = RawKeyToString(*asset.AlphaNum4.Issuer.Ed25519)
			}
			paymentMut = build.CreditAmount{Code: code, Issuer: issuer, Amount: amountString}

			destTrustLine.Balance += xdr.Int64(payment)
			trustLine.Balance -= xdr.Int64(payment)
		}
		result := build.Payment(build.Destination{destinationAccount.Keypair.GetSeed().Address()}, paymentMut)
		Logger.Printf(logTag+"created Payment tx: %+v", result)
		return result
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
	myTrustLine := &TrustLineEntry{}
	myTrustLine.AccountId = sourceAccount.AccountId
	myTrustLine.Asset.Type = xdr.AssetTypeAssetTypeNative
	myTrustLine.Balance = sourceAccount.Balance
	myTrustLine.Limit = math.MaxInt64

	destinationTrustline := &TrustLineEntry{}
	destinationTrustline.AccountId = destinationAccount.AccountId
	destinationTrustline.Asset.Type = xdr.AssetTypeAssetTypeNative
	destinationTrustline.Balance = destinationAccount.Balance
	destinationTrustline.Limit = math.MaxInt64
	return myTrustLine, destinationTrustline
}

func getRandomAccount(database Database) *AccountEntry {
	return database.GetAccountByOrder(rand.Intn(database.GetAccountsCount()))
}
