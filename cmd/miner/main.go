package main

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/mcfx/tcoin/core/block"
	"github.com/mcfx/tcoin/utils/address"
)

func main() {
	addr := flag.String("addr", "", "miner address")
	url := flag.String("url", "", "rpc url")
	flag.Parse()
	_, err := address.ParseAddr(*addr)
	if err != nil {
		log.Fatal(err)
	}
	resChan := make(chan *block.Block, 100)
	var curTask *block.Block
	var curDifficulty block.HashType
	taskMut := sync.Mutex{}
	mineLoop := func(id int) {
		epoch := 0
		for {
			taskMut.Lock()
			task := curTask
			df := curDifficulty
			taskMut.Unlock()
			if task == nil {
				time.Sleep(time.Second)
				continue
			}
			bh := task.Header
			bh.ExtraData[0] = byte(id)
			epoch++
			binary.LittleEndian.PutUint64(bh.ExtraData[1:9], uint64(epoch))
			for i := 0; i < 10000000; i++ {
				binary.LittleEndian.PutUint32(bh.ExtraData[8:12], uint32(i))
				h := bh.ComputeHash()
				if bytes.Compare(h[:], df[:]) <= 0 {
					bh.Hash = h
					task.Header = bh
					resChan <- task
					break
				}
			}
		}
	}
	go mineLoop(0)
	go func() {
		for {
			err := func() error {
				blk := <-resChan
				var buf bytes.Buffer
				err := block.EncodeBlock(&buf, blk)
				if err != nil {
					return err
				}
				body := struct {
					Block []byte `json:"block"`
				}{
					Block: buf.Bytes(),
				}
				data, _ := json.Marshal(body)
				resp, err := http.Post(*url+"submit_block", "application/json", bytes.NewBuffer(data))
				if err != nil {
					return err
				}
				var res map[string]interface{}
				json.NewDecoder(resp.Body).Decode(&res)
				if !res["status"].(bool) {
					return errors.New(res["msg"].(string))
				}
				return nil
			}()
			if err != nil {
				log.Printf("submit block error: %v", err)
			} else {
				log.Printf("submit block ok")
			}
		}
	}()
	for {
		err := func() error {
			data, _ := json.Marshal(map[string]string{"addr": *addr})
			resp, err := http.Post(*url+"get_block_candidate", "application/json", bytes.NewBuffer(data))
			if err != nil {
				return err
			}
			var res map[string]interface{}
			json.NewDecoder(resp.Body).Decode(&res)
			if !res["status"].(bool) {
				return errors.New(res["msg"].(string))
			}
			buf, err := base64.StdEncoding.DecodeString(res["block"].(string))
			if err != nil {
				return err
			}
			buff := bytes.NewBuffer(buf)
			nt, err := block.DecodeBlock(buff)
			if err != nil {
				return err
			}
			nh, err := hex.DecodeString(res["difficulty"].(string))
			if err != nil {
				return err
			}
			taskMut.Lock()
			curTask = nt
			copy(curDifficulty[:], nh)
			taskMut.Unlock()
			return nil
		}()
		if err != nil {
			log.Printf("get block candidate error: %v", err)
		} else {
			log.Printf("update task ok")
		}
		time.Sleep(time.Second * 1)
	}
}
