package sampler

import (
	"encoding/binary"
	"github.com/stellar/go/amount"
	"github.com/stellar/go/build"
	"github.com/stellar/go/keypair"
	"github.com/stellar/go/strkey"
	"github.com/stellar/go/xdr"
	"math"
	"math/rand"
	"sort"
)

type AccountEntry struct {
	xdr.AccountEntry
	SequenceManager
	Keypair SeedProvider
}

type TrustLineEntry struct {
	xdr.TrustLineEntry
	Keypair SeedProvider
}

type Full struct{ keypair.Full }

type Uint64 uint64

type SequenceInitilizer struct {
	account     *AccountEntry
	seqProvider SequenceNumberFetcher
}

type SeedProvider interface {
	GetSeed() *Full
}

type SequenceManager interface {
	GetSequence() (build.Sequence, error)
	SetSequence(build.Sequence)
}

func (entry *AccountEntry) SetAccountEntry(xdrEntry *xdr.AccountEntry) *AccountEntry {
	entry.AccountEntry = *xdrEntry
	return entry
}

func (entry *AccountEntry) GetAccountEntry() *xdr.AccountEntry {
	return &entry.AccountEntry
}

func (value *SequenceInitilizer) GetSequence() (build.Sequence, error) {
	Logger.Print("fetching sequence number for account %s", value.account.Keypair.GetSeed())
	var result build.Sequence
	var error error
	result, error = value.seqProvider.FetchSequenceNumber(value.account.Keypair.GetSeed())
	if error != nil {
		Logger.ErrorPrintf("error while fetching a sequence number for account %s: %s", value.account.Keypair.GetSeed(), error)
		return result, error
	}
	if result.Sequence != 0 {
		provider := Uint64(result.Sequence)
		Logger.Printf("sequence number fetched (%+v), changing provider", provider)
		value.account.SequenceManager = &provider
	}
	return result, nil
}

func (value *SequenceInitilizer) SetSequence(seq build.Sequence) {
	newValue := Uint64(seq.Sequence)
	value.account.SequenceManager = &newValue
}

func (value *Uint64) GetSequence() (build.Sequence, error) {
	return build.Sequence{uint64(*value)}, nil
}

func (value *Uint64) SetSequence(newValue build.Sequence) {
	*value = Uint64(newValue.Sequence)
}

func (seed Uint64) GetSeed() *Full {
	var bytesData [32]byte
	binary.LittleEndian.PutUint64(bytesData[:], uint64(seed))
	result := Full(*fromRawSeed(bytesData))
	return &result
}

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

var rootAccount AccountEntry = AccountEntry{}

func AddRootAccount(database Database, accountFetcher AccountFetcher, sequenceProvider SequenceNumberFetcher) (Database, error) {
	database.BeginTransaction()
	defer database.EndTransaction()
	rootSeed := "SDHOAMBNLGCE2MV5ZKIVZAQD3VCLGP53P3OBSBI6UN5L5XZI5TKHFQL4" // "(V) (;,,;) (V)"
	fullKP := fromRawSeed(seedStringToBytes(rootSeed))
	coreAccount, fetchError := accountFetcher.FetchAccount(fullKP)
	if fetchError != nil {
		database.RejectTransaction()
		return database, fetchError
	}
	rootAccount.Keypair = fullKP
	rootAccount.Balance = coreAccount.Balance
	seqManager := Uint64(0)
	rootAccount.SequenceManager = &seqManager
	database = database.AddAccount(&rootAccount)

	return database, nil
}

func seedStringToBytes(seed string) [32]byte {
	return sliceToFixedArray(strkey.MustDecode(strkey.VersionByteSeed, seed))
}

func sliceToFixedArray(data []byte) [32]byte {
	var resultArray [32]byte
	copy(resultArray[:], data)
	return resultArray
}

type InMemoryDatabase struct {
	orderedData       []*AccountEntry
	mappedData        map[string]*AccountEntry
	orderedTrustlines []*TrustLineEntry
	sequenceProvider  SequenceNumberFetcher
	accountCounter    int
}

func NewInMemoryDatabase(sequenceFetcher SequenceNumberFetcher) Database {
	dataMap := make(map[string]*AccountEntry)
	return &InMemoryDatabase{orderedData: make([]*AccountEntry, 0, 1000000), mappedData: dataMap, sequenceProvider: sequenceFetcher, accountCounter: 0}
}

func (data *InMemoryDatabase) BeginTransaction() {
}

func (data *InMemoryDatabase) EndTransaction() {
}

func (data *InMemoryDatabase) RejectTransaction() {
}

func (data *InMemoryDatabase) GetAccountByOrder(order int) *AccountEntry {
	value := data.orderedData[order]
	return value
}

func (data *InMemoryDatabase) GetAccountsCount() int {
	return len(data.orderedData)
}

var addAccountRand = rand.New(rand.NewSource(0))

// reservoir sampling of accounts
func (data *InMemoryDatabase) AddAccount(account *AccountEntry) Database {
	if account.SequenceManager == nil {
		account.SequenceManager = &SequenceInitilizer{account: account, seqProvider: data.sequenceProvider}
	}
	data.accountCounter++
	var include int
	if data.accountCounter < cap(data.orderedData) {
		data.orderedData = append(data.orderedData, nil)
		include = len(data.orderedData) - 1
	} else {
		include = addAccountRand.Intn(data.accountCounter + 1)
	}
	if include < len(data.orderedData) {
		prevValue := data.orderedData[include]
		if prevValue != nil {
			delete(data.mappedData, prevValue.Keypair.GetSeed().Address())
		}
		data.orderedData[include] = account
		data.mappedData[account.Keypair.GetSeed().Address()] = account
	}

	return data
}

func (data *InMemoryDatabase) GetTrustLineByOrder(ix int) *TrustLineEntry {
	return data.orderedTrustlines[ix]
}

func (data *InMemoryDatabase) GetTrustLineCount() int {
	return len(data.orderedTrustlines)
}

func (data *InMemoryDatabase) AddTrustLine(trustline *TrustLineEntry) {
	data.orderedTrustlines = append(data.orderedTrustlines, trustline)
}

func (data *InMemoryDatabase) GetAccountByAddress(address string) *AccountEntry {
	return data.mappedData[address]
}

type CommitResult func(Database) Database

type RejectResult func(Database) Database

type TransactionGenerator func(uint64, Database) (*build.TransactionBuilder, *AccountEntry, CommitResult, RejectResult)

type MutatorGenerator func(uint64, Database) (build.TransactionMutator, CommitResult, RejectResult)

type GeneratorsListEntry struct {
	Generator func(*AccountEntry) MutatorGenerator
	Bias      uint
}

type generatorsList []GeneratorsListEntry

type TransactionsSampler struct {
	generators func(*AccountEntry) MutatorGenerator
}

func NewTransactionGenerator(generators ...GeneratorsListEntry) TransactionGenerator {
	sampler := &TransactionsSampler{generators: getRandomGeneratorWrapper(generators)}
	return func(size uint64, database Database) (*build.TransactionBuilder, *AccountEntry, CommitResult, RejectResult) {
		return sampler.Generate(size, database)
	}
}

func DefaultTransactionGenerator() TransactionGenerator {
	accountGenerator := GeneratorsListEntry{Generator: GetValidCreateAccountMutator, Bias: 50}
	paymentGenerator := GeneratorsListEntry{Generator: GetValidPaymentMutatorNative, Bias: 50}
	return NewTransactionGenerator(accountGenerator, paymentGenerator)
}

func isRoot(account *AccountEntry) bool {
	return account == &rootAccount
}

func getRandomAccountWithNonZeroSequence(database Database) *AccountEntry {
	for {
		account := getRandomAccount(database)
		seq, error := account.GetSequence()
		if error == nil && (seq.Sequence > 0 || isRoot(account)) {
			Logger.Printf("found an account with non-zero sequence: %s", account)
			return account
		}
	}
	return nil
}

const (
	baseFee          int64 = 100
	baseReserve      int64 = 10 * amount.One
	minimalBalance   int64 = 2 * baseReserve
	minimalOperation       = baseFee + minimalBalance
)

var txRand = rand.New(rand.NewSource(0))
var opRand = rand.New(rand.NewSource(0))

func (sampler *TransactionsSampler) Generate(_ uint64, database Database) (*build.TransactionBuilder, *AccountEntry, CommitResult, RejectResult) {
	var transaction *build.TransactionBuilder
	var sourceAccount *AccountEntry
	var commitOperations []CommitResult
	var rejectOperations []RejectResult
	sourceAccount = getRandomAccountWithNonZeroSequence(database)
	Logger.Printf("using following source account: %s", sourceAccount.Keypair.GetSeed())
	sourceBalance := int64(sourceAccount.Balance)
	availableAmount := sourceBalance - minimalBalance
	if availableAmount <= 0 {
		Logger.Print("account's balance lower than minimal balance")
		return nil, sourceAccount, func(d Database) Database { return d }, func(d Database) Database { return d }
	}
	if availableAmount <= minimalOperation {
		Logger.Printf("account's balance is too small: %d (expected %d)", availableAmount, minimalOperation)
		return nil, sourceAccount, func(d Database) Database { return d }, func(d Database) Database { return d }
	}
	availableAmount = txRand.Int63n(availableAmount-minimalOperation) + minimalOperation
	Logger.Printf("going to spend %d out of %d", availableAmount, sourceBalance)
	maximalNumberOfOperations := availableAmount / minimalOperation
	maximalNumberOfOperations = min(maximalNumberOfOperations, 100)
	size := opRand.Intn(int(maximalNumberOfOperations)) + 1
	availableAmount -= int64(size) * minimalOperation
	balancePartition := GetRandomPartitionWithoutZeros(availableAmount, size)
	Logger.Printf("balance's partition: %v", balancePartition)

	seq, seqError := sourceAccount.GetSequence()
	if seqError != nil {
		Logger.Print("error while getting account's sequence number")
		return nil, sourceAccount, func(d Database) Database { return d }, func(d Database) Database { return d }
	}

	seq.Sequence++
	sourceAccount.SetSequence(seq)

	operations := []build.TransactionMutator{
		build.SourceAccount{sourceAccount.Keypair.GetSeed().Address()},
		seq,
		build.TestNetwork,
	}
	commitOperations = []CommitResult{}
	rejectOperations = []RejectResult{}
	for _, value := range balancePartition {
		generator := sampler.generators(sourceAccount)
		mutator, commitOperation, rejectOperation := generator(uint64(value+minimalBalance), database)
		if mutator == nil {
			Logger.Printf("sampled a nil transaction mutator")
			continue
		}
		operations = append(operations, mutator)
		commitOperations = append(commitOperations, commitOperation)
		rejectOperations = append(rejectOperations, rejectOperation)
		// sourceAccount.Balance -= xdr.Int64(baseFee)
	}
	sourceAccount.Balance -= xdr.Int64(baseFee) * xdr.Int64(len(commitOperations))

	transaction = build.Transaction(
		operations...,
	)

	return transaction, sourceAccount,
		func(database Database) Database {
			for _, operation := range commitOperations {
				database = operation(database)
			}
			return database
		},
		func(database Database) Database {
			seq.Sequence--
			sourceAccount.SetSequence(seq)

			// sourceAccount.Balance += xdr.Int64(baseFee) * xdr.Int64(len(commitOperations))
			return database
		}
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
	Logger.Print("handle entry created")
	switch data.Type {
	case xdr.LedgerEntryTypeAccount:
		publicKey := rawKeyToString(*data.Account.AccountId.Ed25519)
		accountEntry := database.GetAccountByAddress(publicKey)
		Logger.Printf("created entry already in database: %s, %v", publicKey, accountEntry != nil)
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
	Logger.Print("handle entry updated")
	switch data.Type {
	case xdr.LedgerEntryTypeAccount:
		publicKey := rawKeyToString(*data.Account.AccountId.Ed25519)
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

var genRand = rand.New(rand.NewSource(0))

func getRandomGenerator(generators generatorsList, sourceAccount *AccountEntry) MutatorGenerator {
	var result MutatorGenerator
	whole := 100
	var randomCdf uint = uint(genRand.Intn(whole) + 1)
	var cdf uint = 0
	for _, value := range generators {
		cdf += value.Bias
		if cdf >= randomCdf {
			result = value.Generator(sourceAccount)
			break
		}
	}
	return result
}

func GetValidCreateAccountMutator(sourceAccount *AccountEntry) MutatorGenerator {
	return func(startingBalance uint64, database Database) (build.TransactionMutator, CommitResult, RejectResult) {
		destinationKeypair := getNextKeypair() // generateRandomKeypair()
		Logger.Printf("generated account for CreateAccount operation: seed - %s, public - %s", destinationKeypair.GetSeed(), destinationKeypair.GetSeed().Address())
		// TODO set seqnum to ledgernum << 32
		destination := build.Destination{destinationKeypair.GetSeed().Address()}
		amount := build.NativeAmount{amount.String(xdr.Int64(startingBalance))}

		sourceAccount.Balance -= xdr.Int64(int64(startingBalance))

		result := build.CreateAccount(destination, amount)
		Logger.Printf("created CreateAccount tx: %+v", result)
		Logger.Printf("created account's balance is %d", amount)

		return &result,
			func(database Database) Database {
				newAccount := &AccountEntry{Keypair: destinationKeypair}
				newAccount.Balance = xdr.Int64(startingBalance)
				database.AddAccount(newAccount)
				return database
			},
			func(database Database) Database {
				sourceAccount.Balance += xdr.Int64(int64(startingBalance))
				return database
			}
	}
}

var privateKeyIndex uint64 = 0

func getNextKeypair() SeedProvider {
	privateKeyIndex++
	return Uint64(privateKeyIndex)
}

func CreateNewAccount() AccountEntry {
	destinationKeypair := getNextKeypair()
	return AccountEntry{Keypair: destinationKeypair}
}

func generateRandomKeypair() *keypair.Full {
	keypair, _ := keypair.Random()
	return keypair
}

func rawKeyToString(key [32]byte) string {
	return strkey.MustEncode(strkey.VersionByteAccountID, key[:])
}

func bytesToString(data []byte) string {
	return strkey.MustEncode(strkey.VersionByteSeed, data)
}

func GetValidPaymentMutatorNative(sourceAccount *AccountEntry) MutatorGenerator {
	return func(payment uint64, database Database) (build.TransactionMutator, CommitResult, RejectResult) {
		Logger.Printf("sampling a valid payment")
		destinationAccount := getRandomAccountWithNonZeroSequence(database)
		// destinationAccount := getRandomAccount(database)
		amountString := amount.String(xdr.Int64(payment))
		result := build.Payment(build.Destination{destinationAccount.Keypair.GetSeed().Address()}, build.NativeAmount{amountString})

		sourceAccount.Balance -= xdr.Int64(payment)

		Logger.Printf("created Payment tx: %+v", result)
		Logger.Printf("%s created payment for account %s", sourceAccount.Keypair.GetSeed().Address(), destinationAccount.Keypair.GetSeed().Address())
		return result, func(database Database) Database {
				destinationAccount.Balance += xdr.Int64(payment)
				return database
			},
			func(database Database) Database {
				sourceAccount.Balance += xdr.Int64(payment)
				return database
			}
	}
}

func getValidPaymentMutatorFromTrustline(sourceAccount *AccountEntry) MutatorGenerator {
	return func(payment uint64, database Database) (build.TransactionMutator, CommitResult, RejectResult) {
		Logger.Printf("sampling a valid payment")
		destinationAccount := getRandomAccount(database)
		trustLine, destTrustLine := getRandomTrustLine(sourceAccount, destinationAccount, database)
		availableAmount := min(int64(trustLine.Balance), int64(destTrustLine.Limit))
		Logger.Printf("amount of assets available for tx: %d", availableAmount)
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
				issuer = rawKeyToString(*asset.AlphaNum4.Issuer.Ed25519)
			} else if asset.Type == xdr.AssetTypeAssetTypeCreditAlphanum12 {
				code = bytesToString(asset.AlphaNum4.AssetCode[:])
				issuer = rawKeyToString(*asset.AlphaNum4.Issuer.Ed25519)
			}
			paymentMut = build.CreditAmount{Code: code, Issuer: issuer, Amount: amountString}

			destTrustLine.Balance += xdr.Int64(payment)
			trustLine.Balance -= xdr.Int64(payment)
		}
		result := build.Payment(build.Destination{destinationAccount.Keypair.GetSeed().Address()}, paymentMut)
		Logger.Printf("created Payment tx: %+v", result)
		// TODO fix this
		return result, nil, nil
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

var accountRand = rand.New(rand.NewSource(0))

func getRandomAccount(database Database) *AccountEntry {
	numberOfAccounts := database.GetAccountsCount()
	Logger.Printf("getting a random account, available: %d", numberOfAccounts)
	return database.GetAccountByOrder(accountRand.Intn(numberOfAccounts))
}

type Ints64 []int64

func (ints Ints64) Len() int {
	return len(ints)
}

func (ints Ints64) Less(i, j int) bool {
	return ints[i] < ints[j]
}

func (ints Ints64) Swap(i, j int) {
	tmp := ints[i]
	ints[i] = ints[j]
	ints[j] = tmp
}

func getRandomPartition(sum int64, size int, diffFunc func(sum int64, size int) Ints64) Ints64 {
	differences := diffFunc(sum, size)
	Logger.Printf("differences: %v", differences)
	sort.Sort(differences)
	Logger.Printf("sorted differences: %v", differences)
	previous := differences[0]
	for ix := 1; ix < size; ix++ {
		tmp := differences[ix]
		differences[ix] = differences[ix] - previous
		previous = tmp
	}
	return differences
}

var partitionRand = rand.New(rand.NewSource(0))

func GetRandomPartitionWithZeros(sum int64, size int) []int64 {
	diffFunc := func(sum int64, size int) Ints64 {
		var differences Ints64 = make(Ints64, size)
		for ix := 0; ix < size-1; ix++ {
			differences[ix] = partitionRand.Int63n(sum + 1)
		}
		differences[size-1] = sum
		return differences
	}
	return getRandomPartition(sum, size, diffFunc)
}

func GetRandomPartitionWithoutZeros(sum int64, size int) []int64 {
	if sum == 0 {
		return make([]int64, size)
	}
	if size < 1 {
		return []int64{}
	}
	if size == 1 {
		return []int64{sum}
	}
	diffFunc := func(sum int64, size int) Ints64 {
		return append(GetUniformMofN(sum-1, size-1), sum)
	}
	return getRandomPartition(sum, size, diffFunc)
}

var mOfNRand = rand.New(rand.NewSource(0))

func GetUniformMofN(maxValue int64, size int) Ints64 {
	var result Ints64 = make(Ints64, 0, size)
	selected := map[int64]bool{}
	for ix := int64(maxValue - int64(size) + 1); ix <= maxValue; ix++ {
		selectedValue := mOfNRand.Int63n(ix) + 1
		if selected[selectedValue] {
			selectedValue = ix
		}
		selected[selectedValue] = true
		result = append(result, selectedValue)
	}
	return result
}

var mOfNRandSlice = rand.New(rand.NewSource(0))

func GetUniformMofNFromSlice(data []int, size int) []int {
	var result []int = make([]int, 0, size)
	for ix := 0; ix < size; ix++ {
		result[ix] = data[ix]
	}
	for ix := size; ix < len(data); ix++ {
		selectedIx := mOfNRandSlice.Intn(ix + 1)
		if selectedIx < size {
			result[selectedIx] = data[ix]
		}
	}
	return result
}
