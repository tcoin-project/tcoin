package core

import (
	"github.com/mcfx/tcoin/core/block"
	"github.com/mcfx/tcoin/core/consensus"
)

type ChainNodeConfig struct {
	StoragePath          string  `json:"storage_path"`
	StorageFinalizeDepth int     `json:"storage_finalize_depth"`
	StorageDumpDiskRatio float64 `json:"storage_dump_disk_ratio"`
	ListenPort           int     `json:"listen_port"`
	MaxConnections       int     `json:"max_connections"`
}

type ChainGlobalConfig struct {
	ChainId               uint16                    `json:"chain_id"`
	GenesisBlock          *block.Block              `json:"genesis_block"`
	GenesisConsensusState *consensus.ConsensusState `json:"genesis_consensus_state"`
	GenesisBlockReward    uint64                    `json:"genesis_block_reward"`
	BlockReward           uint64                    `json:"block_reward"`
	SeedNodes             []string                  `json:"seed_nodes"`
	Tip1EnableHeight      int                       `json:"tip1_enable_height"`
}
