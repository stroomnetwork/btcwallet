package chain

import (
	"fmt"
	"time"
)

func SetupBitcoind(cfg *BitcoindConfig) (*BitcoindClient, error) {

	c := make(chan *BitcoindConn)

	var connTimeout time.Duration
	if cfg.ConnectionTimeout != 0 {
		connTimeout = cfg.ConnectionTimeout
	} else {
		connTimeout = bitcoindConnectionTimeout
	}

	go func() {
		chainConn, err := NewBitcoindConn(cfg)
		if err != nil {
			log.Errorf("error creating bitcoind connection: %v", err)
		}
		c <- chainConn
	}()

	select {
	case chainConn := <-c:
		if err := chainConn.Start(); err != nil {
			return nil, err
		}
		log.Debug("Starting bitcoind client...")
		btcClient := chainConn.NewBitcoindClient()
		if err := btcClient.Start(); err != nil {
			return nil, err
		}
		return btcClient, nil
	case <-time.After(connTimeout):
		fmt.Println("timeout creating bitcoind connection")
		return nil, fmt.Errorf("timeout creating bitcoind connection")
	}
}

func NewBitcoindConfig(host, user, password string) *BitcoindConfig {
	return &BitcoindConfig{
		Host: host,
		User: user,
		Pass: password,

		PollingConfig: &PollingConfig{
			BlockPollingInterval: time.Millisecond * 100,
			TxPollingInterval:    time.Millisecond * 100,
		},
	}
}
