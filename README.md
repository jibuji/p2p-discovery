# P2P Service Discovery

A libp2p-based decentralized service discovery system that allows peers to discover and register services in a P2P network.

## Features

- DHT-based peer discovery
- Service registration and discovery
- Protocol-based service handling
- Asynchronous peer discovery with channels

## Installation

```bash
go get github.com/jibuji/p2p-discovery
```

## Usage

Here's a basic example of how to use the service discovery:

```go
// Create a new libp2p host
host, err := libp2p.New()
if err != nil {
    log.Fatal(err)
}

// Create a new discovery service
ds := discovery.WithNode(host)

// Register a service
err = ds.RegisterService(ctx, protocolID, []discovery.ServiceCradle{
    {Name: serviceName, Creator: func(peer *rpc.RpcPeer) discovery.ServiceHandler {
        return service.NewCalculatorService(peer)
    },
    },
})
if err != nil {
    log.Fatal(err)
}

// Discover peers
peersChan, err := ds.DiscoverPeers(ctx, "protocol1")
if err != nil {
    log.Fatal(err)
}

// Handle discovered peers
for peer := range peersChan {
    fmt.Printf("Discovered peer: %s\n", peer.ID)
}
```

See the `examples` directory for more detailed examples.

## Project Structure

```
.
├── pkg/
│   └── discovery/
│       ├── discovery.go  # Main discovery interface
│       └── dht.go        # DHT-based discovery implementation
├── examples/
├── go.mod
└── README.md
```

## License

MIT License 