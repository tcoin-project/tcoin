package core

import (
	"bytes"
	"errors"
	"fmt"

	"github.com/mcfx/tcoin/core/block"
	"github.com/mcfx/tcoin/core/consensus"
	"github.com/mcfx/tcoin/network"
	"github.com/mcfx/tcoin/storage"
)

type ChainNode struct {
	se               *storage.StorageEngine
	unresolvedBlocks map[block.HashType]block.BlockHeader
	blockCache       map[block.HashType]*block.Block
	nc               *network.Client
	rchan            chan network.ClientPacket
	config           TCoinNodeConfig
	gConfig          TCoinGlobalConfig
}

func NewChainNode(config TCoinNodeConfig, gConfig TCoinGlobalConfig) (*ChainNode, error) {
	if gConfig.GenesisBlock.Header.ComputeHash() != gConfig.GenesisBlock.Header.Hash {
		return nil, errors.New("failed to init node: header hash mismatch")
	}
	if gConfig.GenesisBlock.ComputeHash() != gConfig.GenesisBlock.Header.BodyHash {
		return nil, errors.New("failed to init node: header hash mismatch")
	}
	sl := storage.EmptySlice()
	err := block.ExecuteBlock(gConfig.GenesisBlock, gConfig.GenesisBlockReward, sl)
	if err != nil {
		return nil, fmt.Errorf("failed to init node: %v", err)
	}
	cs := gConfig.GenesisConsensusState
	if !cs.CheckAndUpdate(gConfig.GenesisBlock) {
		return nil, errors.New("failed to init node: consensus rejected")
	}
	var buf bytes.Buffer
	err = consensus.EncodeConsensus(&buf, cs)
	if err != nil {
		return nil, fmt.Errorf("failed to init node: %v", err)
	}
	err = block.EncodeBlock(&buf, gConfig.GenesisBlock)
	if err != nil {
		return nil, fmt.Errorf("failed to init node: %v", err)
	}
	se, err := storage.NewStorageEngine(storage.StorageEngineConfig{
		FinalizeDepth: config.StorageFinalizeDepth,
		DumpDiskRatio: config.StorageDumpDiskRatio,
		Path:          config.StoragePath,
	}, sl, storage.SliceKeyType(gConfig.GenesisBlock.Header.Hash), buf.Bytes())
	if err != nil {
		return nil, fmt.Errorf("failed to init node: %v", err)
	}
	rchan := make(chan network.ClientPacket, 10000)
	nc, err := network.NewClient(&network.ClientConfig{
		Port:           config.ListenPort,
		MaxConnections: config.MaxConnections,
	}, rchan, gConfig.ChainId)
	if err != nil {
		return nil, fmt.Errorf("failed to init node: %v", err)
	}
	cn := &ChainNode{
		se:               se,
		unresolvedBlocks: make(map[block.HashType]block.BlockHeader),
		blockCache:       make(map[block.HashType]*block.Block),
		nc:               nc,
		rchan:            rchan,
		config:           config,
		gConfig:          gConfig,
	}
	return cn, nil
}

func (cn *ChainNode) Run() {

}
