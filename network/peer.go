package network

import (
	"bufio"
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

func NewPeer(id int, conn net.Conn, rq chan peerPacket) *Peer {
	//log.Printf("new peer %d", id)
	p := &Peer{
		id:      id,
		conn:    conn,
		r:       bufio.NewReader(conn),
		w:       bufio.NewWriter(conn),
		rq:      rq,
		wq:      make(chan packet, 10),
		stop:    make(chan bool, 20),
		stopped: make(chan bool, 10),
	}
	go p.readFunc()
	go p.writeFunc()
	go p.heartBeat()
	return p
}

func (p *Peer) Stop() {
	//log.Printf("direct stop")
	p.istop()
	for i := 0; i < 4; i++ {
		<-p.stopped
	}
}

func (p *Peer) istop() {
	for i := 0; i < 5; i++ {
		p.stop <- true
	}
	p.stopped <- true
	//log.Printf("stop triggered")
	p.conn.SetDeadline(time.Now())
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

func (p *Peer) writeFunc() {
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
