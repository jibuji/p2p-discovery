# TODO:

the discovery service has two main purposes:

1. discover peers
2. register services

## 1. Discover peers

periodically poll the DHT for peers that have the service we are looking for, then the upper layer can connect and request additional information

## 2. Register services

register a service protocol and service handlers
can listen multiple protocols, each protocol can have multiple handlers

## 3. expected api examples

```go
// cteate discovery node
dn := discovery.WithNode(node)

// discover peers by protocol
peersChan, err := dn.StartDiscoverPeers(context.Background(), "protocol1")
if err != nil {
    log.Fatal(err)
}

go func() {
    for peer := range peersChan {
        fmt.Println(peer)
    }
}()

// register service
err = dn.RegisterService(context.Background(), "protocol1", "service1", handler)
if err != nil {
    log.Fatal(err)
}
```

