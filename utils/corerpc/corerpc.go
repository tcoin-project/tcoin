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

func (s *Server) Run(addr string) {
	s.r.Run(addr)
}
