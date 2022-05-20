package core

import (
	"encoding/binary"
	"log"
	"math/rand"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/mcfx/tcoin/core/block"
	"github.com/mcfx/tcoin/core/consensus"
)

func testKeyPair(id int) (block.PubkeyType, block.PrivkeyType) {
	rnd := rand.New(rand.NewSource(114514 + int64(id)))
	a, b := block.GenKeyPair(rnd)
	return a, b
}

func testInitBlock() *block.Block {
	pub, _ := testKeyPair(0)
	b := &block.Block{
		Header: block.BlockHeader{
			ParentHash: block.HashType{0xcc},
			ExtraData:  block.HashType{},
		},
		Miner: block.PubkeyToAddress(pub),
		Time:  114514,
		Txs:   []*block.Transaction{},
	}
	b.FillHash()
	return b
}

func startTestNode(t *testing.T, portBase, id int) *ChainNode {
	config := ChainNodeConfig{
		StoragePath:          "/tmp/tcoin_test/u" + strconv.Itoa(id),
		StorageFinalizeDepth: 20,
		StorageDumpDiskRatio: 0.8,
		ListenPort:           portBase + id,
		MaxConnections:       10,
	}
	os.RemoveAll(config.StoragePath)
	var bi uint64 = 1000000000
	gConfig := ChainGlobalConfig{
		ChainId:      8888,
		GenesisBlock: testInitBlock(),
		GenesisConsensusState: &consensus.ConsensusState{
			Height:           -1,
			LastBlockTime:    0,
			LastKeyBlockTime: 0,
			Difficulty:       block.HashType{0xff},
		},
		GenesisBlockReward: bi * 100,
		BlockReward:        bi,
	}
	cn, err := NewChainNode(config, gConfig, nil)
	if err != nil {
		t.Fatalf("failed to start node %d: %v", id, err)
	}
	return cn
}

func genTestBlocks(n int, ko int) []*block.Block {
	var bi = 1000000000
	pub, _ := testKeyPair(1)
	res := make([]*block.Block, 0)
	lst := testInitBlock().Header.Hash
	for i := 1; i <= n; i++ {
		b := &block.Block{
			Header: block.BlockHeader{
				ParentHash: lst,
				ExtraData:  block.HashType{},
			},
			Miner: block.PubkeyToAddress(pub),
			Time:  uint64(i * bi * 10),
			Txs:   []*block.Transaction{},
		}
		b.FillHash()
		for j := ko; b.Header.Hash[0] != 0; j++ {
			binary.BigEndian.PutUint64(b.Header.ExtraData[:8], uint64(j))
			b.Header.Hash = b.Header.ComputeHash()
		}
		res = append(res, b)
		lst = b.Header.Hash
	}
	return res
}

func testTwoNodes(t *testing.T, n, pb int) {
	cn1 := startTestNode(t, pb, 1)
	cn2 := startTestNode(t, pb, 2)
	cn1.nc.AddPeers([]string{"127.0.0.1:" + strconv.Itoa(pb+2)})
	go cn1.Run()
	go cn2.Run()
	bs := genTestBlocks(n, 1)
	time.Sleep(time.Second * 10)
	for i, b := range bs {
		log.Printf("feed block %d", i+1)
		err := cn1.SubmitBlock(b)
		if err != nil {
			t.Fatal(err)
		}
		time.Sleep(time.Second * 2)
	}
	time.Sleep(time.Second * 5)
	_, cs1, err := cn1.GetHighest()
	if err != nil {
		t.Fatal(err)
	}
	if cs1.Height < n-2 {
		t.Fatal("node1 height too small")
	}
	_, cs2, err := cn2.GetHighest()
	if err != nil {
		t.Fatal(err)
	}
	if cs2.Height < n-2 {
		t.Fatal("node2 height too small")
	}
	cn1.Stop()
	cn2.Stop()
}

func TestCore1(t *testing.T) {
	testTwoNodes(t, 30, 21000)
}

func TestCore2(t *testing.T) {
	testTwoNodes(t, 15, 22000)
}

func testTwoNodesInitSync(t *testing.T, n, n2, pb int, fast bool) {
	cn1 := startTestNode(t, pb, 1)
	cn2 := startTestNode(t, pb, 2)
	go cn1.Run()
	go cn2.Run()
	bs := genTestBlocks(n, 1)
	for i, b := range bs {
		log.Printf("feed block %d", i+1)
		err := cn2.SubmitBlock(b)
		if err != nil {
			t.Fatal(err)
		}
		if i+1 == n2 {
			cn1.nc.AddPeers([]string{"127.0.0.1:" + strconv.Itoa(pb+2)})
			time.Sleep(time.Second * 15)
		}
		if i >= n2 || !fast {
			time.Sleep(time.Second * 2)
		}
	}
	time.Sleep(time.Second * 5)
	_, cs1, err := cn1.GetHighest()
	if err != nil {
		t.Fatal(err)
	}
	if cs1.Height < n-2 {
		t.Fatal("node1 height too small")
	}
	_, cs2, err := cn2.GetHighest()
	if err != nil {
		t.Fatal(err)
	}
	if cs2.Height < n-2 {
		t.Fatal("node2 height too small")
	}
	cn1.Stop()
	cn2.Stop()
}

func TestCore3(t *testing.T) {
	testTwoNodesInitSync(t, 18, 8, 23000, false)
}

func TestCore4(t *testing.T) {
	testTwoNodesInitSync(t, 100, 67, 24000, true)
}

func TestFork(t *testing.T) {
	pb := 25000
	cn1 := startTestNode(t, pb, 1)
	cn2 := startTestNode(t, pb, 2)
	go cn1.Run()
	go cn2.Run()
	n := 17
	bs1 := genTestBlocks(n, 1)
	bs2 := genTestBlocks(7, 100000000000000)
	for i, b := range bs1 {
		log.Printf("feed block %d", i+1)
		err := cn1.SubmitBlock(b)
		if err != nil {
			t.Fatal(err)
		}
	}
	for i, b := range bs2 {
		log.Printf("feed block %d", i+1)
		err := cn2.SubmitBlock(b)
		if err != nil {
			t.Fatal(err)
		}
	}
	cn1.nc.AddPeers([]string{"127.0.0.1:" + strconv.Itoa(pb+2)})
	time.Sleep(time.Second * 100)
	_, cs1, err := cn1.GetHighest()
	if err != nil {
		t.Fatal(err)
	}
	if cs1.Height < n-2 {
		t.Fatal("node1 height too small")
	}
	_, cs2, err := cn2.GetHighest()
	if err != nil {
		t.Fatal(err)
	}
	if cs2.Height < n-2 {
		t.Fatal("node2 height too small")
	}
	cn1.Stop()
	cn2.Stop()
}
