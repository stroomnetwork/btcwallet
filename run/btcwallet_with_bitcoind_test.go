package run

import (
	"fmt"
	"github.com/btcsuite/btcd/integration/rpctest"
	"github.com/btcsuite/btclog"
	"github.com/btcsuite/btcwallet/wtxmgr"
	"github.com/stretchr/testify/assert"
	"github.com/stroomnetwork/btcwallet/cfgutil"
	"github.com/stroomnetwork/btcwallet/chain"
	"github.com/stroomnetwork/btcwallet/wallet"
	"github.com/stroomnetwork/frost"
	"github.com/stroomnetwork/frost/crypto"
	"math/rand"
	"os"
	"os/exec"
	"path"
	"testing"
	"time"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/txscript"
	"github.com/stretchr/testify/require"
)

func TestImportAddress(t *testing.T) {
	// Set up 2 btcd miners.
	miner1, miner2 := setupMiners(t)
	addr := miner1.P2PAddress()

	require.NotNil(t, miner1)
	require.NotNil(t, miner2)

	// Set up a bitcoind node and connect it to miner 1.
	cfg := setupBitcoind(t, addr)

	btcWallet := createBtcWallet(t, cfg)

	time.Sleep(5 * time.Second)
	t.Log(btcWallet.ChainSynced())
}

func createBtcWallet(t *testing.T, cfg *chain.BitcoindConfig) *wallet.Wallet {
	signer, key1, key2 := createFrostSigner(t)

	tempWalletDir, err := os.MkdirTemp("", "wallet")
	require.NoError(t, err)

	t.Cleanup(func() {
		os.RemoveAll(tempWalletDir)
	})
	initLogRotator(path.Join(tempWalletDir, "logs"))

	btcw := btclog.NewBackend(os.Stdout).Logger("BTCW")
	btcw.SetLevel(btclog.LevelDebug)
	wallet.UseLogger(btcw)
	chain.UseLogger(btcw)
	wtxmgr.UseLogger(btcw)

	config := DefaultConfig()
	config.Regtest = true

	//config.AppDataDir = cfgutil.SetExplicitString(m.config.Appdata)
	//config.CAFile = cfgutil.SetExplicitString(cfg.certFile)
	config.CanConsolePrompt = false
	config.WalletPrivatePass = "password"
	config.DisableServerTLS = true

	config.RPCConnect = cfg.Host
	config.BtcdUsername = cfg.User
	config.BtcdPassword = cfg.Pass
	config.AppDataDir = cfgutil.SetExplicitString(tempWalletDir)

	walletConfig := &BtcwalletConfig{
		Signer:         signer,
		Pk1:            key1,
		Pk2:            key2,
		BitcoindConfig: cfg,
		Config:         config,
		InitTimeout:    10 * time.Second,
		FeeCoefficient: 0,
	}

	w, err := SafeInitWalletWithConfig(walletConfig)
	require.NoError(t, err)
	require.NoError(t, w.Unlock([]byte("password"), nil))

	return w
}

func createFrostSigner(t *testing.T) (*frost.SoloSigner, *btcec.PublicKey, *btcec.PublicKey) {
	seedKey1, _ := crypto.GetDeterministicKeysBip340("key1")
	signer := frost.NewSoloSigner(seedKey1)

	pk1, err := signer.RequestPubKey("pk1")
	assert.NoError(t, err)
	assert.NotNil(t, pk1)
	assert.True(t, pk1.IsOnCurve())

	pk2, err := signer.RequestPubKey("pk2")
	assert.NoError(t, err)
	assert.NotNil(t, pk2)
	assert.True(t, pk2.IsOnCurve())
	assert.NotEqual(t, pk1.SerializeCompressed(), pk2.SerializeCompressed())

	return signer, pk1, pk2
}

// setUpMiners sets up two miners that can be used for a re-org test.
func setupMiners(t *testing.T) (*rpctest.Harness, *rpctest.Harness) {
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

// setupBitcoind starts up a bitcoind node with either a zmq connection or
// rpc polling connection and returns a client wrapper of this connection.
func setupBitcoind(t *testing.T, minerAddr string) *chain.BitcoindConfig {

	// Start a bitcoind instance and connect it to miner1.
	tempBitcoindDir, err := os.MkdirTemp("", "bitcoind")
	require.NoError(t, err)

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
		"-rpcuser=weks",
		"-rpcpassword=weks",
		fmt.Sprintf("-rpcport=%d", rpcPort),
		"-disablewallet",
	)
	require.NoError(t, bitcoind.Start())

	t.Cleanup(func() {
		bitcoind.Process.Kill()
		bitcoind.Wait()
	})

	// Wait for the bitcoind instance to start up.
	time.Sleep(time.Second)

	host := fmt.Sprintf("127.0.0.1:%d", rpcPort)
	cfg := &chain.BitcoindConfig{
		ChainParams: &chaincfg.RegressionNetParams,
		Host:        host,
		User:        "weks",
		Pass:        "weks",
		// Fields only required for pruned nodes, not
		// needed for these tests.
		Dialer:             nil,
		PrunedModeMaxPeers: 0,
	}

	cfg.PollingConfig = &chain.PollingConfig{
		BlockPollingInterval: time.Millisecond * 100,
		TxPollingInterval:    time.Millisecond * 100,
	}

	return cfg
}

// randPubKeyHashScript generates a P2PKH script that pays to the public key of
// a randomly-generated private key.
func randPubKeyHashScript() ([]byte, *btcec.PrivateKey, error) {
	privKey, err := btcec.NewPrivateKey()
	if err != nil {
		return nil, nil, err
	}

	pubKeyHash := btcutil.Hash160(privKey.PubKey().SerializeCompressed())
	addrScript, err := btcutil.NewAddressPubKeyHash(
		pubKeyHash, &chaincfg.RegressionNetParams,
	)
	if err != nil {
		return nil, nil, err
	}

	pkScript, err := txscript.PayToAddrScript(addrScript)
	if err != nil {
		return nil, nil, err
	}

	return pkScript, privKey, nil
}
