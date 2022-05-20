package main

import (
	"bytes"
	crand "crypto/rand"
	"encoding/json"
	"log"
	"net/http"

	"github.com/mcfx/tcoin/core/block"
	"github.com/mcfx/tcoin/utils/address"
)

const rpcUrl = "https://uarpc.mcfx.us/"

func readWallet(addr string) block.AccountInfo {
	data, _ := json.Marshal(map[string]string{"addr": addr})
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
	return res.Data
}

type account struct {
	pubkey  block.PubkeyType
	privkey block.PrivkeyType
	addr    block.AddressType
	eaddr   string
}

func newAccount() account {
	var a account
	a.pubkey, a.privkey = block.GenKeyPair(crand.Reader)
	a.addr = block.PubkeyToAddress(a.pubkey)
	a.eaddr = address.EncodeAddr(a.addr)
	return a
}

func main() {
	as := []account{}
	n := 10
	m := 10
	for i := 0; i < n; i++ {
		as = append(as, newAccount())
		log.Printf("account %d: %s", i, as[i].eaddr)
	}
	for i := 0; i < m; i++ {
		log.Printf("round %d", i)
		for j := 0; j < n; j++ {
			k := (i + j + 1) % n
			tx := &block.Transaction{
				TxType:       1,
				SenderPubkey: as[j].pubkey,
				Receiver:     as[k].addr,
				Value:        0,
				GasLimit:     0,
				Fee:          0,
				Nonce:        uint64(i),
				Data:         []byte{},
			}
			tx.Sign(as[j].privkey)
			var buf bytes.Buffer
			err := block.EncodeTx(&buf, tx)
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
}
