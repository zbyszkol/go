package horizon

import (
	"encoding/base64"
	"encoding/json"
	"time"
)

type Problem struct {
	Type     string                     `json:"type"`
	Title    string                     `json:"title"`
	Status   int                        `json:"status"`
	Detail   string                     `json:"detail,omitempty"`
	Instance string                     `json:"instance,omitempty"`
	Extras   map[string]json.RawMessage `json:"extras,omitempty"`
}

type Account struct {
	Links struct {
		Self         Link `json:"self"`
		Transactions Link `json:"transactions"`
		Operations   Link `json:"operations"`
		Payments     Link `json:"payments"`
		Effects      Link `json:"effects"`
		Offers       Link `json:"offers"`
	} `json:"_links"`

	HistoryAccount
	Sequence             string            `json:"sequence"`
	SubentryCount        int32             `json:"subentry_count"`
	InflationDestination string            `json:"inflation_destination,omitempty"`
	HomeDomain           string            `json:"home_domain,omitempty"`
	Thresholds           AccountThresholds `json:"thresholds"`
	Flags                AccountFlags      `json:"flags"`
	Balances             []Balance         `json:"balances"`
	Signers              []Signer          `json:"signers"`
	Data                 map[string]string `json:"data"`
}

func (a Account) GetNativeBalance() string {
	for _, balance := range a.Balances {
		if balance.Asset.Type == "native" {
			return balance.Balance
		}
	}

	return "0"
}

// MustGetData returns decoded value for a given key. If the key does
// not exist, empty slice will be returned. If there is an error
// decoding a value, it will panic.
func (this *Account) MustGetData(key string) []byte {
	bytes, err := this.GetData(key)
	if err != nil {
		panic(err)
	}
	return bytes
}

// GetData returns decoded value for a given key. If the key does
// not exist, empty slice will be returned.
func (this *Account) GetData(key string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(this.Data[key])
}

type AccountFlags struct {
	AuthRequired  bool `json:"auth_required"`
	AuthRevocable bool `json:"auth_revocable"`
}

type AccountThresholds struct {
	LowThreshold  byte `json:"low_threshold"`
	MedThreshold  byte `json:"med_threshold"`
	HighThreshold byte `json:"high_threshold"`
}

type Asset struct {
	Type   string `json:"asset_type"`
	Code   string `json:"asset_code,omitempty"`
	Issuer string `json:"asset_issuer,omitempty"`
}

type Balance struct {
	Balance string `json:"balance"`
	Limit   string `json:"limit,omitempty"`
	Asset
}

type HistoryAccount struct {
	ID        string `json:"id"`
	PT        string `json:"paging_token"`
	AccountID string `json:"account_id"`
}

type Link struct {
	Href      string `json:"href"`
	Templated bool   `json:"templated,omitempty"`
}

type TransactionSuccess struct {
	Links struct {
		Transaction Link `json:"transaction"`
	} `json:"_links"`
	Hash   string `json:"hash"`
	Ledger int32  `json:"ledger"`
	Env    string `json:"envelope_xdr"`
	Result string `json:"result_xdr"`
	Meta   string `json:"result_meta_xdr"`
}

// TransactionResultCodes represent a summary of result codes returned from
// a single xdr TransactionResult
type TransactionResultCodes struct {
	TransactionCode string   `json:"transaction"`
	OperationCodes  []string `json:"operations,omitempty"`
}

type Signer struct {
	PublicKey string `json:"public_key"`
	Weight    int32  `json:"weight"`
	Key       string `json:"key"`
	Type      string `json:"type"`
}

type Payment struct {
	ID          string `json:"id"`
	Type        string `json:"type"`
	PagingToken string `json:"paging_token"`

	Links struct {
		Transaction struct {
			Href string `json:"href"`
		} `json:"transaction"`
	} `json:"_links"`

	// payment/path_payment fields
	From        string `json:"from"`
	To          string `json:"to"`
	AssetType   string `json:"asset_type"`
	AssetCode   string `json:"asset_code"`
	AssetIssuer string `json:"asset_issuer"`
	Amount      string `json:"amount"`

	// transaction fields
	Memo struct {
		Type  string `json:"memo_type"`
		Value string `json:"memo"`
	}
}

type Transaction struct {
	ID              string    `json:"id"`
	PagingToken     string    `json:"paging_token"`
	Hash            string    `json:"hash"`
	Ledger          int32     `json:"ledger"`
	LedgerCloseTime time.Time `json:"created_at"`
	Account         string    `json:"source_account"`
	AccountSequence string    `json:"source_account_sequence"`
	FeePaid         int32     `json:"fee_paid"`
	OperationCount  int32     `json:"operation_count"`
	EnvelopeXdr     string    `json:"envelope_xdr"`
	ResultXdr       string    `json:"result_xdr"`
	ResultMetaXdr   string    `json:"result_meta_xdr"`
	FeeMetaXdr      string    `json:"fee_meta_xdr"`
	MemoType        string    `json:"memo_type"`
	Memo            string    `json:"memo,omitempty"`
	Signatures      []string  `json:"signatures"`
	ValidAfter      string    `json:"valid_after,omitempty"`
	ValidBefore     string    `json:"valid_before,omitempty"`
}

type Effect struct {
	ID          string `json:"id"`
	Type        string `json:"type"`
	TypeID      int32  `json:"type_i"`
	PagingToken string `json:"paging_token"`
	Account     string `json:"account"`

	Links struct {
		Operation Link `json:"operation"`
		Succeeds  Link `json:"succeeds"`
		Proceeds  Link `json:"precedes"`
	} `json:"_links"`

	// "account_created",
	StartingBalance string `json:"starting_balance"`

	// "account_removed",

	// account_credited, account_debited
	AssetType   string `json:"asset_type"`
	AssetCode   string `json:"asset_code,omitempty"`
	AssetIssuer string `json:"asset_issuer,omitempty"`
	Amount      string `json:"amount"`

	//  "account_thresholds_updated",
	LowThreshold  int32 `json:"low_threshold"`
	MedThreshold  int32 `json:"med_threshold"`
	HighThreshold int32 `json:"high_threshold"`

	//  "account_home_domain_updated",
	HomeDomain string `json:"home_domain"`

	//     "account_flags_updated",
	AuthRequired  *bool `json:"auth_required_flag,omitempty"`
	AuthRevokable *bool `json:"auth_revokable_flag,omitempty"`

	// signer_created, signer_removed, signer_updated
	Weight    int32  `json:"weight"`
	PublicKey string `json:"public_key"`
	Key       string `json:"key"`

	//         "trustline_created, trustline_removed, trustline_updated
	Limit string `json:"limit"`

	//  trustline_authorized, trustline_deauthorized
	Trustor string `json:"trustor"`

	//             "offer_created",
	//             "offer_removed",
	//              "offer_updated",

	//                     "trade",
	Seller            string `json:"seller"`
	OfferID           int64  `json:"offer_id"`
	SoldAmount        string `json:"sold_amount"`
	SoldAssetType     string `json:"sold_asset_type"`
	SoldAssetCode     string `json:"sold_asset_code,omitempty"`
	SoldAssetIssuer   string `json:"sold_asset_issuer,omitempty"`
	BoughtAmount      string `json:"bought_amount"`
	BoughtAssetType   string `json:"bought_asset_type"`
	BoughtAssetCode   string `json:"bought_asset_code,omitempty"`
	BoughtAssetIssuer string `json:"bought_asset_issuer,omitempty"`

	//             "data_created",
	//              "data_removed",
	//              "data_updated",

}
