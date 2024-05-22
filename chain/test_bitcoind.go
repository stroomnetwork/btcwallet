package chain

import (
	"fmt"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/integration/rpctest"
	"github.com/stretchr/testify/require"
	"math/rand"
	"os"
	"os/exec"
	"testing"
	"time"
)

// setUpMiners sets up two miners that can be used for a re-org test.
func SetupMiners(t *testing.T) (*rpctest.Harness, *rpctest.Harness) {
	trickle := fmt.Sprintf("--trickleinterval=%v", 10*time.Millisecond)
	args := []string{trickle}

	miner1, err := rpctest.New(
		&chaincfg.RegressionNetParams, nil, args, "",
	)
	require.NoError(t, err)

	t.Cleanup(func() {
		miner1.TearDown()
	})

	require.NoError(t, miner1.SetUp(true, 1))

	miner2, err := rpctest.New(
		&chaincfg.RegressionNetParams, nil, args, "",
	)
	require.NoError(t, err)

	t.Cleanup(func() {
		miner2.TearDown()
	})

	require.NoError(t, miner2.SetUp(false, 0))

	// Connect the miners.
	require.NoError(t, rpctest.ConnectNode(miner1, miner2))

	err = rpctest.JoinNodes(
		[]*rpctest.Harness{miner1, miner2}, rpctest.Blocks,
	)
	require.NoError(t, err)

	return miner1, miner2
}

// SetupTestBitcoind starts up a bitcoind node with either a zmq connection or
// rpc polling connection and returns a client wrapper of this connection.
func SetupTestBitcoind(t *testing.T, minerAddr string,
	rpcPolling bool) *BitcoindClient {

	// Start a bitcoind instance and connect it to miner1.
	tempBitcoindDir, err := os.MkdirTemp("", "bitcoind")
	require.NoError(t, err)

	zmqBlockHost := "ipc:///" + tempBitcoindDir + "/blocks.socket"
	zmqTxHost := "ipc:///" + tempBitcoindDir + "/tx.socket"
	t.Cleanup(func() {
		os.RemoveAll(tempBitcoindDir)
	})

	rpcPort := rand.Int()%(65536-1024) + 1024
	bitcoind := exec.Command(
		"bitcoind",
		"-datadir="+tempBitcoindDir,
		"-regtest",
		"-connect="+minerAddr,
		"-txindex",
		"-rpcauth=weks:469e9bb14ab2360f8e226efed5ca6f"+
			"d$507c670e800a95284294edb5773b05544b"+
			"220110063096c221be9933c82d38e1",
		fmt.Sprintf("-rpcport=%d", rpcPort),
		"-disablewallet",
		"-zmqpubrawblock="+zmqBlockHost,
		"-zmqpubrawtx="+zmqTxHost,
	)
	require.NoError(t, bitcoind.Start())

	t.Cleanup(func() {
		bitcoind.Process.Kill()
		bitcoind.Wait()
	})

	// Wait for the bitcoind instance to start up.
	time.Sleep(time.Second)

	host := fmt.Sprintf("127.0.0.1:%d", rpcPort)
	cfg := &BitcoindConfig{
		ChainParams: &chaincfg.RegressionNetParams,
		Host:        host,
		User:        "weks",
		Pass:        "weks",
		// Fields only required for pruned nodes, not
		// needed for these tests.
		Dialer:             nil,
		PrunedModeMaxPeers: 0,
	}

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
	require.NoError(t, err)
	require.NoError(t, chainConn.Start())

	t.Cleanup(func() {
		chainConn.Stop()
	})

	// Create a bitcoind client.
	btcClient := chainConn.NewBitcoindClient()
	require.NoError(t, btcClient.Start())

	t.Cleanup(func() {
		btcClient.Stop()
	})

	return btcClient
}
