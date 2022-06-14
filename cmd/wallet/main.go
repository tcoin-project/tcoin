package main

import (
	"bufio"
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"hash/fnv"
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

func asAsmByteArr(s []byte) string {
	for len(s)%4 != 0 {
		s = append(s, 0)
	}
	res := []string{}
	for _, x := range s {
		res = append(res, fmt.Sprintf(".byte %d", x))
	}
	return strings.Join(res, "\n")
}

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

	sendTx := func(txType byte, toAddr block.AddressType, amount uint64, s []byte, gasLimit uint64) {
		ai := readWallet(eaddr)
		tx := &block.Transaction{
			TxType:       txType,
			SenderPubkey: pubkey,
			Receiver:     toAddr,
			Value:        amount,
			GasLimit:     gasLimit,
			Fee:          0,
			Nonce:        ai.Nonce,
			Data:         s,
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
			fmt.Printf("Balance: %f (%d)\n", float64(ai.Balance)/1e9, ai.Balance)
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
			sendTx(1, toAddr, uint64(amount), []byte(msg), 40000+uint64(len(msg)))
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
		case "deploy":
			b, err := ioutil.ReadFile(cmd[0])
			if err != nil {
				panic(err)
			}
			asmx := []string{
				"mv s0, ra",
				"addi a0, sp, -32",
				"la a1, code",
				fmt.Sprintf("li a2, %d", len(b)),
				fmt.Sprintf("li a3, %d", block.CREATE_INIT|block.CREATE_TRIMELF),
				fmt.Sprintf("li a4, %d", 0),
				fmt.Sprintf("li t0, -%d", block.SYSCALL_CREATE*8),
				"srli t0, t0, 1",
				"jalr t0",
				"addi a0, a0, -8",
				"li a1, 32",
				"sd a1, 0(a0)",
				"mv ra, s0",
				"ret",
				"code:",
				asAsmByteArr(b),
			}
			code := vm.AsmToBytes(strings.Join(asmx, "\n"))
			code2 := vm.AsmToBytes(strings.Join(append(asmx[:len(asmx)-7], asmx[len(asmx)-4:]...), "\n"))
			xaddr, t := runViewRawCode(eaddr, code)
			if t != "" {
				fmt.Printf("Error happened: %s\n", t)
				return
			}
			var contractAddr block.AddressType
			copy(contractAddr[:], xaddr)
			gas, t := estimateGas(eaddr, code2)
			if t != "" {
				fmt.Printf("Error happened: %s\n", t)
				return
			}
			fmt.Printf("gas: %d\n", gas)
			fmt.Printf("addr: %s\n", address.EncodeAddr(contractAddr))
			sendTx(2, block.AddressType{}, 0, code2, uint64(gas))
		case "get":
			caddrt := cmd[0]
			caddr, err := address.ParseAddr(caddrt)
			if err != nil {
				panic(err)
			}
			var asm []string
			hs := fnv.New32a()
			hs.Write([]byte(cmd[1]))
			selector := int32(hs.Sum32())
			switch cmd[1] {
			case "totalSupply":
				asm = []string{
					fmt.Sprintf("li a0, %d", selector),
					"li a1, 0",
					"jalr s1",
					"sd a0, -8(sp)",
					"li a0, 8",
					"sd a0, -16(sp)",
					"addi a0, sp, -16",
				}
			default:
				panic(fmt.Sprintf("%s not supported", cmd[1]))
			}
			asm = append([]string{
				"mv s0, ra",
				"la a0, caddr",
				fmt.Sprintf("li t0, -%d", block.SYSCALL_LOAD_CONTRACT*8),
				"srli t0, t0, 1",
				"jalr t0",
				"mv s1, a0",
			}, asm...)
			asm = append(asm,
				"mv ra, s0",
				"ret",
				"caddr:",
				asAsmByteArr(caddr[:]),
			)
			code := vm.AsmToBytes(strings.Join(asm, "\n"))
			x, t := runViewRawCode(eaddr, code)
			if t != "" {
				fmt.Printf("Error happened: %s\n", t)
				return
			}
			switch cmd[1] {
			case "totalSupply":
				fmt.Printf("total supply: %d\n", binary.LittleEndian.Uint64(x))
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
