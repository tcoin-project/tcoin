package corerpc

import (
	"bytes"
	"encoding/hex"
	"strconv"

	"github.com/mcfx/tcoin/core"
	"github.com/mcfx/tcoin/core/block"
	"github.com/mcfx/tcoin/core/consensus"
	"github.com/mcfx/tcoin/utils/address"

	"github.com/gin-gonic/gin"
)

type Server struct {
	r *gin.Engine
	c *core.ChainNode
}

func NewServer(c *core.ChainNode) *Server {
	s := &Server{
		r: gin.New(),
		c: c,
	}
	s.r.POST("/get_block_candidate", s.getBlockCandidate)
	s.r.POST("/submit_block", s.submitBlock)
	s.r.GET("/get_highest", s.getHighest)
	s.r.POST("/get_account_info", s.getAccountInfo)
	s.r.GET("/get_account_info/:addr", s.getAccountInfo)
	s.r.POST("/submit_tx", s.submitTx)
	s.r.GET("/get_block/:blockid", s.getBlock)
	s.r.GET("/get_storage_at/:addr/:pos", s.getStorageAt)
	s.r.POST("/estimate_gas", s.estimateGas)
	s.r.POST("/run_view_raw_code", s.runViewRawCode)
	s.r.GET("/explorer/get_account_transactions/:addr/:page", s.explorerGetAccountTransactions)
	s.r.GET("/explorer/get_transaction/:txh", s.explorerGetTransaction)
	s.r.GET("/explorer/get_block_by_hash/:hash", s.explorerGetBlockByHash)
	return s
}

func (s *Server) getBlockCandidate(c *gin.Context) {
	var body struct {
		Addr string `json:"addr"`
	}
	c.BindJSON(&body)
	addr, err := address.ParseAddr(body.Addr)
	if err != nil {
		c.JSON(200, gin.H{"status": false, "msg": err.Error()})
		return
	}
	_, cs, err := s.c.GetHighest()
	if err != nil {
		c.JSON(200, gin.H{"status": false, "msg": err.Error()})
		return
	}
	b := s.c.GetBlockCandidate(addr)
	var buf bytes.Buffer
	err = block.EncodeBlock(&buf, b)
	if err != nil {
		c.JSON(200, gin.H{"status": false, "msg": err.Error()})
	} else {
		c.JSON(200, gin.H{"status": true, "block": buf.Bytes(), "difficulty": hex.EncodeToString(cs.Difficulty[:])})
	}
}

func (s *Server) submitBlock(c *gin.Context) {
	var body struct {
		Block []byte `json:"block"`
	}
	c.BindJSON(&body)
	buf := bytes.NewBuffer(body.Block)
	b, err := block.DecodeBlock(buf)
	if err != nil {
		c.JSON(200, gin.H{"status": false, "msg": err.Error()})
		return
	}
	err = s.c.SubmitBlock(b)
	if err != nil {
		c.JSON(200, gin.H{"status": false, "msg": err.Error()})
	} else {
		c.JSON(200, gin.H{"status": true})
	}
}

func (s *Server) getHighest(c *gin.Context) {
	b, cs, err := s.c.GetHighest()
	if err != nil {
		c.JSON(200, gin.H{"status": false, "msg": err.Error()})
		return
	}
	var buf bytes.Buffer
	err = block.EncodeBlock(&buf, b)
	if err != nil {
		c.JSON(200, gin.H{"status": false, "msg": err.Error()})
		return
	}
	var buf2 bytes.Buffer
	err = consensus.EncodeConsensus(&buf2, cs)
	if err != nil {
		c.JSON(200, gin.H{"status": false, "msg": err.Error()})
		return
	}
	c.JSON(200, gin.H{"status": true, "block": buf.Bytes(), "consensus": buf2.Bytes(), "height": cs.Height})
}

func (s *Server) getAccountInfo(c *gin.Context) {
	var body struct {
		Addr string `json:"addr"`
	}
	body.Addr = c.Param("addr")
	if body.Addr == "" {
		c.BindJSON(&body)
	}
	addr, err := address.ParseAddr(body.Addr)
	if err != nil {
		c.JSON(200, gin.H{"status": false, "msg": err.Error()})
		return
	}
	ai := s.c.GetAccountInfo(addr)
	c.JSON(200, gin.H{"status": true, "data": ai})
}

func (s *Server) submitTx(c *gin.Context) {
	var body struct {
		Tx []byte `json:"tx"`
	}
	c.BindJSON(&body)
	buf := bytes.NewBuffer(body.Tx)
	tx, err := block.DecodeTx(buf)
	if err != nil {
		c.JSON(200, gin.H{"status": false, "msg": err.Error()})
		return
	}
	err = s.c.SubmitTx(tx)
	if err != nil {
		c.JSON(200, gin.H{"status": false, "msg": err.Error()})
	} else {
		c.JSON(200, gin.H{"status": true})
	}
}

func (s *Server) getBlock(c *gin.Context) {
	blockIdStr := c.Param("blockid")
	blockId, err := strconv.Atoi(blockIdStr)
	if err != nil {
		c.JSON(200, gin.H{"status": false, "msg": err.Error()})
		return
	}
	b, cs, err := s.c.GetBlock(blockId)
	if err != nil {
		c.JSON(200, gin.H{"status": false, "msg": err.Error()})
		return
	}
	var buf bytes.Buffer
	err = block.EncodeBlock(&buf, b)
	if err != nil {
		c.JSON(200, gin.H{"status": false, "msg": err.Error()})
		return
	}
	var buf2 bytes.Buffer
	err = consensus.EncodeConsensus(&buf2, cs)
	if err != nil {
		c.JSON(200, gin.H{"status": false, "msg": err.Error()})
		return
	}
	c.JSON(200, gin.H{"status": true, "block": buf.Bytes(), "consensus": buf2.Bytes(), "height": cs.Height})
}

func (s *Server) getStorageAt(c *gin.Context) {
	raddr := c.Param("addr")
	rpos := c.Param("pos")
	addr, err := address.ParseAddr(raddr)
	if err != nil {
		c.JSON(200, gin.H{"status": false, "msg": err.Error()})
		return
	}
	pos, err := hex.DecodeString(rpos)
	if err != nil {
		c.JSON(200, gin.H{"status": false, "msg": err.Error()})
		return
	}
	if len(pos) != block.HashLen {
		c.JSON(200, gin.H{"status": false, "msg": "pos length invalid"})
		return
	}
	var posc block.HashType
	copy(posc[:], pos)
	res := s.c.GetStorageAt(addr, posc)
	c.JSON(200, gin.H{"status": true, "data": hex.EncodeToString(res[:])})
}

func (s *Server) estimateGas(c *gin.Context) {
	var body struct {
		Origin string `json:"origin"`
		Code   []byte `json:"code"`
	}
	c.BindJSON(&body)
	addr, err := address.ParseAddr(body.Origin)
	if err != nil {
		c.JSON(200, gin.H{"status": false, "msg": err.Error()})
		return
	}
	if len(body.Code) == 0 || body.Code == nil {
		c.JSON(200, gin.H{"status": false, "msg": "invalid code"})
		return
	}
	useGas, err := s.c.EstimateGas(addr, body.Code)
	var es interface{} = nil
	if err != nil {
		es = err.Error()
	}
	c.JSON(200, gin.H{"status": true, "gas": useGas, "error": es})
}

func (s *Server) runViewRawCode(c *gin.Context) {
	var body struct {
		Origin string `json:"origin"`
		Code   []byte `json:"code"`
	}
	c.BindJSON(&body)
	addr, err := address.ParseAddr(body.Origin)
	if err != nil {
		c.JSON(200, gin.H{"status": false, "msg": err.Error()})
		return
	}
	if len(body.Code) == 0 || body.Code == nil {
		c.JSON(200, gin.H{"status": false, "msg": "invalid code"})
		return
	}
	res, err := s.c.RunViewRawCode(addr, body.Code)
	var es interface{} = nil
	if err != nil {
		es = err.Error()
	}
	c.JSON(200, gin.H{"status": true, "data": res, "error": es})
}

func (s *Server) explorerGetAccountTransactions(c *gin.Context) {
	raddr := c.Param("addr")
	pageStr := c.Param("page")
	page, err := strconv.Atoi(pageStr)
	if err != nil {
		c.JSON(200, gin.H{"status": false, "msg": err.Error()})
		return
	}
	if page <= 0 || page > 10000000 {
		c.JSON(200, gin.H{"status": false, "msg": "page too large"})
		return
	}
	addr, err := address.ParseAddr(raddr)
	if err != nil {
		c.JSON(200, gin.H{"status": false, "msg": err.Error()})
		return
	}
	res, n := s.c.ExplorerGetAccountTransactions(addr, page*100-99, page*100)
	c.JSON(200, gin.H{"status": true, "txs": res, "total": n})
}

func (s *Server) explorerGetTransaction(c *gin.Context) {
	txhStr := c.Param("txh")
	txht, err := hex.DecodeString(txhStr)
	if err != nil {
		c.JSON(200, gin.H{"status": false, "msg": err.Error()})
		return
	}
	if len(txht) != block.HashLen {
		c.JSON(200, gin.H{"status": false, "msg": "hash length invalid"})
		return
	}
	var txh block.HashType
	copy(txh[:], txht)
	tx, height, err := s.c.ExplorerGetTransaction(txh)
	if err != nil {
		c.JSON(200, gin.H{"status": false, "msg": err.Error()})
		return
	}
	var buf bytes.Buffer
	err = block.EncodeTx(&buf, tx)
	if err != nil {
		c.JSON(200, gin.H{"status": false, "msg": err.Error()})
		return
	}
	c.JSON(200, gin.H{"status": true, "tx": buf.Bytes(), "height": height})
}

func (s *Server) explorerGetBlockByHash(c *gin.Context) {
	hashStr := c.Param("hash")
	hasht, err := hex.DecodeString(hashStr)
	if err != nil {
		c.JSON(200, gin.H{"status": false, "msg": err.Error()})
		return
	}
	if len(hasht) != block.HashLen {
		c.JSON(200, gin.H{"status": false, "msg": "hash length invalid"})
		return
	}
	var hash block.HashType
	copy(hash[:], hasht)
	b, cs, err := s.c.ExplorerGetBlockByHash(hash)
	if err != nil {
		c.JSON(200, gin.H{"status": false, "msg": err.Error()})
		return
	}
	var buf bytes.Buffer
	err = block.EncodeBlock(&buf, b)
	if err != nil {
		c.JSON(200, gin.H{"status": false, "msg": err.Error()})
		return
	}
	var buf2 bytes.Buffer
	err = consensus.EncodeConsensus(&buf2, cs)
	if err != nil {
		c.JSON(200, gin.H{"status": false, "msg": err.Error()})
		return
	}
	c.JSON(200, gin.H{"status": true, "block": buf.Bytes(), "consensus": buf2.Bytes(), "height": cs.Height})
}

func (s *Server) Run(addr string) {
	s.r.Run(addr)
}
