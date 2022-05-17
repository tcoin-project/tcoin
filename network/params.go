package network

import (
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"net"
	"time"
)

const MaxTimeout = time.Second * 120
const HeartBeatTime = time.Second * 30
const DialTimeout = time.Second * 10
const BanTime = time.Second * 3600

const PktHeartBeat = 1
const PktFindPeer = 2
const PktPeerInfo = 3
const PktChain = 4

const PeerHelloSalt = "Tc01n_1111aa"
const PeerHelloNonceLen = 8

var errNetworkIdMismatch = errors.New("peer network id mismatch")

type ClientConfig struct {
	PublicIP               string `json:"public_ip"`
	Port                   int    `json:"port"`
	MaxOutgoingConnections int    `json:"max_outgoing_connections"`
	MaxIncomingConnections int    `json:"max_incoming_connections"`
}

func connStrId(s string) int {
	t := sha256.Sum256([]byte(s))
	return int(binary.LittleEndian.Uint64(t[:8])&0xfffffffffffffff) + 1
}

func connId(conn net.Conn) int {
	return connStrId(conn.RemoteAddr().String())
}
