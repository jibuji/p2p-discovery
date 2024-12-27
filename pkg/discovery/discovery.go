package discovery

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/jibuji/go-stream-rpc/rpc"
	"github.com/jibuji/go-stream-rpc/session"
	rpcStream "github.com/jibuji/go-stream-rpc/stream/libp2p"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p/core/discovery"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/libp2p/go-libp2p/p2p/discovery/routing"
)

type ServiceHandler interface{}

// ServiceCreator is a factory for creating services
type ServiceCreator func(peer *rpc.RpcPeer) ServiceHandler

// Discovery represents the main discovery service
type Discovery struct {
	host   host.Host
	kdht   *dht.IpfsDHT
	ctx    context.Context
	cancel context.CancelFunc

	mu               sync.RWMutex // Protects protocolRegistry
	protocolRegistry map[string]bool
}

// WithNode creates a new Discovery instance with the given libp2p host
func WithNode(node host.Host) *Discovery {
	// Create a context that will be canceled on interrupt
	ctx, cancel := context.WithCancel(context.Background())

	// Create a new DHT client with bootstrap nodes
	kademliaDHT, err := dht.New(ctx, node,
		dht.Mode(dht.ModeServer),
		// dht.BootstrapPeers(
		// 	dht.GetDefaultBootstrapPeerAddrInfos()...,
		// ),
	)
	if err != nil {
		log.Fatal(err)
	}

	// Bootstrap the DHT
	if err := kademliaDHT.Bootstrap(ctx); err != nil {
		log.Fatal(err)
	}

	d := &Discovery{
		host:             node,
		kdht:             kademliaDHT,
		ctx:              ctx,
		cancel:           cancel,
		protocolRegistry: make(map[string]bool),
	}
	d.startAnnounceProtocol()
	node.Network().Notify(&network.NotifyBundle{
		ConnectedF: func(net network.Network, conn network.Conn) {
			go func() {
				// try 3 times when failed, interval 10 second
				for i := 0; i < 30; i++ {
					err := d.advertiseProtocols()
					if err == nil {
						log.Println("=====Advertise protocols success")
						break
					}
					log.Printf("=====Failed to advertise protocols, try %d times: %v", i, err)
					time.Sleep(10 * time.Second)
				}
			}()
		},
	})
	return d
}

// DiscoverPeers discovers peers that provide the specified protocol
func (d *Discovery) StartDiscoverPeers(protocol string) (<-chan peer.AddrInfo, error) {
	routingDiscovery := routing.NewRoutingDiscovery(d.kdht)
	peerChan := make(chan peer.AddrInfo)
	interval := 20 * time.Second
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		defer close(peerChan)
		selfID := d.host.ID()

		// Function to handle one discovery cycle with timeout
		discoverWithTimeout := func() {
			// Create a context with timeout for this discovery cycle
			cycleCtx, cancel := context.WithTimeout(d.ctx, 9*interval/10)
			defer cancel()

			// Keep track of discovered peers to avoid duplicates within a cycle
			discoveredPeers := make(map[peer.ID]peer.AddrInfo)
			// Start peer discovery
			peers, err := routingDiscovery.FindPeers(cycleCtx, protocol, discovery.Limit(1000))
			if err != nil {
				log.Printf("Failed to find peers for protocol %s: %v\n", protocol, err)
				return
			}

			// Use a separate goroutine to handle peer channel processing
			done := make(chan struct{})
			go func() {
				defer close(done)
				for p := range peers {
					if p.ID == selfID { // Don't add self
						continue
					}

					// Skip if we've already discovered this peer in this cycle
					if _, exists := discoveredPeers[p.ID]; exists {
						continue
					}

					discoveredPeers[p.ID] = p

					select {
					case <-cycleCtx.Done():
						return
					case peerChan <- p:
						log.Printf("Discovered new peer %s for protocol %s\n", p.ID, protocol)
					}
				}
			}()

			// Wait for either completion or timeout
			select {
			case <-cycleCtx.Done():
				if cycleCtx.Err() == context.DeadlineExceeded {
					log.Printf("Peer discovery cycle timed out for protocol %s after finding %d peers\n",
						protocol, len(discoveredPeers))
				}
			case <-done:
				log.Printf("Completed peer discovery cycle for protocol %s, found %d peers\n",
					protocol, len(discoveredPeers))
			}
		}

		// Initial discovery
		discoverWithTimeout()

		// Periodic discovery
		for {
			select {
			case <-d.ctx.Done():
				return
			case <-ticker.C:
				discoverWithTimeout()
			}
		}
	}()

	return peerChan, nil
}

type ServiceCradle struct {
	Name    string
	Creator ServiceCreator
}

// RegisterService registers a new service with its handler
func (d *Discovery) RegisterService(ctx context.Context, protocolName string, serviceCradles []ServiceCradle) error {
	d.mu.Lock()
	if _, ok := d.protocolRegistry[protocolName]; ok {
		d.mu.Unlock()
		return fmt.Errorf("protocolName %s already exists", protocolName)
	}
	d.protocolRegistry[protocolName] = true
	d.mu.Unlock()

	d.host.SetStreamHandler(protocol.ID(protocolName), func(stream network.Stream) {
		libp2pStream := rpcStream.NewLibP2PStream(stream)

		// Create custom session if needed
		customSession := session.NewMemSession()

		// Create a new RPC peer with custom session
		peer := rpc.NewRpcPeer(libp2pStream, rpc.WithSession(customSession))
		defer peer.Close()
		for _, serviceCradle := range serviceCradles {
			serviceHandler := serviceCradle.Creator(peer)
			peer.RegisterService(serviceCradle.Name, serviceHandler)
		}
		err := peer.Wait()
		if err != nil {
			log.Println("=====wait error", err)
		}
	})

	routingDiscovery := routing.NewRoutingDiscovery(d.kdht)
	routingDiscovery.Advertise(ctx, protocolName)
	return nil
}

func (d *Discovery) advertiseProtocols() error {
	d.mu.RLock()
	protocols := make([]string, 0, len(d.protocolRegistry))
	for protocolName := range d.protocolRegistry {
		protocols = append(protocols, protocolName)
	}
	d.mu.RUnlock()

	routingDiscovery := routing.NewRoutingDiscovery(d.kdht)
	for _, protocolName := range protocols {
		_, err := routingDiscovery.Advertise(d.ctx, protocolName)
		if err != nil {
			log.Printf("Failed to advertise protocol %s: %v\n", protocolName, err)
			return err
		}
	}
	return nil
}

func (d *Discovery) startAnnounceProtocol() error {
	// Start a goroutine for periodic advertisement
	go func() {
		ticker := time.NewTicker(time.Minute * 10) // Re-advertise every 10 minutes
		defer ticker.Stop()

		for {
			select {
			case <-d.ctx.Done():
				return
			case <-ticker.C:
				d.advertiseProtocols()
			}
		}
	}()

	return nil
}
