package chain

import (
	"time"
)

func SetupBitcoind(cfg *BitcoindConfig) (*BitcoindClient, error) {

	cfg.PollingConfig = &PollingConfig{
		BlockPollingInterval: time.Millisecond * 100,
		TxPollingInterval:    time.Millisecond * 100,
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
