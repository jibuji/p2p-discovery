package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"log"
	"time"

	"github.com/jibuji/go-stream-rpc/rpc"
	"github.com/jibuji/go-stream-rpc/session"
	rpcStream "github.com/jibuji/go-stream-rpc/stream/libp2p"
	"github.com/jibuji/p2p-discovery/examples/basic/calculator/proto"
	"github.com/jibuji/p2p-discovery/examples/basic/calculator/proto/service"
	"github.com/jibuji/p2p-discovery/pkg/discovery"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/network"
)

const (
	protocolID  = service.CalculatorProtocolID
	serviceName = "Calculator"
)

func main() {
	// Create a context that will be canceled on interrupt
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Load or create private key before creating the host
	priv, err := loadOrCreatePrivateKey("node.key")
	if err != nil {
		log.Fatal(err)
	}
	// Create a new libp2p host
	host, err := libp2p.New(
		libp2p.Identity(priv),
		libp2p.ListenAddrStrings("/ip4/0.0.0.0/tcp/61212"),
	)
	if err != nil {
		log.Fatal(err)
	}

	// Log our address
	hostAddr := host.Addrs()[0]
	fullAddr := fmt.Sprintf("%s/p2p/%s", hostAddr.String(), host.ID().String())
	log.Printf("Host address: %s\n", fullAddr)

	log.Println("start discovery")
	// Create a new discovery service
	ds := discovery.WithNode(host)

	log.Println("start register service")
	host.SetStreamHandler(protocolID, handleStream)
	// Register echo service
	err = ds.RegisterService(protocolID, []discovery.ServiceCradle{
		{Name: serviceName, Creator: func(peer *rpc.RpcPeer) discovery.ServiceHandler {
			log.Println("register service for protocol", protocolID)
			log.Println("peer", peer)
			go func() {
				time.Sleep(10 * time.Second)
				log.Println("begin request peer", peer)
				client := proto.NewCalculatorClient(peer)
				log.Println("client", client)
				resp := client.Multiply(&proto.MultiplyRequest{A: 1, B: 2})
				// client.Add(&proto.AddRequest{A: 1, B: 2})
				log.Println("Multiply result:", resp)
			}()
			return service.NewCalculatorService(peer)
		},
		},
	})
	if err != nil {
		log.Fatal(err)
	}

	log.Println("start discover peers")
	go func() {
		// Discover peers
		peerChan, err := ds.StartDiscoverPeers(protocolID)
		if err != nil {
			log.Fatal(err)
		}

		// Handle discovered peers
		for peer := range peerChan {
			fmt.Printf("=====Discovered peer: %s\n", peer.ID)
		}

		log.Println("=====discover peers done")
	}()

	<-ctx.Done()
}

func handleStream(s network.Stream) {
	log.Printf("New connection from: %s\n", s.Conn().RemotePeer())

	libp2pStream := rpcStream.NewLibP2PStream(s)

	// Create custom session if needed
	customSession := session.NewMemSession()

	// Create a new RPC peer with custom session
	peer := rpc.NewRpcPeer(libp2pStream, rpc.WithSession(customSession))
	defer peer.Close()
	peer.RegisterService("Calculator", service.NewCalculatorService(peer))
	// Register the calculator service
	// proto.RegisterCalculatorServer(peer, &calculator.CalculatorService{})
	err := peer.Wait()
	if err != nil {
		log.Println(err)
	}
}

func loadOrCreatePrivateKey(path string) (crypto.PrivKey, error) {
	// Try to load existing key
	if keyData, err := ioutil.ReadFile(path); err == nil {
		decoded, err := hex.DecodeString(string(keyData))
		if err != nil {
			return nil, err
		}
		return crypto.UnmarshalPrivateKey(decoded)
	}

	// Generate new key if none exists
	priv, _, err := crypto.GenerateKeyPairWithReader(crypto.Ed25519, -1, rand.Reader)
	if err != nil {
		return nil, err
	}

	// Save the new key
	keyBytes, err := crypto.MarshalPrivateKey(priv)
	if err != nil {
		return nil, err
	}
	err = ioutil.WriteFile(path, []byte(hex.EncodeToString(keyBytes)), 0600)
	if err != nil {
		return nil, err
	}

	return priv, nil
}
