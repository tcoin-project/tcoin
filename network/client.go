package network

import (
	crand "crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/libp2p/go-reuseport"
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
	peerBanTime map[string]time.Time
	allPeers    []int
	allPeerCons []string
	sendPeers   []byte
	stop        chan bool
	stopped     chan bool
	peersMut    sync.Mutex
	networkId   uint16
	nonce       []byte
}

func NewClient(config *ClientConfig, ccp chan ClientPacket, networkId uint16) (*Client, error) {
	c := &Client{
		config:      config,
		peers:       make(map[int]*Peer),
		peerCon:     make(map[int]string),
		ccp:         ccp,
		cpp:         make(chan peerPacket, 500),
		peerInfo:    make(map[string]time.Time),
		peerBanTime: make(map[string]time.Time),
		allPeers:    []int{},
		allPeerCons: []string{},
		sendPeers:   []byte("{}"),
		stop:        make(chan bool, 50),
		stopped:     make(chan bool, 10),
		networkId:   networkId,
		nonce:       make([]byte, 8),
	}
	_, err := crand.Read(c.nonce)
	if err != nil {
		return nil, fmt.Errorf("failed to set up client nonce: %v", err)
	}
	if config.Path != "" {
		err = os.MkdirAll(filepath.Join(config.Path, "net"), 0o755)
		if err != nil {
			return nil, fmt.Errorf("error when creating network client: %v", err)
		}
	}
	c.ln, err = reuseport.Listen("tcp", ":"+strconv.Itoa(c.config.Port))
	if err != nil {
		return nil, fmt.Errorf("failed to listen port %d: %v", c.config.Port, err)
	}
	go c.listen()
	for i := 0; i < 4; i++ {
		go c.readLoop()
	}
	go c.maintainSendPeers()
	go c.maintainPeers()
	go c.maintainConns()
	go c.broadcastFindPeer()
	if config.Path != "" {
		b, err := ioutil.ReadFile(filepath.Join(c.config.Path, "net", "peers.json"))
		if err == nil {
			var s []string
			if err := json.Unmarshal(b, &s); err == nil {
				c.AddPeers(s)
			}
		}
	}
	return c, nil
}

func (c *Client) Stop() {
	c.istop()
	for i := 0; i < 10; i++ {
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
		if v != nil {
			v.Stop()
		}
	}
	c.peers = make(map[int]*Peer)
	c.peerCon = make(map[int]string)
	c.peersMut.Unlock()
}

func (c *Client) countPeers() int {
	return len(c.peers)
}

func (c *Client) handleConn(id int, conn net.Conn) {
	c.peers[id] = nil
	go func() {
		p, err := NewPeer(id, conn, c.cpp, c.networkId, c.nonce)
		if err == nil {
			rm := conn.RemoteAddr().String()
			c.peersMut.Lock()
			if p2, ok := c.peers[id]; !ok || p2 == nil {
				//log.Printf("conn: %s - %s", conn.LocalAddr().String(), conn.RemoteAddr().String())
				c.peers[id] = p
				c.peerCon[id] = rm
			}
			c.peersMut.Unlock()
		} else if errors.Is(err, errNetworkIdMismatch) || errors.Is(err, errSelf) {
			c.DiscardPeer(id, time.Hour*100000)
		} else {
			c.DiscardPeer(id, time.Minute*2)
		}
	}()
}

func (c *Client) listen() {
	defer c.istop()
	for {
		conn, err := c.ln.Accept()
		if err != nil {
			return
		}
		id := connId(conn)
		c.peersMut.Lock()
		if _, ok := c.peers[id]; !ok {
			if c.countPeers() < c.config.MaxConnections {
				c.handleConn(id, conn)
			} else {
				go conn.Close()
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
		//log.Printf("%d got packet: %d %d %s", c.config.Port, pp.id, pp.pkt.tp, pp.pkt.data)
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
			c.DiscardPeer(pp.id, time.Minute*2)
		}
	}
}

func (c *Client) maintainSendPeers() {
	defer c.istop()
	for {
		ts := time.Now()
		tk := ts.Add(time.Second * -600)
		tk2 := ts.Add(time.Second * -7200)
		res := make(map[string]bool)
		c.peersMut.Lock()
		for _, k := range c.peerCon {
			res[k] = true
		}
		rk := []string{}
		for k, v := range c.peerInfo {
			if v.After(tk) {
				res[k] = true
			} else if v.Before(tk2) {
				rk = append(rk, k)
			}
		}
		for _, k := range rk {
			delete(c.peerInfo, k)
			go c.DiscardPeer(connStrId(k), time.Duration(0))
		}
		c.peersMut.Unlock()
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
		if c.config.Path != "" {
			b, err := json.Marshal(t)
			if err == nil {
				ioutil.WriteFile(filepath.Join(c.config.Path, "net", "peers.json"), b, 0o755)
			}
		}
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
	peer, ok := c.peers[id]
	c.peersMut.Unlock()
	if ok && peer != nil {
		peer.wq <- pkt
	}
}

func (c *Client) broadcast(pkt packet, count int) {
	o := make([]int, count)
	bpeers := make([]*Peer, count)
	bpeers = bpeers[:0]
	c.peersMut.Lock()
	le := len(c.allPeers)
	for i := 0; i < count && i < le; i++ {
		var x int
		for {
			x = rand.Intn(le)
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
		o[i] = x
		id := c.allPeers[x]
		if peer, ok := c.peers[id]; ok && peer != nil {
			bpeers = append(bpeers, peer)
		}
	}
	c.peersMut.Unlock()
	for _, peer := range bpeers {
		peer.wq <- pkt
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

func (c *Client) tryConn(id int, host string) {
	la, _ := net.ResolveTCPAddr("tcp", ":"+strconv.Itoa(c.config.Port))
	d := net.Dialer{
		Timeout:   DialTimeout,
		Control:   reuseport.Control,
		LocalAddr: la,
	}
	conn, err := d.Dial("tcp", host)
	c.peersMut.Lock()
	if err == nil {
		if p, ok := c.peers[id]; !ok || p == nil {
			c.handleConn(id, conn)
			return
		}
	}
	c.peersMut.Unlock()
	c.DiscardPeer(id, time.Duration(0))
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
		if c.countPeers() < c.config.MaxConnections {
			for i := 0; i < 3 && len(c.allPeerCons) > 0; i++ {
				x := rand.Intn(len(c.allPeerCons))
				px := c.allPeerCons[x]
				id := connStrId(px)
				if _, ok := c.peers[id]; !ok {
					c.peers[id] = nil
					go c.tryConn(id, px)
				}
			}
		}
		c.peersMut.Unlock()
	}
}

func (c *Client) maintainConns() {
	defer c.istop()
	for {
		slp := time.After(time.Second * 5)
		select {
		case <-slp:
		case <-c.stop:
			return
		}
		q := []int{}
		c.peersMut.Lock()
		for id, p := range c.peers {
			if p != nil && p.Stopped() {
				q = append(q, id)
			}
		}
		c.peersMut.Unlock()
		for _, id := range q {
			go c.DiscardPeer(id, time.Duration(0))
		}
	}
}

func (c *Client) DiscardPeer(id int, banTime time.Duration) {
	c.peersMut.Lock()
	con, ok := c.peerCon[id]
	delete(c.peerCon, id)
	if ok {
		c.peerBanTime[con] = time.Now().Add(banTime)
	}
	peer, ok := c.peers[id]
	delete(c.peers, id)
	c.peersMut.Unlock()
	if ok && peer != nil {
		peer.Stop()
	}
}

func (c *Client) AddPeers(peers []string) {
	c.peersMut.Lock()
	for _, ps := range peers {
		if len(ps) < 100 {
			_, ok := c.peerInfo[ps]
			if !ok {
				c.peerInfo[ps] = time.Now().Add(time.Second * -300)
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

func (c *Client) GetPeerCount() (int, int) {
	c.peersMut.Lock()
	defer c.peersMut.Unlock()
	act := 0
	for _, p := range c.peers {
		if p != nil {
			act++
		}
	}
	return len(c.peers), act
}
