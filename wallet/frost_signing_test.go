package wallet

import (
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
	"github.com/stretchr/testify/assert"
	"github.com/stroomnetwork/btcwallet/frost"
	"github.com/stroomnetwork/btcwallet/waddrmgr"
	"testing"
	"time"
)

func TestFrostSigning(t *testing.T) {
	t.Parallel()

	w, cleanup := testWallet(t)
	defer cleanup()

	validators := frost.GetValidators(5, 3)
	pubKey, err := validators[0].MakePubKey("test")
	if err != nil {
		log.Info(err)
		return
	}

	w.FrostSigner = validators[0]
	err = w.Unlock([]byte("world"), time.After(10*time.Minute))
	assert.NoError(t, err)
	err = w.ImportPublicKey(pubKey, waddrmgr.TaprootPubKey)
	assert.NoError(t, err)

	p2shAddr, err := txscript.PayToTaprootScript(pubKey)
	assert.NoError(t, err)

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

	//addUtxoToWallet(t, w, waddrmgr.KeyScopeBIP0086)

	tx, err := w.CreateSimpleTx(&waddrmgr.KeyScopeBIP0086, 0, []*wire.TxOut{getTxOut(t)}, 1, 100, CoinSelectionLargest, false)
	assert.NoError(t, err)
	assert.NotNil(t, tx)

	_ = w.PublishTransaction(tx.Tx, "")
}
