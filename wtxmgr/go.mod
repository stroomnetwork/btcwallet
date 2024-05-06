module github.com/btcsuite/btcwallet/wtxmgr

require (
	github.com/btcsuite/btcd v0.24.0
	github.com/btcsuite/btcd/btcutil v1.1.5
	github.com/btcsuite/btcd/chaincfg/chainhash v1.1.0
	github.com/btcsuite/btclog v0.0.0-20170628155309-84c8d2346e9f
	github.com/btcsuite/btcwallet/walletdb v1.4.2
	github.com/lightningnetwork/lnd/clock v1.0.1
)

require (
	github.com/btcsuite/btcd/btcec/v2 v2.1.3 // indirect
	github.com/decred/dcrd/crypto/blake256 v1.0.0 // indirect
	github.com/decred/dcrd/dcrec/secp256k1/v4 v4.0.1 // indirect
	go.etcd.io/bbolt v1.3.7 // indirect
	golang.org/x/crypto v0.17.0 // indirect
	golang.org/x/sys v0.15.0 // indirect
)

go 1.18
