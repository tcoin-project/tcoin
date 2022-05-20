package main

import (
	"encoding/json"
	"flag"
	"io/ioutil"
	"log"

	"github.com/mcfx/tcoin/core"
	"github.com/mcfx/tcoin/utils/corerpc"
)

func main() {
	cfn := flag.String("config", "", "config file")
	gcfn := flag.String("globalConfig", "", "global config file")
	rpcAddr := flag.String("rpc", "", "rpc listen addr")
	flag.Parse()
	if *cfn == "" {
		log.Fatal("no config file provided")
	}
	var c core.ChainNodeConfig
	cf, err := ioutil.ReadFile(*cfn)
	if err != nil {
		log.Fatalf("failed to read config: %v", err)
	}
	json.Unmarshal(cf, &c)
	var gc core.ChainGlobalConfig
	gcf, err := ioutil.ReadFile(*gcfn)
	if err != nil {
		log.Fatalf("failed to read global config: %v", err)
	}
	json.Unmarshal(gcf, &gc)

	n, err := core.NewChainNode(c, gc, nil)
	if err != nil {
		log.Fatalf("failed to set up node: %v", err)
	}
	if *rpcAddr != "" {
		rpc := corerpc.NewServer(n)
		go rpc.Run(*rpcAddr)
	}
	n.Run()
}
