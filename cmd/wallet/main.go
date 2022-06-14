package main

import (
	"bufio"
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/mcfx/tcoin/core/block"
	"github.com/mcfx/tcoin/utils/address"
	"github.com/mcfx/tcoin/vm"
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

func estimateGas(addr string, code []byte) (int, string) {
	data, _ := json.Marshal(map[string]interface{}{"origin": addr, "code": code})
	resp, err := http.Post(rpcUrl+"estimate_gas", "application/json", bytes.NewBuffer(data))
	if err != nil {
		panic(err)
	}
	var res struct {
		Status bool   `json:"status"`
		Msg    string `json:"msg"`
		Gas    int    `json:"gas"`
		Error  string `json:"error"`
	}
	json.NewDecoder(resp.Body).Decode(&res)
	if !res.Status {
		panic(res.Msg)
	}
	return res.Gas, res.Error
}

func runViewRawCode(addr string, code []byte) ([]byte, string) {
	data, _ := json.Marshal(map[string]interface{}{"origin": addr, "code": code})
	resp, err := http.Post(rpcUrl+"run_view_raw_code", "application/json", bytes.NewBuffer(data))
	if err != nil {
		panic(err)
	}
	var res struct {
		Status bool   `json:"status"`
		Msg    string `json:"msg"`
		Data   []byte `json:"data"`
		Error  string `json:"error"`
	}
	json.NewDecoder(resp.Body).Decode(&res)
	if !res.Status {
		panic(res.Msg)
	}
	return res.Data, res.Error
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
	process := func(cmd []string) {
		defer func() {
			if e := recover(); e != nil {
				fmt.Printf("error: %v\n", e)
			}
		}()
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
		case "estimate_gas_asm":
			b, err := ioutil.ReadFile(cmd[0])
			if err != nil {
				panic(err)
			}
			code := vm.AsmToBytes(string(b))
			x, t := estimateGas(eaddr, code)
			fmt.Printf("Gas used: %d\n", x)
			if t != "" {
				fmt.Printf("Error happened: %s\n", t)
			}
		case "run_view_asm":
			b, err := ioutil.ReadFile(cmd[0])
			if err != nil {
				panic(err)
			}
			code := vm.AsmToBytes(string(b))
			x, t := runViewRawCode(eaddr, code)
			if x != nil {
				fmt.Printf("result: %x\n", x)
			}
			if t != "" {
				fmt.Printf("Error happened: %s\n", t)
			}
		}
	}
	for {
		fmt.Printf("> ")
		line, err := rd.ReadString('\n')
		if err != nil {
			panic(err)
		}
		cmd := strings.Split(line[:len(line)-1], " ")
		process(cmd)
	}
}
