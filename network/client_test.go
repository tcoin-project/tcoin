package network

import (
	"log"
	"strconv"
	"testing"
	"time"
)

func testClientFullmesh(t *testing.T, nClients, mconn, req, slp, port int, topo []int) {
	cs := []*Client{}
	rs := []chan ClientPacket{}
	for i := 0; i < nClients; i++ {
		rs = append(rs, make(chan ClientPacket, 100))
	}
	for i := 0; i < nClients; i++ {
		c, err := NewClient(&ClientConfig{
			Port:           i + port,
			MaxConnections: mconn,
		}, rs[i], 8888)
		if err != nil {
			t.Fatal(err)
		}
		cs = append(cs, c)
	}
	for i := 0; i < len(topo); i += 2 {
		cs[topo[i]].AddPeers([]string{"127.0.0.1:" + strconv.Itoa(port+topo[i+1])})
	}
	time.Sleep(time.Second * time.Duration(slp))
	for i := 0; i < nClients; i++ {
		log.Printf("checking %d", i)
		tot, act := cs[i].GetPeerCount()
		if act < req {
			t.Fatalf("client %d only has %d peers (%d active)", i, tot, act)
		}
	}
	for i := 0; i < nClients; i++ {
		log.Printf("stopping %d", i)
		cs[i].Stop()
	}
}

func TestClientFullmesh1(t *testing.T) {
	testClientFullmesh(t, 2, 10, 1, 20, 21000, []int{0, 1})
}

func TestClientFullmesh2(t *testing.T) {
	testClientFullmesh(t, 3, 10, 2, 40, 22000, []int{0, 1, 1, 2})
}

func TestClientFullmesh3(t *testing.T) {
	testClientFullmesh(t, 5, 10, 4, 60, 23000, []int{0, 1, 1, 2, 2, 3, 3, 4, 4, 0})
}

func TestClientFullmesh4(t *testing.T) {
	testClientFullmesh(t, 5, 2, 2, 60, 24000, []int{0, 1, 1, 2, 2, 3, 3, 4})
}

func TestClientFullmesh5(t *testing.T) {
	testClientFullmesh(t, 10, 6, 3, 300, 25000, []int{0, 1, 0, 2, 0, 3, 1, 4, 1, 5, 2, 6, 2, 7, 3, 8, 3, 9})
}
