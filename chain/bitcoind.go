package chain

import (
	"errors"
	"time"
)

func SetupBitcoind(cfg *BitcoindConfig) (*BitcoindClient, error) {

	var chainConn *BitcoindConn
	go func() {
		chainConn, _ = NewBitcoindConn(cfg)
	}()

	time.Sleep(2 * time.Second)
	if chainConn == nil {
		return nil, errors.New("failed to connect to bitcoind")
	}

	btcClient := chainConn.NewBitcoindClient()
	err := btcClient.Start()
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

		PollingConfig: &PollingConfig{
			BlockPollingInterval: time.Millisecond * 100,
			TxPollingInterval:    time.Millisecond * 100,
		},
	}
}
