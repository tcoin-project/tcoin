package main

import (
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

	"github.com/chzyer/readline"
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
	debugBuiltin := false
	if len(os.Args) >= 3 {
		rpcUrl = os.Args[2]
	}
	if len(os.Args) >= 4 {
		if os.Args[3] == "debug" {
			debugBuiltin = true
		}
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
		case "read", "write":
			caddrt := cmd[0]
			caddr, err := address.ParseAddr(caddrt)
			if err != nil {
				panic(err)
			}
			funcs := map[string]string{
				"name":         "c",
				"symbol":       "c",
				"decimals":     "i",
				"totalSupply":  "i",
				"balanceOf":    "ia",
				"transfer":     "iai",
				"transferFrom": "iaai",
				"approve":      "iai",
				"allowance":    "iaa",
			}
			cmdo := cmd
			op2 := cmd[1]
			cmd = cmd[2:]
			if strings.Contains(op2, "(") && !strings.Contains(op2, ")") {
				op2 = op2 + " " + cmd[0]
				cmd = cmd[1:]
			}
			funcName := ""
			funcSig := ""
			callValue := uint64(0)
		ofor:
			for {
				switch op2[len(op2)-1] {
				case ']':
					t := strings.Split(op2[:len(op2)-1], "[")
					if len(t) != 2 {
						panic("can't parse " + op2)
					}
					op2 = t[0]
					funcSig = t[1]
					continue
				case ')':
					t := strings.Split(op2[:len(op2)-1], "(")
					if len(t) != 2 {
						panic("can't parse " + op2)
					}
					op2 = t[0]
					u := strings.ToLower(t[1])
					if len(u) >= 6 && u[len(u)-6:] == " tcoin" {
						f, err := strconv.ParseFloat(u[:len(u)-6], 64)
						if err != nil {
							panic(err)
						}
						callValue = uint64(math.Round(f * 1e9))
					} else {
						i, err := strconv.Atoi(u)
						if err != nil {
							panic(err)
						}
						callValue = uint64(i)
					}
					continue
				default:
					break ofor
				}
			}
			funcName = op2
			if funcSig == "" {
				if t, ok := funcs[op2]; ok {
					funcSig = t
				} else {
					panic(fmt.Sprintf("function signature of %s is unknown", op2))
				}
			}
			hs := fnv.New32a()
			hs.Write([]byte(funcName))
			selector := int32(hs.Sum32())
			fmt.Printf("%s %s %d\n", funcName, funcSig, callValue)
			genWorker := func(argSpec string) []string {
				s := []string{
					"j later",
					"calldata:",
				}
				at := argSpec[1:]
				var a1s string
				if len(at) == 0 {
					a1s = "li a1, 0"
				} else if len(at) == 1 && at[0] == 'i' {
					x, err := strconv.Atoi(cmd[0])
					if err != nil {
						panic(err)
					}
					a1s = fmt.Sprintf("li a1, %d", x)
				} else {
					a1s = "la a1, calldata"
					datas := []string{}
					pos := 0x10000020 + 0x8*len(at)
					for i := 0; i < len(at); i++ {
						var k uint64
						if at[i] == 'i' {
							x, err := strconv.Atoi(cmd[i])
							if err != nil {
								panic(err)
							}
							k = uint64(x)
						} else if at[i] == 'a' {
							k = uint64(pos)
							addr, err := address.ParseAddr(cmd[i])
							if err != nil {
								t, err2 := hex.DecodeString(cmd[i])
								if err2 != nil {
									panic(err)
								}
								if len(t) != block.AddressLen {
									panic("hex address length invalid")
								}
								copy(addr[:], t)
							}
							datas = append(datas, asAsmByteArr(addr[:]))
							pos += len(addr)
						}
						buf := make([]byte, 8)
						binary.LittleEndian.PutUint64(buf, k)
						s = append(s, asAsmByteArr(buf))
					}
					if len(at) == 1 {
						s = append(s[:len(s)-1], datas[0])
					} else {
						s = append(s, datas...)
					}
				}
				if callValue == 0 {
					s = append(s,
						"later:",
						fmt.Sprintf("li a0, %d", selector),
						a1s,
						"jalr s1",
					)
				} else {
					s = append(s,
						"later:",
						"mv a0, s1",
						fmt.Sprintf("li a1, %d", selector),
						strings.Replace(a1s, "a1", "a2", 1),
						fmt.Sprintf("li a3, %d", callValue),
						"li a4, 1000000000",
						"addi a5, sp, -1200",
						"addi a6, a5, 8",
						fmt.Sprintf("li t0, -%d", block.SYSCALL_PROTECTED_CALL*8),
						"srli t0, t0, 1",
						"jalr t0",
						"lb t1, 0(a5)",
						"bne t1, zero, success",
						"mv a0, a6",
						fmt.Sprintf("li t0, -%d", block.SYSCALL_REVERT*8),
						"srli t0, t0, 1",
						"jalr t0",
						"success:",
					)
				}
				if op == "write" {
					return s
				}
				if argSpec[0] == 'i' {
					s = append(s,
						"sd a0, -8(sp)",
						"li a0, 8",
						"sd a0, -16(sp)",
						"addi a0, sp, -16",
					)
				} else if argSpec[0] == 'c' {
					s = append(s,
						"addi a1, sp, -1024",
						"mv a2, a1",
						"loop:",
						"lb a3, 0(a0)",
						"beq a3, zero, final",
						"sb a3, 0(a2)",
						"addi a0, a0, 1",
						"addi a2, a2, 1",
						"j loop",
						"final:",
						"addi a0, a1, -8",
						"sub a2, a2, a1",
						"sd a2, 0(a0)",
					)
				} else if argSpec[0] == 'a' {
					s = append(s,
						"ld t0, 0(a0)",
						"sd t0, -32(sp)",
						"ld t0, 8(a0)",
						"sd t0, -24(sp)",
						"ld t0, 16(a0)",
						"sd t0, -16(sp)",
						"ld t0, 24(a0)",
						"sd t0, -8(sp)",
						"addi a0, sp, -40",
						"li t0, 32",
						"sd t0, 0(a0)",
					)
				}
				return s
			}
			postProcess := func(r byte, data []byte) {
				if r == 'i' {
					fmt.Printf("%s: %d\n", strings.Join(cmdo[1:], " "), binary.LittleEndian.Uint64(data))
				} else if r == 'c' {
					fmt.Printf("%s: %s\n", strings.Join(cmdo[1:], " "), string(data))
				} else if r == 'a' {
					var addr block.AddressType
					copy(addr[:], data)
					fmt.Printf("%s: %s\n", strings.Join(cmdo[1:], " "), address.EncodeAddr(addr))
				}
			}
			asm := append([]string{
				"mv s0, ra",
				"la a0, caddr",
				fmt.Sprintf("li t0, -%d", block.SYSCALL_LOAD_CONTRACT*8),
				"srli t0, t0, 1",
				"jalr t0",
				"mv s1, a0",
			}, genWorker(funcSig)...)
			asm = append(asm,
				"mv ra, s0",
				"ret",
				"caddr:",
				asAsmByteArr(caddr[:]),
			)
			code := vm.BuiltinAsmToBytes(strings.Join(asm, "\n"))
			if debugBuiltin {
				code2 := vm.AsmToBytes(strings.Join(asm, "\n"))
				if !bytes.Equal(code, code2) {
					fmt.Printf("%x\n", code)
					fmt.Printf("%x\n", code2)
					panic("not equal")
				}
			}
			fmt.Printf("%x\n", code)
			if op == "read" {
				x, t := runViewRawCode(eaddr, code)
				if t != "" {
					fmt.Printf("Error happened: %s\n", t)
					return
				}
				postProcess(funcSig[0], x)
			} else if op == "write" {
				gas, t := estimateGas(eaddr, code)
				if t != "" {
					fmt.Printf("Error happened: %s\n", t)
					return
				}
				fmt.Printf("gas: %d\n", gas)
				sendTx(2, block.AddressType{}, 0, code, uint64(gas))
			}
		case "parse":
			addr, err := address.ParseAddr(cmd[0])
			if err != nil {
				panic(err)
			}
			for i := 0; i < 4; i++ {
				fmt.Printf("%d\n", binary.LittleEndian.Uint64(addr[i*8:i*8+8]))
			}
		}
	}
	l, err := readline.NewEx(&readline.Config{
		Prompt:            "> ",
		HistoryFile:       "~/.tcoinwallet_history",
		InterruptPrompt:   "^C",
		EOFPrompt:         "exit",
		HistorySearchFold: true,
	})
	if err != nil {
		panic(err)
	}
	defer l.Close()
	for {
		line, err := l.Readline()
		if err != nil {
			break
		}
		line = strings.Trim(line, " ")
		if line == "exit" {
			break
		}
		cmd := strings.Split(line, " ")
		process(cmd)
	}
}
