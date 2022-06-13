package main

import (
	"bufio"
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/mcfx/tcoin/core/block"
	"github.com/mcfx/tcoin/utils/address"
)

var rpcUrl = "https://uarpc.mcfx.us/"

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

func main() {
	tmp := os.Args[1]
	if tmp == "gen" {
		_, priv := block.GenKeyPair(rand.Reader)
		fmt.Printf("privkey: %x\n", ed25519.PrivateKey(priv[:]).Seed())
		return
	}
	if len(os.Args) >= 3 {
		rpcUrl = os.Args[2]
	}
	tmpb, err := hex.DecodeString(tmp)
	if err != nil {
		panic(err)
	}
	var pubkey block.PubkeyType
	var privkey block.PrivkeyType
	tmps := ed25519.NewKeyFromSeed(tmpb)
	copy(privkey[:], tmps)
	copy(pubkey[:], tmps[32:])
	addr := block.PubkeyToAddress(pubkey)
	eaddr := address.EncodeAddr(addr)
	rd := bufio.NewReader(os.Stdin)
	fmt.Printf("Address: %s\n", eaddr)
	for {
		fmt.Printf("> ")
		line, err := rd.ReadString('\n')
		if err != nil {
			panic(err)
		}
		cmd := strings.Split(line[:len(line)-1], " ")
		op := cmd[0]
		cmd = cmd[1:]
		switch op {
		case "exit":
			return
		case "show":
			fmt.Printf("Address: %s\n", eaddr)
			ai := readWallet(eaddr)
			fmt.Printf("Balance: %f\n", float64(ai.Balance)/1e9)
			fmt.Printf("Nonce: %d\n", ai.Nonce)
		case "transfer":
			to := cmd[0]
			toAddr, err := address.ParseAddr(to)
			if err != nil {
				panic(err)
			}
			amountF, err := strconv.ParseFloat(cmd[1], 64)
			if err != nil {
				panic(err)
			}
			msg := strings.Join(cmd[2:], " ")
			amount := int(math.Round(amountF * 1e9))
			ai := readWallet(eaddr)
			tx := &block.Transaction{
				TxType:       1,
				SenderPubkey: pubkey,
				Receiver:     toAddr,
				Value:        uint64(amount),
				GasLimit:     40000,
				Fee:          0,
				Nonce:        ai.Nonce,
				Data:         []byte(msg),
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
}
