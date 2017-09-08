package sampler

import (
	"github.com/stellar/go/xdr"
	"math"
	"math/rand"
	"net/http"
	"net/url"
	"time"
)

type Database interface {
	// SearchAccount(xdr.AccountId) xdr.AccountEntry
	GetAccountByOrder(int) xdr.AccountEntry
	AddAccount(xdr.AccountEntry)
	GetAccountsNumber() int
	GetTrustLineByOrder(int) xdr.TrustLineEntry
	GetTrustLineCount() int
	// SearchOfferById(uint64) xdr.OfferEntry
	// SearchOfferBySellingAsset(xdr.Asset) xdr.OfferEntry
	// SearchOfferByBuyingAsset(xdr.Asset) xdr.OfferEntry
}

func Test(database Database) {
	sampler := TransactionsSampler{}
	for {
		transactionSize := Size(rand.Int63n(100) + 1)
		transaction := sampler.Generate(transactionSize)()
		sendError := sendTransaction("127.0.0.1:11626", &transaction)
		if sendError != nil {
			panic("abandon ship!")
		}

	}
}

func sendTransaction(address string, transaction *xdr.Transaction) error {
	coreUrl, urlError := url.Parse("http://" + address)
	if urlError != nil {
		return urlError
	}
	coreUrl.Path += "/tx"
	parameters := url.Values{}
	txData, txError := xdr.MarshalBase64(transaction)
	if txError != nil {
		return txError
	}
	parameters.Add("blob", txData)
	coreUrl.RawQuery = parameters.Encode()
	var httpClient = &http.Client{Timeout: time.Second * 5}
	_, getError := httpClient.Get(coreUrl.RequestURI())
	if getError != nil {
		return getError
	}
	return nil
}

type InMemoryDatabase struct {
	// orderedData map[uint64]xdr.AccountEntry
	orderedData       []xdr.AccountEntry
	orderedTrustlines []xdr.TrustLineEntry
	// map[xdr.PublicKey]AccountEntry
}

func deleteElement(ix int, data []int) []int {
	result := data
	if ix < int(math.Floor(float64(len(data))/float64(2))) {
		// shift right first part
		for i := 0; i < ix; i++ {
			data[i+1] = data[i]
		}
		result = data[1:]
	} else {
		// shift left second part
		for i := ix + 1; i < len(data); i++ {
			data[i-1] = data[i]
		}
		result = data[:len(data)-1]
	}
	return result
}

func (data *InMemoryDatabase) GetAccountByOrder(order int) xdr.AccountEntry {
	return data.orderedData[order]
}

func (data *InMemoryDatabase) GetAccountsNumber() int {
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

type GeneratorsList []struct {
	Generator func(xdr.AccountEntry) OperationGenerator
	Bias      uint32
}

type TransactionsSampler struct {
	database   Database
	generators func(xdr.AccountEntry) OperationGenerator
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
func (sampler *TransactionsSampler) Generate(size Size) func() xdr.Transaction {
	return func() xdr.Transaction {
		sourceAccount := getRandomAccount(sampler.database)
		const minimalFee = 100
		fee := getRandomFee(sourceAccount)
		var transaction xdr.Transaction
		transaction.SourceAccount = sourceAccount.AccountId
		transaction.Fee = xdr.Uint32(fee)
		transaction.SeqNum = sourceAccount.SeqNum
		transaction.Operations = make([]xdr.Operation, size)
		generator := sampler.generators(sourceAccount)
		for it := Size(0); it < size; it++ {
			transaction.Operations[it] = generator(1, sampler.database)
		}
		return transaction
	}
}

func getRandomFee(account xdr.AccountEntry) uint32 {
	return 0
}

func geometric(p float64) int64 {
	return 4
}

func getRandomGeneratorWrapper(generatorsList GeneratorsList) func(xdr.AccountEntry) OperationGenerator {
	return func(sourceAccount xdr.AccountEntry) OperationGenerator {
		return getRandomGenerator(generatorsList, sourceAccount)
	}
}

func getRandomGenerator(generators GeneratorsList, sourceAccount xdr.AccountEntry) OperationGenerator {
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

// type SizeMapper func(StructField) Size

// type ValueGenerator func(StructField) interface{}

// func Sample(argType reflect.Type, sizeMapper SizeMapper, valueGenerator ValueGenerator) Generator {
// 	expressions := buildExpressions(argType, sizeMapper)
// 	return generate(expressions, argType, size, ValueGenerator)
// }
// type OperationBody struct {
// 	Type                 OperationType
// 	CreateAccountOp      *CreateAccountOp
// 	PaymentOp            *PaymentOp
// 	PathPaymentOp        *PathPaymentOp
// 	ManageOfferOp        *ManageOfferOp
// 	CreatePassiveOfferOp *CreatePassiveOfferOp
// 	SetOptionsOp         *SetOptionsOp
// 	ChangeTrustOp        *ChangeTrustOp
// 	AllowTrustOp         *AllowTrustOp
// 	Destination          *AccountId
// 	ManageDataOp         *ManageDataOp
// }
// type OperationType int32

// const (
// 	OperationTypeCreateAccount      OperationType = 0
// 	OperationTypePayment            OperationType = 1
// 	OperationTypePathPayment        OperationType = 2
// 	OperationTypeManageOffer        OperationType = 3
// 	OperationTypeCreatePassiveOffer OperationType = 4
// 	OperationTypeSetOptions         OperationType = 5
// 	OperationTypeChangeTrust        OperationType = 6
// 	OperationTypeAllowTrust         OperationType = 7
// 	OperationTypeAccountMerge       OperationType = 8
// 	OperationTypeInflation          OperationType = 9
// 	OperationTypeManageData         OperationType = 10
// )
func getValidCreateAccoutOpOperation(sourceAccount xdr.AccountEntry) OperationGenerator {
	return func(size Size, database Database) xdr.Operation {
		accountOp, accountEntry := generateValidCreateAccountOp(sourceAccount)
		database.AddAccount(accountEntry)
		body := xdr.OperationBody{Type: xdr.OperationTypeCreateAccount, CreateAccountOp: &accountOp}
		return xdr.Operation{SourceAccount: &sourceAccount.AccountId, Body: body}
	}
}

func generateNewAccountEntry() xdr.AccountEntry {
	return xdr.AccountEntry{}
}

func generateValidCreateAccountOp(sourceAccount xdr.AccountEntry) (xdr.CreateAccountOp, xdr.AccountEntry) {
	destinationAccount := generateNewAccountEntry()
	startingBalance := rand.Int63n(int64(sourceAccount.Balance)) + 1
	destinationAccount.Balance = xdr.Int64(startingBalance)
	return xdr.CreateAccountOp{Destination: destinationAccount.AccountId, StartingBalance: destinationAccount.Balance}, destinationAccount
}

// type PaymentOp struct {
// 	Destination AccountId
// 	Asset       Asset
// 	Amount      Int64
// }
// type TrustLineEntry struct {
// 	AccountId AccountId
// 	Asset     Asset
// 	Balance   Int64
// 	Limit     Int64
// 	Flags     Uint32
// 	Ext       TrustLineEntryExt
// }
func getValidPaymentOpOperation(sourceAccount xdr.AccountEntry) OperationGenerator {
	return func(size Size, database Database) xdr.Operation {
		paymentOp := generateValidPaymentOp(sourceAccount, database)
		body := xdr.OperationBody{Type: xdr.OperationTypePayment, PaymentOp: &paymentOp}
		return xdr.Operation{SourceAccount: &sourceAccount.AccountId, Body: body}
	}
}

func min(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}

func generateValidPaymentOp(sourceAccount xdr.AccountEntry, database Database) xdr.PaymentOp {
	destinationAccount := getRandomAccount(database)
	trustLine, destTrustLine := getRandomTrustLine(sourceAccount, destinationAccount, database)
	amount := rand.Int63n(min(int64(trustLine.Balance), int64(destTrustLine.Limit)))
	return xdr.PaymentOp{Destination: destinationAccount.AccountId, Asset: trustLine.Asset, Amount: xdr.Int64(amount)}
}

func getRandomTrustLine(sourceAccount, destinationAccount xdr.AccountEntry, database Database) (xdr.TrustLineEntry, xdr.TrustLineEntry) {
	trustLine := xdr.TrustLineEntry{}
	return trustLine, trustLine
}

func getRandomAccount(database Database) xdr.AccountEntry {
	return database.GetAccountByOrder(rand.Intn(database.GetAccountsNumber()))
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
