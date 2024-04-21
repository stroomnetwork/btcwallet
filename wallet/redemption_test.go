package wallet

import (
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
	"github.com/stretchr/testify/assert"
	"github.com/stroomnetwork/btcwallet/waddrmgr"
	"github.com/stroomnetwork/btcwallet/wallet/txauthor"
	"testing"
)

// TODO(dp) test mined double spend with miner.Client.Generate(1)

func TestUnminedDoubleSpendFromSameWallet(t *testing.T) {
	t.Parallel()

	w, cleanup := testWallet(t)
	defer cleanup()

	_ = addUtxoToWallet(t, w)

	tx, err := createTX(w)

	assert.NoError(t, err)
	assert.NotNil(t, tx)

	w.PublishTransaction(tx.Tx, "")

	// double spend the same redemptionId
	tx, err = createTX(w)

	assert.Error(t, err)
	assert.Nil(t, tx)
}

func createTX(w *Wallet) (*txauthor.AuthoredTx, error) {

	var redemptionId uint32 = 1

	return w.CreateTxWithRedemptionIdAndCheckDoubleSpend(NewBlockIdentifierFromHeight(0), NewBlockIdentifierFromHeight(0),
		redemptionId, &waddrmgr.KeyScopeBIP0049Plus, 0, []*wire.TxOut{getTxOut()}, 1, 100,
		CoinSelectionLargest, false)
}

func addUtxoToWallet(t *testing.T, w *Wallet) error {
	keyScope := waddrmgr.KeyScopeBIP0049Plus
	addr, err := w.CurrentAddress(0, keyScope)
	if err != nil {
		t.Fatalf("unable to get current address: %v", addr)
	}
	p2shAddr, err := txscript.PayToAddrScript(addr)
	if err != nil {
		t.Fatalf("unable to convert wallet address to p2sh: %v", err)
	}

	incomingTx := &wire.MsgTx{
		TxIn: []*wire.TxIn{
			{},
		},
		TxOut: []*wire.TxOut{},
	}
	for amt := int64(5000); amt <= 125000; amt += 10000 {
		incomingTx.AddTxOut(wire.NewTxOut(amt, p2shAddr))
	}

	addUtxo(t, w, incomingTx)
	return err
}

func getTxOut() *wire.TxOut {
	addr, err := btcutil.DecodeAddress("SR9zEMt5qG7o1Q7nGcLPCMqv5BrNHcw2zi", &chaincfg.SimNetParams)
	if err != nil {
		log.Info(err)
		return nil
	}
	p2shAddr, err := txscript.PayToAddrScript(addr)
	if err != nil {
		log.Info(err)
		return nil
	}
	return wire.NewTxOut(10000, p2shAddr)
}
