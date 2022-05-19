package corerpc

import (
	"bytes"

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
	return s
}

func (s *Server) getBlockCandidate(c *gin.Context) {
	var body struct {
		Miner block.AddressType `json:"miner"`
	}
	c.BindJSON(&body)
	b := s.c.GetBlockCandidate(body.Miner)
	var buf bytes.Buffer
	err := block.EncodeBlock(&buf, b)
	if err != nil {
		c.JSON(200, gin.H{"status": false, "msg": err.Error()})
	} else {
		c.JSON(200, gin.H{"status": true, "block": buf.Bytes()})
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
	c.JSON(200, gin.H{"status": true, "block": buf.Bytes(), "consensus": buf2.Bytes()})
}

func (s *Server) getAccountInfo(c *gin.Context) {
	var body struct {
		Addr string `json:"addr"`
	}
	c.BindJSON(&body)
	addr, err := address.ParseAddr(body.Addr)
	if err != nil {
		c.JSON(200, gin.H{"status": false, "msg": err.Error()})
		return
	}
	ai := s.c.GetAccountInfo(addr)
	c.JSON(200, gin.H{"status": true, "data": ai})
}

func (s *Server) Run(addr string) {
	s.r.Run(addr)
}
