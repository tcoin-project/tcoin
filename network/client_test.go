package network

import (
	"log"
	"strconv"
	"testing"
	"time"
)

func testClientFullmesh(t *testing.T, nClients, mout, min, slp int, topo []int) {
	cs := []*Client{}
	rs := []chan ClientPacket{}
	for i := 0; i < nClients; i++ {
		rs = append(rs, make(chan ClientPacket, 100))
	}
	for i := 0; i < nClients; i++ {
		c, err := NewClient(&ClientConfig{
			PublicIP:               "127.0.0.1",
			Port:                   i + 20000,
			MaxOutgoingConnections: mout,
			MaxIncomingConnections: min,
		}, rs[i], 8888)
		if err != nil {
			t.Fatal(err)
		}
		cs = append(cs, c)
	}
	for i := 0; i < len(topo); i += 2 {
		cs[topo[i]].AddPeers([]string{"127.0.0.1:" + strconv.Itoa(20000+topo[i+1])})
	}
	time.Sleep(time.Second * time.Duration(slp))
	for i := 0; i < nClients; i++ {
		log.Printf("checking %d", i)
		tot, act := cs[i].GetPeerCount()
		if act < mout {
			t.Fatalf("client %d only has %d outgoing peers (%d active)", i, tot, act)
		}
	}
	for i := 0; i < nClients; i++ {
		log.Printf("stopping %d", i)
		cs[i].Stop()
	}
}

func TestClientFullmesh1(t *testing.T) {
	testClientFullmesh(t, 2, 1, 2, 20, []int{0, 1})
}

func TestClientFullmesh2(t *testing.T) {
	testClientFullmesh(t, 3, 2, 3, 40, []int{0, 1, 1, 2})
}

func TestClientFullmesh3(t *testing.T) {
	testClientFullmesh(t, 5, 4, 5, 60, []int{0, 1, 1, 2, 2, 3, 3, 4, 4, 0})
}
