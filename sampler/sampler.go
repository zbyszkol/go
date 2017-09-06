package sampler

import (
	"github.com/stellar/go/xdr"
	"math/rand"
)

type Database interface {
	// SearchAccount(xdr.AccountId) xdr.AccountEntry
	GetAccountByOrder(uint64) xdr.AccountEntry
	// AddAccount(xdr.AccountEntry)
	GetAccountsNumber() uint64
	// SearchOfferById(uint64) xdr.OfferEntry
	// SearchOfferBySellingAsset(xdr.Asset) xdr.OfferEntry
	// SearchOfferByBuyingAsset(xdr.Asset) xdr.OfferEntry
}

type InMemoryDatabase struct {
	orderedData map[uint64]xdr.AccountEntry
	// map[xdr.PublicKey]AccountEntry
}

func (data *InMemoryDatabase) GetAccountByOrder(order uint64) xdr.AccountEntry {
	return data.orderedData[order]
}

func (data *InMemoryDatabase) GetAccountsNumber() uint64 {
	return len(data.orderedData)
}

type Size uint64

type TransactionGenerator func(Size) xdr.Transaction

type OperationGenerator func(Size, Database) xdr.Operation

type OperationGeneratorGenerator func(Size) OperationGenerator

type GeneratorsList []struct {
	Generator func(xdr.AccountEntry) OperationGenerator
	Bias      uint32
}

type TransactionsSampler struct {
	database   Database
	generators OperationGeneratorGenerator
}

func createOperationGenerator(generators GeneratorsList) OperationGeneratorGenerator {
	return func(size Size) OperationGenerator {
		return getRandomGenerator(generators)
	}
}

func CreateTransactionsGenerator() TransactionGenerator {
	return nil
}

func (sampler *TransactionsSampler) Generate(size Size) func() xdr.Transaction {
	return func() xdr.Transaction {
		sourceAccount := getRandomAccount(sampler.database)
		transactionSize := rand.Intn(100) + 1
		var transaction xdr.Transaction
		transaction.Operations = make([]xdr.Operation, transactionSize)
		for it := 0; it < transactionSize; it++ {
			generator := getRandomGenerator(sourceAccount)
			transaction.Operations[it] = generator.Generate(1)
		}
		return transaction
	}
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
	return func(Size, Database) xdr.Operation {
		accountOp := generateValidCreateAccountOp(sourceAccount)
		return xdr.Operation{Type: xdr.OperationTypeCreateAccount, CreateAccountOp: accountOp}
	}
}
func generateValidCreateAccountOp(sourceAccount xdr.AccountEntry) xdr.CreateAccountOp {
	destinationAccountId := generateNewAccountId()
	startingBalance := rand.Intn(sourceAccount.Balance) + 1
	return xdr.CreateAccountOp{Destination: destinationAccountId, StartingBalance: startingBalance}
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
	return func(Size, database Database) xdr.Operation {
		paymentOp := generateValidPaymentOp(sourceAccount, database)
		return xdr.Operation{Type: xdr.OperationTypePayment, PaymentOp: paymentOp}
	}
}

func generateValidPaymentOp(sourceAccount xdr.AccountEntry, database Database) xdr.PaymentOp {
	destinationAccount := getRandomAccount(database)
	trustLine, destTrustLine := getRandomTrustLine(sourceAccount, destinationAccount, database)
	amount := rand.Intn(math.min(trustLine.Balance, destTrustLine.Limit))
	return xdr.PaymentOp{Destination: destinationAccount.AccountId, Asset: trustLine.Asset, Amount: amount}
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
func getValidCreateOfferOpOperation(sourceAccount xdr.AccountEntry) OperationGenerator {
	return func(Size, database Database) xdr.Operation {
		createOffer := generateValidCreateManagerOfferOp(sourceAccount, database)
		return xdr.Operation{Type: xdr.OperationTypeManageOffer, ManageOfferOp: createOffer}
	}
}

func generateValidCreateManagerOfferOp(sourceAccount xdr.AccountEntry, database Database) xdr.ManageOfferOp {
	consumable := rand.Intn(2) == 0
	var sellingAsset, buyingAsset xdr.Asset
	var maxAmount xdr.Int64
	var price xdr.Price

	if consumable {
		sellingAsset, buyingAsset, maxAmount, price = getRandomConsumableAssets(sourceAccount)
	} else {
		sellingAsset, buyingAsset, maxAmount, price = getRandomAssets(sourceAccount)
	}
	amount := rand.Intn(maxAmount) + 1
	const NewOfferId xdr.Uint64 = 0
	return xdr.ManageOfferOp{Selling: sellingAsset, Buying: buyingAsset, Amount: amount, Price: price, OfferId: NewOfferId}
}

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
func getValidChangeTrustOpOperation(sourceAccount xdr.AccountEntry) OperationGenerator {
	return func(Size, database Database) xdr.Operation {
		changeTrust := generateValidChangeTrustOp(sourceAccount, database)
		return xdr.Operation{Type: xdr.OperationTypeManageOffer, ManageOfferOp: createOffer}
	}
}

func generateValidChangeTrustOp(sourceAccount xdr.AccountEntry, database Database) {
	trustLine := getRandomAsset(database)
	limit := rand.Int63()
	return ChangeTrustOp{Line: trustLine, Limit: limit}
}

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
