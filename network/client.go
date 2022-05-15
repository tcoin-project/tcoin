package network

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"net"
	"strconv"
	"sync"
	"time"
)

type ClientPacket struct {
	PeerId int
	Data   []byte
}

type Client struct {
	config      *ClientConfig
	ln          net.Listener
	peers       map[int]*Peer
	peerCon     map[int]string
	ccp         chan ClientPacket
	cpp         chan peerPacket
	peerInfo    map[string]time.Time
	allPeers    []int
	allPeerCons []string
	sendPeers   []byte
	stop        chan bool
	stopped     chan bool
	peersMut    sync.Mutex
}

func NewClient(config *ClientConfig, ccp chan ClientPacket) (*Client, error) {
	c := &Client{
		config:      config,
		peers:       make(map[int]*Peer),
		peerCon:     make(map[int]string),
		ccp:         ccp,
		cpp:         make(chan peerPacket, 500),
		peerInfo:    make(map[string]time.Time),
		allPeers:    []int{},
		allPeerCons: []string{},
		sendPeers:   []byte("{}"),
		stop:        make(chan bool, 50),
		stopped:     make(chan bool, 10),
	}
	var err error
	c.ln, err = net.Listen("tcp", ":"+strconv.Itoa(c.config.Port))
	if err != nil {
		return nil, fmt.Errorf("failed to listen port %d: %v", c.config.Port, err)
	}
	go c.listen()
	go c.readLoop()
	go c.maintainSendPeers()
	go c.maintainPeers()
	go c.broadcastFindPeer()
	return c, nil
}

func (c *Client) Stop() {
	c.istop()
	for i := 0; i < 6; i++ {
		<-c.stopped
	}
}

func (c *Client) istop() {
	for i := 0; i < 5; i++ {
		c.stop <- true
	}
	c.ln.Close()
	c.stopped <- true
	c.peersMut.Lock()
	for _, v := range c.peers {
		v.Stop()
	}
	c.peers = make(map[int]*Peer)
	c.peerCon = make(map[int]string)
	c.peersMut.Unlock()
}

func (c *Client) countPeers(outgoing bool) int {
	cnt := 0
	for id := range c.peers {
		if (id > 0) == outgoing {
			cnt++
		}
	}
	return cnt
}

func (c *Client) listen() {
	defer c.istop()
	for {
		conn, err := c.ln.Accept()
		if err != nil {
			return
		}
		id := -connId(conn)
		c.peersMut.Lock()
		if _, ok := c.peers[id]; !ok {
			if c.countPeers(false) < c.config.MaxIncomingConnections {
				c.peers[id] = NewPeer(id, conn, c.cpp)
			} else {
				conn.Close()
			}
		}
		c.peersMut.Unlock()
	}
}

func (c *Client) readLoop() {
	defer c.istop()
	for {
		var pp peerPacket
		select {
		case pp = <-c.cpp:
		case <-c.stop:
			return
		}
		//log.Printf("got packet: %d %d %s", pp.id, pp.pkt.tp, pp.pkt.data)
		c.peersMut.Lock()
		if si, ok := c.peerCon[pp.id]; ok {
			c.peerInfo[si] = time.Now()
		}
		c.peersMut.Unlock()
		err := func() error {
			if pp.pkt.tp == PktHeartBeat {
			} else if pp.pkt.tp == PktFindPeer {
				c.peersMut.Lock()
				k := c.sendPeers
				c.peersMut.Unlock()
				c.writeTo(pp.id, packet{
					tp:   PktPeerInfo,
					data: k,
				})
			} else if pp.pkt.tp == PktPeerInfo {
				tmp := make([]string, 0)
				err := json.Unmarshal(pp.pkt.data, &tmp)
				if err != nil {
					return err
				}
				c.AddPeers(tmp)
			} else if pp.pkt.tp == PktChain {
				c.ccp <- ClientPacket{
					PeerId: pp.id,
					Data:   pp.pkt.data,
				}
			} else {
				return errors.New("unknown packet type")
			}
			return nil
		}()
		if err != nil {
			c.DiscardPeer(pp.id)
		}
	}
}

func (c *Client) maintainSendPeers() {
	defer c.istop()
	for {
		ts := time.Now()
		tk := ts.Add(time.Second * -600)
		res := make(map[string]bool)
		c.peersMut.Lock()
		for _, k := range c.peerCon {
			res[k] = true
		}
		rk := []string{}
		for k, v := range c.peerInfo {
			if v.After(tk) {
				res[k] = true
			} else {
				rk = append(rk, k)
			}
		}
		for _, k := range rk {
			delete(c.peerInfo, k)
		}
		c.peersMut.Unlock()
		if c.config.PublicIP != "" {
			res[c.config.PublicIP+":"+strconv.Itoa(c.config.Port)] = true
		}
		t := make([]string, len(res))
		cnt := 0
		for k := range res {
			t[cnt] = k
			cnt++
		}
		for i := 0; i < 20 && i < len(t); i++ {
			j := rand.Intn(len(t)-i) + i
			if j > i {
				t[i], t[j] = t[j], t[i]
			}
		}
		tu := t
		if len(t) > 20 {
			tu = t[:20]
		}
		c.peersMut.Lock()
		ti := make([]int, len(c.peers))
		cnt = 0
		for k := range c.peers {
			ti[cnt] = k
			cnt++
		}
		c.allPeers = ti
		c.allPeerCons = t
		c.sendPeers, _ = json.Marshal(tu)
		c.peersMut.Unlock()
		te := time.Now()
		slp := time.After(te.Sub(ts)*10 + time.Second)
		select {
		case <-slp:
		case <-c.stop:
			return
		}
	}
}

func (c *Client) writeTo(id int, pkt packet) {
	c.peersMut.Lock()
	defer c.peersMut.Unlock()
	if peer, ok := c.peers[id]; ok {
		peer.wq <- pkt
	}
}

func (c *Client) broadcast(pkt packet, count int) {
	c.peersMut.Lock()
	defer c.peersMut.Unlock()
	o := make([]int, count)
	for i := 0; i < count && i < len(c.allPeers); i++ {
		var x int
		for {
			x = rand.Intn(len(c.allPeers))
			flag := true
			for j := 0; j < i; j++ {
				if o[j] == x {
					flag = false
				}
			}
			if flag {
				break
			}
		}
		id := c.allPeers[x]
		if peer, ok := c.peers[id]; ok {
			peer.wq <- pkt
		}
	}
}

func (c *Client) WriteTo(id int, data []byte) {
	nd := make([]byte, len(data))
	copy(nd, data)
	c.writeTo(id, packet{
		tp:   PktChain,
		data: nd,
	})
}

func (c *Client) Broadcast(data []byte, count int) {
	nd := make([]byte, len(data))
	copy(nd, data)
	c.broadcast(packet{
		tp:   PktChain,
		data: nd,
	}, count)
}

func (c *Client) maintainPeers() {
	defer c.istop()
	for {
		slp := time.After(time.Second * 5)
		select {
		case <-slp:
		case <-c.stop:
			return
		}
		c.peersMut.Lock()
		if c.countPeers(true) < c.config.MaxOutgoingConnections {
			for i := 0; i < 3; i++ {
				x := rand.Intn(len(c.allPeerCons))
				px := c.allPeerCons[x]
				if px == c.config.PublicIP+":"+strconv.Itoa(c.config.Port) {
					continue
				}
				id := connStrId(px)
				if _, ok := c.peers[id]; !ok {
					c.peersMut.Unlock()
					d := net.Dialer{Timeout: DialTimeout}
					conn, err := d.Dial("tcp", px)
					c.peersMut.Lock()
					if err == nil {
						c.peers[id] = NewPeer(id, conn, c.cpp)
					}
				}
			}
		}
		c.peersMut.Unlock()
	}
}

func (c *Client) DiscardPeer(id int) {
	c.peersMut.Lock()
	delete(c.peerCon, id)
	peer, ok := c.peers[id]
	delete(c.peers, id)
	c.peersMut.Unlock()
	if ok {
		peer.Stop()
	}
}

func (c *Client) AddPeers(peers []string) {
	c.peersMut.Lock()
	for _, ps := range peers {
		if len(ps) < 100 {
			_, ok := c.peerInfo[ps]
			if !ok {
				c.peerInfo[ps] = time.Now().Add(time.Second * 300)
			}
		}
	}
	c.peersMut.Unlock()
}

func (c *Client) broadcastFindPeer() {
	defer c.istop()
	empty := []byte{}
	for {
		c.broadcast(packet{
			tp:   PktFindPeer,
			data: empty,
		}, 3)
		slp := time.After(time.Second * 10)
		select {
		case <-c.stop:
			return
		case <-slp:
		}
	}
}

func (c *Client) GetAllPeerCons() []string {
	c.peersMut.Lock()
	defer c.peersMut.Unlock()
	return c.allPeerCons
}
