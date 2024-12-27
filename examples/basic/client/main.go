package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"time"

	"github.com/jibuji/go-stream-rpc/rpc"
	rpcStream "github.com/jibuji/go-stream-rpc/stream/libp2p"
	"github.com/jibuji/p2p-discovery/examples/basic/calculator/proto"
	"github.com/jibuji/p2p-discovery/examples/basic/calculator/proto/service"
	"github.com/jibuji/p2p-discovery/pkg/discovery"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/multiformats/go-multiaddr"
)

const (
	protocolID  = service.CalculatorProtocolID
	serviceName = "Calculator"
)

func main() {
	// Parse command line flags
	peerAddr := flag.String("peer", "", "Multiaddr of peer to connect to (optional)")
	flag.Parse()

	// Create a context that will be canceled on interrupt
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create a new libp2p host
	host, err := libp2p.New()
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
	// Register echo service
	err = ds.RegisterService(protocolID, []discovery.ServiceCradle{
		{Name: serviceName, Creator: func(ctx context.Context, peer *rpc.RpcPeer) discovery.ServiceHandler {
			log.Println("register service for protocol", protocolID)
			go func() {
				time.Sleep(10 * time.Second)
				client := proto.NewCalculatorClient(peer)
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

	// If peer address is provided, connect to it
	if *peerAddr != "" {
		maddr, err := multiaddr.NewMultiaddr(*peerAddr)
		if err != nil {
			log.Printf("Invalid peer multiaddr %s: %v\n", *peerAddr, err)
		} else {
			// Extract the peer ID from the multiaddr
			addrInfo, err := peer.AddrInfoFromP2pAddr(maddr)
			if err != nil {
				log.Printf("Failed to parse peer address info: %v\n", err)
			} else {
				// Connect to the peer
				if err := host.Connect(ctx, *addrInfo); err != nil {
					log.Printf("Failed to connect to peer %s: %v\n", *peerAddr, err)
				} else {
					go connectToPeer(ctx, host, addrInfo)
				}
			}
		}
	}

	host.SetStreamHandler(protocol.ID("test/1.0.0"), func(stream network.Stream) {
		log.Println("New connection on test/1.0.0 from: ", stream.Conn().RemotePeer())
		libp2pStream := rpcStream.NewLibP2PStream(stream)
		peer := rpc.NewRpcPeer(libp2pStream)
		peer.RegisterService("Calculator", service.NewCalculatorService(peer))
	})
	<-ctx.Done()
}

func connectToPeer(ctx context.Context, host host.Host, addrInfo *peer.AddrInfo) {
	log.Printf("Successfully connected to peer %s\n", addrInfo.ID)
	// time.Sleep(10 * time.Second)
	stream, err := host.NewStream(ctx, addrInfo.ID, protocol.ID("whatevs"))
	if err != nil {
		log.Printf("Failed to create stream to peer %s: %v\n", addrInfo.ID, err)
	} else {
		log.Printf("Stream created to peer %s\n", addrInfo.ID)
		rp := rpc.NewRpcPeer(stream)
		rp.RegisterService("Calculator", service.NewCalculatorService(rp))
		resp := proto.NewCalculatorClient(rp).Add(&proto.AddRequest{A: 1, B: 2})
		log.Printf("Add result: %v\n", resp)
		// let's periodically call the peer
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				resp = proto.NewCalculatorClient(rp).Add(&proto.AddRequest{A: 1, B: 2})
				log.Printf("Add result: %v\n", resp)
			case <-ctx.Done():
				return
			case err := <-rp.ErrorChannel():
				log.Println("=====error", err)
				return
			}
		}
	}
}
