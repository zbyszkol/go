// +build gofuzz

package txresult

import (
	"github.com/stellar/go/xdr"
	"bytes"
	"fmt"
)

func Fuzz(data []byte) int {
	var txr xdr.TransactionResult
	err := xdr.SafeUnmarshal(data, &txr)
	if err != nil {
		return 0
	}

	var out bytes.Buffer
	_, err = xdr.Marshal(&out, txr)
	if err != nil {
		fmt.Println("failed to encode:", err)
		fmt.Printf("%#v", data)
		panic(err)
	}

	if !bytes.Equal(data, out.Bytes()) {
		fmt.Println("failed to roundtrip")
		fmt.Printf("%#v", data)
		panic("failed to roundtrip")
	}

	return 1
}
