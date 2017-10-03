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
	"sort"
)

type AccountEntry struct {
	xdr.AccountEntry
	SequenceManager
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

type SequenceManager interface {
	GetSequence() (build.Sequence, error)
	SetSequence(build.Sequence)
}

func (value *Uint64) GetSequence() (build.Sequence, error) {
	return build.Sequence{uint64(*value)}, nil
}

func (value *Uint64) SetSequence(newValue build.Sequence) {
	*value = Uint64(newValue.Sequence)
}

type SequenceInitilizer struct {
	account     *AccountEntry
	seqProvider SequenceNumberFetcher
}

func (value *SequenceInitilizer) GetSequence() (build.Sequence, error) {
	Logger.Print("fetching sequence number")
	var result build.Sequence
	seq, error := value.seqProvider.FetchSequenceNumber(value.account.Keypair.GetSeed())
	if error != nil {
		Logger.Printf("error while fetching a sequence number: %v", error)
		return result, nil
	}
	provider := Uint64(seq.Sequence)
	Logger.Printf("sequence number fetched, changing provider")
	value.account.SequenceManager = &provider
	return seq, nil
}

func (value *SequenceInitilizer) SetSequence(seq build.Sequence) {
	newValue := Uint64(seq.Sequence)
	value.account.SequenceManager = &newValue
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
	sequenceProvider  SequenceNumberFetcher
	// backup            []tuple
	// added             []*AccountEntry
	// mappedTrustlines  map[string]*TrustLineEntry
}

func NewInMemoryDatabase(sequenceFetcher SequenceNumberFetcher) Database {
	dataMap := make(map[string]*AccountEntry)
	return &InMemoryDatabase{orderedData: circularBuffer.NewCircularBuffer(1000), mappedData: dataMap, sequenceProvider: sequenceFetcher}
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
	account.SequenceManager = &SequenceInitilizer{account: account, seqProvider: data.sequenceProvider}
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
	paymentGenerator := generatorsListEntry{generator: getValidPaymentMutatorNative, bias: 50}
	generatorsList := []generatorsListEntry{accountGenerator, paymentGenerator}
	sampler := &TransactionsSampler{generators: getRandomGeneratorWrapper(generatorsList)}
	return func(size uint64, database Database) (*build.TransactionBuilder, *AccountEntry) {
		return sampler.Generate(size, database)
	}
}

const (
	baseFee        int64 = 100
	baseReserve    int64 = 10 * amount.One
	minimalBalance int64 = 2 * baseReserve
)

func (sampler *TransactionsSampler) Generate(_ uint64, database Database) (*build.TransactionBuilder, *AccountEntry) {
	const minimalOperation = baseFee + minimalBalance

	sourceAccount := getRandomAccount(database)
	sourceBalance := int64(sourceAccount.Balance)
	availableAmount := sourceBalance - minimalBalance
	if availableAmount <= 0 {
		Logger.Print("account's balance lower than minimal balance")
		return nil, sourceAccount
	}
	if availableAmount < minimalOperation {
		Logger.Print("account's balance is too small")
		return nil, sourceAccount
	}
	availableAmount -= minimalOperation
	availableAmount = rand.Int63n(availableAmount) + minimalOperation
	Logger.Printf("going to spend %d out of %d", availableAmount, sourceBalance)
	maximalNumberOfOperations := availableAmount / minimalOperation
	maximalNumberOfOperations = min(maximalNumberOfOperations, 100)
	size := rand.Intn(int(maximalNumberOfOperations)) + 1
	availableAmount -= int64(size) * minimalOperation
	balancePartition := GetRandomPartitionWithoutZeros(availableAmount, size)
	Logger.Printf("balance's partition: %v", balancePartition)

	seq, seqError := sourceAccount.GetSequence()
	if seqError != nil {
		Logger.Print("error while getting account's sequence number")
		return nil, sourceAccount
	}
	seq.Sequence++
	sourceAccount.SetSequence(seq)
	operations := []build.TransactionMutator{
		build.SourceAccount{sourceAccount.Keypair.GetSeed().Address()},
		seq,
		build.TestNetwork,
	}
	for _, value := range balancePartition {
		generator := sampler.generators(sourceAccount)
		mutator := generator(uint64(value+minimalBalance), database)
		if mutator == nil {
			Logger.Printf("sampled a nil transaction mutator")
			continue
		}
		operations = append(operations, mutator)
		sourceAccount.Balance -= xdr.Int64(baseFee)
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
	return func(startingBalance uint64, database Database) build.TransactionMutator {
		destinationKeypair := getNextKeypair() // generateRandomKeypair()
		destination := build.Destination{destinationKeypair.GetSeed().Address()}
		amount := build.NativeAmount{amount.String(xdr.Int64(startingBalance))}

		newAccount := &AccountEntry{Keypair: destinationKeypair}
		newAccount.Balance = xdr.Int64(startingBalance)
		database.AddAccount(newAccount)
		result := build.CreateAccount(destination, amount)
		Logger.Printf("created CreateAccount tx: %+v", result)

		sourceAccount.Balance -= xdr.Int64(int64(startingBalance))
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

func getValidPaymentMutatorNative(sourceAccount *AccountEntry) MutatorGenerator {
	return func(payment uint64, database Database) build.TransactionMutator {
		Logger.Printf("sampling a valid payment")
		destinationAccount := getRandomAccount(database)
		amountString := amount.String(xdr.Int64(payment))
		result := build.Payment(build.Destination{destinationAccount.Keypair.GetSeed().Address()}, build.NativeAmount{amountString})
		destinationAccount.Balance += xdr.Int64(payment)
		sourceAccount.Balance -= xdr.Int64(payment)
		Logger.Printf("created Payment tx: %+v", result)
		return result
	}
}

func getValidPaymentMutatorFromTrustline(sourceAccount *AccountEntry) MutatorGenerator {
	return func(payment uint64, database Database) build.TransactionMutator {
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
		Logger.Printf("created Payment tx: %+v", result)
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

func GetRandomPartitionWithZeros(sum int64, size int) []int64 {
	diffFunc := func(sum int64, size int) Ints64 {
		var differences Ints64 = make(Ints64, size)
		for ix := 0; ix < size-1; ix++ {
			differences[ix] = rand.Int63n(sum + 1)
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

func GetUniformMofN(maxValue int64, size int) Ints64 {
	var result Ints64 = make(Ints64, 0, size)
	selected := map[int64]bool{}
	for ix := int64(maxValue - int64(size) + 1); ix <= maxValue; ix++ {
		selectedValue := rand.Int63n(ix) + 1
		if selected[selectedValue] {
			selectedValue = ix
		}
		selected[selectedValue] = true
		result = append(result, selectedValue)
	}
	return result
}

func GetUniformMofNFromSlice(data []int, size int) []int {
	var result []int = make([]int, 0, size)
	selected := map[int]bool{}
	for ix := len(data) - size; ix < len(data); ix++ {
		selectedIndex := rand.Intn(ix)
		if selected[selectedIndex] {
			selectedIndex = ix
		}
		selected[selectedIndex] = true
		result = append(result, data[selectedIndex])
	}
	return result
}
