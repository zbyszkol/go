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
	GetAccountByOrder(int) AccountEntry
	AddAccount(AccountEntry) Database
	GetAccountsCount() int
	GetTrustLineByOrder(int) TrustLineEntry
	GetTrustLineCount() int
	AddTrustLine(TrustLineEntry)
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
	rootAccount := AccountEntry{Keypair: fullKP, Balance: 1000000000000000000, SeqNum: rootSequenceNumber + 1}
	return database.AddAccount(rootAccount), nil
}

type InMemoryDatabase struct {
	orderedData       []xdr.AccountEntry
	orderedTrustlines []xdr.TrustLineEntry
}

func deleteElement(ix int, data []int) []int {
	for i := 0; i < ix; i++ {
		data[i+1] = data[i]
	}
	return data[1:]
	// var result []int
	// if ix < int(math.Floor(float64(len(data))/float64(2))) {
	// 	// shift right first part
	// 	for i := 0; i < ix; i++ {
	// 		data[i+1] = data[i]
	// 	}
	// 	result = data[1:]
	// } else {
	// 	// shift left second part
	// 	for i := ix + 1; i < len(data); i++ {
	// 		data[i-1] = data[i]
	// 	}
	// 	result = data[:len(data)-1]
	// }
	// return result
}

func (data *InMemoryDatabase) GetAccountByOrder(order int) xdr.AccountEntry {
	return data.orderedData[order]
}

func (data *InMemoryDatabase) GetAccountsCount() int {
	return len(data.orderedData)
}

func (data *InMemoryDatabase) AddAccount(account xdr.AccountEntry) {
	data.orderedData = append(data.orderedData, account)
}

func (data *InMemoryDatabase) GetTrustLineByOrder(ix int) xdr.TrustLineEntry {
	return data.orderedTrustlines[ix]
}

func (data *InMemoryDatabase) GetTrustLineCount() int {
	return len(data.orderedTrustlines)
}

type Size uint64

type TransactionGenerator func(Size) xdr.Transaction

type OperationGenerator func(Size, Database) xdr.Operation

type MutatorGenerator func(Size, Database) build.TransactionMutator

type GeneratorsList []struct {
	Generator func(xdr.AccountEntry) MutatorGenerator
	Bias      uint32
}

type TransactionsSampler struct {
	database   Database
	generators func(AccountEntry) MutatorGenerator
}

func CreateTransactionsGenerator() TransactionGenerator {
	return nil
}

// type Transaction struct {
// 	SourceAccount AccountId
// 	Fee           Uint32
// 	SeqNum        SequenceNumber
// 	TimeBounds    *TimeBounds
// 	Memo          Memo
// 	Operations    []Operation `xdrmaxsize:"100"`
// 	Ext           TransactionExt
// }
func (sampler *TransactionsSampler) Generate(size Size) func() (build.TransactionBuilder, AccountEntry) {
	return func() build.TransactionEnvelopeBuilder {
		sourceAccount := getRandomAccount(sampler.database)
		// const minimalFee = 100
		// fee := getRandomFee(sourceAccount)
		transaction := build.Transaction(
			build.SourceAccount{sourceAccount.Keypair.Address()},
			build.Sequence{sourceAccount.SeqNum},
			build.TestNetwork,
		)
		generator := sampler.generators(sourceAccount)
		for it := Size(0); it < size; it++ {
			operation := generator(1, sampler.database)
			transaction.Mutate(operation)
		}
		return transaction, sourceAccount
	}
}

func getRandomFee(account AccountEntry) uint32 {
	return 100
}

func geometric(p float64) int64 {
	return 4
}

func getRandomGeneratorWrapper(generatorsList GeneratorsList) func(xdr.AccountEntry) MutatorGenerator {
	return func(sourceAccount xdr.AccountEntry) MutatorGenerator {
		return getRandomGenerator(generatorsList, sourceAccount)
	}
}

func getRandomGenerator(generators GeneratorsList, sourceAccount xdr.AccountEntry) MutatorGenerator {
	whole := 100
	var randomCdf uint32 = uint32(rand.Intn(whole) + 1)
	var cdf uint32 = 0
	for _, value := range generators {
		cdf += value.Bias
		if cdf >= randomCdf {
			return value.Generator(sourceAccount)
		}
	}
	return generators[len(generators)-1].Generator(sourceAccount)
}

func getValidCreateAccountMutator(sourceAccount AccountEntry) MutatorGenerator {
	return func(size Size, database Database) build.TransactionMutator {
		destinationKeypair := generateRandomKeypair()
		destination := build.Destination{destinationKeypair.Address()}
		startingBalance := rand.Int63n(int64(sourceAccount.Balance)) + 1
		amount := build.NativeAmount{strconv.FormatInt(startingBalance, 10)}
		return build.CreateAccount(destination, amount)
	}
}

func generateRandomKeypair() *keypair.Full {
	keypair, _ := keypair.Random()
	return keypair
}

func getValidPaymentMutator(sourceAccount AccountEntry) MutatorGenerator {
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

// func getValidPaymentOpOperation(sourceAccount xdr.AccountEntry) OperationGenerator {
// 	return func(size Size, database Database) xdr.Operation {
// 		paymentOp := generateValidPaymentOp(sourceAccount, database)
// 		body := xdr.OperationBody{Type: xdr.OperationTypePayment, PaymentOp: &paymentOp}
// 		return xdr.Operation{SourceAccount: &sourceAccount.AccountId, Body: body}
// 	}
// }

func min(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}

// func generateValidPaymentOp(sourceAccount xdr.AccountEntry, database Database) xdr.PaymentOp {
// 	destinationAccount := getRandomAccount(database)
// 	trustLine, destTrustLine := getRandomTrustLine(sourceAccount, destinationAccount, database)
// 	amount := rand.Int63n(min(int64(trustLine.Balance), int64(destTrustLine.Limit)))
// 	return xdr.PaymentOp{Destination: destinationAccount.AccountId, Asset: trustLine.Asset, Amount: xdr.Int64(amount)}
// }

// TODO: now it returns only native assets
func getRandomTrustLine(sourceAccount, destinationAccount AccountEntry, database Database) (TrustLineEntry, TrustLineEntry) {
	myTrustLine := TrustLineEntry{
		AccountId: sourceAccount.AccountId,
		Asset:     build.NativeAsset(),
		Balance:   sourceAccount.Balance,
		Limit:     math.MaxInt64,
	}
	destinationTrustline := TrustLineEntry{
		AccountId: destinationAccount.AccountId,
		Asset:     build.NativeAsset(),
		Balance:   destinationAccount.Balance,
		Limit:     math.MaxInt64,
	}
	return myTrustLine, destinationTrustline
}

func getRandomAccount(database Database) AccountEntry {
	return database.GetAccountByOrder(rand.Intn(database.GetAccountsCount()))
}

// type ManageOfferOp struct {
// 	Selling Asset
// 	Buying  Asset
// 	Amount  Int64
// 	Price   Price
// 	OfferId Uint64
// }
// type Price struct {
// 	N Int32
// 	D Int32
// }
// func getValidCreateOfferOpOperation(sourceAccount xdr.AccountEntry) OperationGenerator {
// 	return func(size Size, database Database) xdr.Operation {
// 		createOffer := generateValidCreateManagerOfferOp(sourceAccount, database)
// 		body := xdr.OperationBody{Type: xdr.OperationTypeManageOffer, ManageOfferOp: &createOffer}
// 		return xdr.Operation{SourceAccount: &sourceAccount.AccountId, Body: body}
// 	}
// }

// func generateValidCreateManagerOfferOp(sourceAccount xdr.AccountEntry, database Database) xdr.ManageOfferOp {
// 	consumable := rand.Intn(2) == 0
// 	var sellingAsset, buyingAsset xdr.Asset
// 	var maxAmount xdr.Int64
// 	var price xdr.Price

// 	if consumable {
// 		sellingAsset, buyingAsset, maxAmount, price = getRandomConsumableAssets(sourceAccount)
// 	} else {
// 		sellingAsset, buyingAsset, maxAmount, price = getRandomAssets(sourceAccount)
// 	}
// 	amount := rand.Intn(maxAmount) + 1
// 	const NewOfferId xdr.Uint64 = 0
// 	return xdr.ManageOfferOp{Selling: sellingAsset, Buying: buyingAsset, Amount: amount, Price: price, OfferId: NewOfferId}
// }

// type AllowTrustOp struct {
// 	Trustor   AccountId
// 	Asset     AllowTrustOpAsset
// 	Authorize bool
// }
// type ChangeTrustOp struct {
// 	Line  Asset
// 	Limit Int64
// }
// TODO add option for biased data generation
// func getValidChangeTrustOpOperation(sourceAccount xdr.AccountEntry) OperationGenerator {
// 	return func(size Size, database Database) xdr.Operation {
// 		changeTrust := generateValidChangeTrustOp(sourceAccount, database)
// 		body := xdr.OperationBody{Type: xdr.OperationTypeManageOffer, ChangeTrustOp: &changeTrust}
// 		return xdr.Operation{SourceAccount: &sourceAccount.AccountId, Body: body}
// 	}
// }

// func generateValidChangeTrustOp(sourceAccount xdr.AccountEntry, database Database) xdr.ChangeTrustOp {
// 	trustLine := getRandomAsset(database)
// 	limit := xdr.Int64(rand.Int63())
// 	return xdr.ChangeTrustOp{Line: trustLine, Limit: limit}
// }

// func getRandomAsset(database Database) xdr.Asset {

// }

// var parentStageKeypair = indexToKeypair([argv.sourceIndex])
// sourceAccount.pubKey = parentStageKeypair.publicKey();
// sourceAccount.secret = parentStageKeypair.secret();

// TODO keep in-memory database of created accounts and offers. Randomly remove elements from it to keep size.

// Type                 OperationType
// CreateAccountOp      *CreateAccountOp
// PaymentOp            *PaymentOp
// PathPaymentOp        *PathPaymentOp
// ManageOfferOp        *ManageOfferOp
// CreatePassiveOfferOp *CreatePassiveOfferOp
// SetOptionsOp         *SetOptionsOp
// ChangeTrustOp        *ChangeTrustOp
// AllowTrustOp         *AllowTrustOp
// Destination          *AccountId
// ManageDataOp         *ManageDataOp

// CREATE_ACCOUNT = 0,
// PAYMENT = 1,
// PATH_PAYMENT = 2,
// MANAGE_OFFER = 3,
// CREATE_PASSIVE_OFFER = 4,
// SET_OPTIONS = 5,
// CHANGE_TRUST = 6,
// ALLOW_TRUST = 7,
// ACCOUNT_MERGE = 8,
// INFLATION = 9,
// MANAGE_DATA = 10
