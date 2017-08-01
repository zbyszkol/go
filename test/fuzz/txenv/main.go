// +build gofuzz

package txenv

import (
	"github.com/stellar/go/xdr"
	"bytes"
	"fmt"
)

func Fuzz(data []byte) int {
	var txe xdr.TransactionEnvelope
	err := xdr.SafeUnmarshal(data, &txe)
	if err != nil {
		return 0
	}

	var out bytes.Buffer
	_, err = xdr.Marshal(&out, txe)
	if err != nil {
		fmt.Println("failed to roundtrip")
		fmt.Printf("%#v", data)
		panic(err)
	}

	return 1
}
