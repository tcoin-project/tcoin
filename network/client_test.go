package network

import (
	"strconv"
	"testing"
	"time"
)

func testClient(t *testing.T, nClients, mout, min, slp int, topo []int) {
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
		}, rs[i])
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
		cs[i].Stop()
	}
}

func TestClient1(t *testing.T) {
	testClient(t, 2, 2, 2, 60, []int{0, 1})
}

func TestClient2(t *testing.T) {
	testClient(t, 5, 5, 3, 60, []int{0, 1, 1, 2, 2, 3, 3, 4})
}
