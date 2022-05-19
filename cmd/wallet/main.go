package main

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"os"
	"strconv"

	"github.com/mcfx/tcoin/core/block"
	"github.com/mcfx/tcoin/utils/address"
)

const rpcUrl = "https://uarpc.mcfx.us/"

func main() {
	op := os.Args[1]
	if op == "genWallet" {
		_, priv := block.GenKeyPair(rand.Reader)
		fmt.Printf("privkey: %x\n", priv[:])
	} else if op == "showWallet" {
		t, _ := hex.DecodeString(os.Args[2])
		var pubkey block.PubkeyType
		copy(pubkey[:], t[32:])
		addr := block.PubkeyToAddress(pubkey)
		eaddr := address.EncodeAddr(addr)
		fmt.Printf("Address: %s\n", eaddr)
		data, _ := json.Marshal(map[string]string{"addr": eaddr})
		resp, err := http.Post(rpcUrl+"get_account_info", "application/json", bytes.NewBuffer(data))
		if err != nil {
			panic(err)
		}
		var res struct {
			Status bool              `json:"status"`
			Msg    string            `json:"msg"`
			Data   block.AccountInfo `json:"data"`
		}
		json.NewDecoder(resp.Body).Decode(&res)
		if !res.Status {
			panic(res.Msg)
		}
		fmt.Printf("Balance: %f\n", float64(res.Data.Balance)/1e9)
		fmt.Printf("Nonce: %d\n", res.Data.Nonce)
	} else if op == "transfer" {
		t, _ := hex.DecodeString(os.Args[2])
		to := os.Args[3]
		toAddr, err := address.ParseAddr(to)
		if err != nil {
			panic(err)
		}
		amountF, err := strconv.ParseFloat(os.Args[4], 64)
		if err != nil {
			panic(err)
		}
		amount := int(math.Round(amountF * 1e9))
		var pubkey block.PubkeyType
		var privkey block.PrivkeyType
		copy(pubkey[:], t[32:])
		copy(privkey[:], t)
		tx := &block.Transaction{
			TxType:       1,
			SenderPubkey: pubkey,
			Receiver:     toAddr,
			Value:        uint64(amount),
			GasLimit:     0,
			Fee:          0,
			Nonce:        0,
			Data:         []byte{},
		}
		tx.Sign(privkey)
		var buf bytes.Buffer
		err = block.EncodeTx(&buf, tx)
		if err != nil {
			panic(err)
		}
		data, _ := json.Marshal(map[string][]byte{"tx": buf.Bytes()})
		resp, err := http.Post(rpcUrl+"submit_tx", "application/json", bytes.NewBuffer(data))
		if err != nil {
			panic(err)
		}
		var res map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&res)
		if !res["status"].(bool) {
			panic(res["msg"].(string))
		}
	}
}
