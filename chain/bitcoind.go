package chain

import (
	"os"
	"time"
)

// SetupBitcoind starts up a bitcoind node with either a zmq connection or
// rpc polling connection and returns a client wrapper of this connection.
func SetupBitcoind(cfg *BitcoindConfig, rpcPolling bool) (*BitcoindClient, error) {

	tempBitcoindDir, err := os.MkdirTemp("", "bitcoind")
	if err != nil {
		return nil, err
	}

	zmqBlockHost := "ipc://" + tempBitcoindDir + "/blocks.socket"
	zmqTxHost := "ipc://" + tempBitcoindDir + "/tx.socket"

	if rpcPolling {
		cfg.PollingConfig = &PollingConfig{
			BlockPollingInterval: time.Millisecond * 100,
			TxPollingInterval:    time.Millisecond * 100,
		}
	} else {
		cfg.ZMQConfig = &ZMQConfig{
			ZMQBlockHost:           zmqBlockHost,
			ZMQTxHost:              zmqTxHost,
			ZMQReadDeadline:        5 * time.Second,
			MempoolPollingInterval: time.Millisecond * 100,
		}
	}

	chainConn, err := NewBitcoindConn(cfg)
	if err != nil {
		return nil, err
	}

	// Create a bitcoind client.
	btcClient := chainConn.NewBitcoindClient()
	err = btcClient.Start()
	if err != nil {
		return nil, err
	}

	return btcClient, nil
}

func NewBitcoindConfig(host, user, password string) *BitcoindConfig {
	return &BitcoindConfig{
		Host: host,
		User: user,
		Pass: password,
	}
}
