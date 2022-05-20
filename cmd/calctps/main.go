package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/mcfx/tcoin/core/block"
)

const rpcUrl = "http://127.0.0.1:60157/"

func getBlock(id int) *block.Block {
	resp, err := http.Get(rpcUrl + "get_block/" + strconv.Itoa(id))
	if err != nil {
		panic(err)
	}
	var res struct {
		Status    bool   `json:"status"`
		Block     []byte `json:"block"`
		Consensus string `json:"consensus"`
		Height    int    `json:"height"`
		Msg       string `json:"msg"`
	}
	json.NewDecoder(resp.Body).Decode(&res)
	if !res.Status {
		panic(res.Msg)
	}
	if res.Height != id {
		panic("height mismatch")
	}
	buf := bytes.NewBuffer(res.Block)
	b, err := block.DecodeBlock(buf)
	if err != nil {
		panic(err)
	}
	return b
}

func main() {
	s := []*block.Block{}
	for i := 7000; i < 7900; i++ {
		cur := getBlock(i)
		s = append(s, cur)
		if len(s) > 30 {
			o := s[0].Time
			s = s[1:]
			sumtx := 0
			for _, t := range s {
				sumtx += len(t.Txs)
			}
			tps := float64(sumtx) / float64(s[len(s)-1].Time-o) * 1e9
			if tps > 0.1 {
				fmt.Printf("tps %d: %.4f\n", i, tps)
			}
		}
	}
}
