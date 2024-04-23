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
	assert.NoError(t, err)
	assert.NotNil(t, pubKey)

	w.FrostSigner = validators[0]
	err = w.Unlock([]byte("world"), time.After(10*time.Minute))
	assert.NoError(t, err)

	err = w.ImportPublicKey(pubKey, waddrmgr.TaprootPubKey)
	assert.NoError(t, err)

	p2shAddr, err := txscript.PayToTaprootScript(pubKey)
	assert.NoError(t, err)
	assert.NotNil(t, p2shAddr)

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

	accounts, err := w.Accounts(waddrmgr.KeyScopeBIP0086)
	assert.NoError(t, err)
	assert.NotNil(t, accounts)
	assert.True(t, len(accounts.Accounts) > 1)

	address, err := w.CurrentAddress(0, waddrmgr.KeyScopeBIP0044)
	assert.NoError(t, err)

	out := wire.NewTxOut(10000, address.ScriptAddress())

	tx, err := w.CreateSimpleTx(&waddrmgr.KeyScopeBIP0086, accounts.Accounts[1].AccountNumber, []*wire.TxOut{out}, 1, 10, CoinSelectionLargest, false)
	assert.NoError(t, err)
	assert.NotNil(t, tx)

	err = w.PublishTransaction(tx.Tx, "")
	assert.NoError(t, err)
}
