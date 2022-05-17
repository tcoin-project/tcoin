package network

import (
	"bufio"
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"io"
	"net"
	"time"
)

type peerPacket struct {
	id  int
	pkt packet
}

type Peer struct {
	id      int
	conn    net.Conn
	r       *bufio.Reader
	w       *bufio.Writer
	rq      chan peerPacket
	wq      chan packet
	stop    chan bool
	stopped chan bool
}

func NewPeer(id int, conn net.Conn, rq chan peerPacket, networkId uint16, cnonce []byte) (*Peer, error) {
	//log.Printf("new peer %d", id)
	p := &Peer{
		id:      id,
		conn:    conn,
		r:       bufio.NewReader(conn),
		w:       bufio.NewWriter(conn),
		rq:      rq,
		wq:      make(chan packet, 100),
		stop:    make(chan bool, 30),
		stopped: make(chan bool, 10),
	}
	buf := make([]byte, PeerHelloNonceLen)
	_, err := rand.Read(buf)
	if err != nil {
		return nil, err
	}
	buf = append(buf, []byte(PeerHelloSalt)...)
	buf = append(buf, byte(networkId), byte(networkId>>8))
	hs := sha256.Sum256(buf)
	buf2 := make([]byte, PeerHelloNonceLen+8)
	copy(buf2[:PeerHelloNonceLen], buf[:PeerHelloNonceLen])
	copy(buf2[PeerHelloNonceLen:], hs[:8])
	buf2 = append(buf2, cnonce...)
	p.conn.SetDeadline(time.Now().Add(MaxTimeout))
	_, err = p.w.Write(buf2)
	if err != nil {
		return nil, err
	}
	err = p.w.Flush()
	if err != nil {
		return nil, err
	}
	_, err = io.ReadFull(p.r, buf2)
	if err != nil {
		return nil, err
	}
	copy(buf[:PeerHelloNonceLen], buf2[:PeerHelloNonceLen])
	hs = sha256.Sum256(buf)
	if !bytes.Equal(buf2[PeerHelloNonceLen:PeerHelloNonceLen+8], hs[:8]) {
		return nil, errNetworkIdMismatch
	}
	if bytes.Equal(buf2[PeerHelloNonceLen+8:], cnonce) {
		return nil, errSelf
	}
	go p.readFunc()
	go p.writeLoop()
	go p.heartBeat()
	return p, nil
}

func (p *Peer) Stop() {
	//log.Printf("direct stop")
	p.istop()
	for i := 0; i < 4; i++ {
		<-p.stopped
	}
	p.stopped <- true
}

func (p *Peer) Stopped() bool {
	select {
	case <-p.stopped:
		p.stopped <- true
		return true
	default:
		return false
	}
}

func (p *Peer) istop() {
	for i := 0; i < 5; i++ {
		p.stop <- true
	}
	p.stopped <- true
	//log.Printf("stop triggered")
	p.conn.SetDeadline(time.Now())
	p.conn.Close()
}

func (p *Peer) readFunc() {
	defer p.istop()
	for {
		p.conn.SetDeadline(time.Now().Add(MaxTimeout))
		pk, err := decodePacket(p.r)
		if err != nil {
			//log.Printf("read error: %v", err)
			return
		}
		select {
		case <-p.stop:
			return
		default:
		}
		p.rq <- peerPacket{
			id:  p.id,
			pkt: pk,
		}
	}
}

func (p *Peer) writeLoop() {
	defer p.istop()
	for {
		select {
		case pk := <-p.wq:
			err := encodePacket(p.w, pk)
			//log.Printf("wrote packet: %v", pk)
			if err != nil {
				//log.Printf("write error: %v", err)
				return
			}
			p.w.Flush()
		case <-p.stop:
			return
		}
	}
}

func (p *Peer) heartBeat() {
	defer p.istop()
	empty := []byte{}
	for {
		p.wq <- packet{tp: PktHeartBeat, data: empty}
		slp := time.After(HeartBeatTime)
		select {
		case <-p.stop:
			return
		case <-slp:
		}
	}
}
