# Code Comprehension Challenge

I checked the code from the first release candidate of CometBFT 2.0 ( https://github.com/cometbft/cometbft/blob/v2.0.0-rc1 ).

The code from the application-level Handshake itself lives at https://github.com/cometbft/cometbft/blob/v2.0.0-rc1/p2p/handshake.go

The Handshake happens right after a peer connects to a Node. Connections are handled by the Switch abstraction 
(see https://github.com/cometbft/cometbft/blob/v2.0.0-rc1/p2p/switch.go) within each node.

First, a connection happens through a transport, which is TCP-based and encrypted (AKA upgraded in the code) 
through a protocol called STS (Station-to-Station), as shown by:

* https://github.com/cometbft/cometbft/blob/v2.0.0-rc1/node/setup.go (createTransport() at line 411)
* https://github.com/cometbft/cometbft/blob/v2.0.0-rc1/p2p/transport/tcp/tcp.go (NewMultiplexTransport() at line 126)
* Calls to `upgrade()` when dialing and accepting peers 
  ( https://github.com/cometbft/cometbft/blob/v2.0.0-rc1/p2p/transport/tcp/tcp.go lines 184 and 281 )
* Which calls `upgradeSecretConn()` at https://github.com/cometbft/cometbft/blob/v2.0.0-rc1/p2p/transport/tcp/tcp.go 
  line 388
* Which in turn calls `MakeSecretConnection()` at
  https://github.com/cometbft/cometbft/blob/v2.0.0-rc1/p2p/transport/tcp/conn/secret_connection.go line 101.


Unfortunately I am not a cryptography expert but, by reading the code, and the (seemingly outdated) documentation at
https://github.com/cometbft/cometbft/blob/main/spec/p2p/legacy-docs/peer.md#connections
`MakeSecretConnection()`does a Diffie-Helman key-exchange (using X25519 keys) and chacha20poly1305 for encryption.

First, ephemeral keys are generated and exchanged (at https://github.com/cometbft/cometbft/blob/v2.0.0-rc1/p2p/transport/tcp/conn/secret_connection.go line 110).

Then, the common Diffie Hellman secret is computed (line 128).

And, finally, secrets for encryption and decryption are derived and saved (lines 137 to 151).

After that, when dialing, the remote peer ID is verified ( https://github.com/cometbft/cometbft/blob/v2.0.0-rc1/p2p/transport/tcp/tcp.go  line  366)


Now, getting to the application-level handshake itself (https://github.com/cometbft/cometbft/blob/v2.0.0-rc1/p2p/handshake.go)
it seems like node information (`DefaultNodeInfo` protobuf message at https://github.com/cometbft/cometbft/blob/v2.0.0-rc1/proto/cometbft/p2p/v1/types.proto) 
is exchanged (line 77) through the now encrypted transport.

The remote node information is then mutually validated and checked for compatibility with the local node's information. 
If they are compatible (which involves checking the protocol version, network and common channels) the handshake succeeds.

Here is a Diagram representing all the exchanges between the nodes (including the initial TCP connection).

It is worth noting that the TCP connection establishment is simplified (the TCP connection is composed of a 3-way 
handshake see https://en.wikipedia.org/wiki/Transmission_Control_Protocol#Connection_establishment )


```
Dialer Node                                  Listener Node
-----------                                  -------------
TCP connect ------------------------------->  Accept TCP

STS encryption "upgrade" handshake 
(X25519 + AEAD)  <------------------------->  (establish shared keys)
- exchange ephemeral pubkeys
- derive session keys

PeerID authentication (match expected ID) -->  verify / reject

NodeInfo exchange (protobuf) -------------->  send DefaultNodeInfo
                              <------------  send DefaultNodeInfo

CompatibleWith() check (channels, versions)
```