package wallet

import (
	"github.com/btcsuite/btcd/wire"
	"github.com/stretchr/testify/assert"
	"github.com/stroomnetwork/btcwallet/waddrmgr"
	"github.com/stroomnetwork/btcwallet/wallet/txauthor"
	"testing"
)

func TestMinedTxDoubleSpend(t *testing.T) {
	doubleSpendTest(t, true)
}

func TestUnminedTxDoubleSpendFrom(t *testing.T) {
	doubleSpendTest(t, false)
}

func doubleSpendTest(t *testing.T, mineTx bool) {
	t.Parallel()

	w, cleanup := testWallet(t)
	defer cleanup()

	addUtxoToWallet(t, w, waddrmgr.KeyScopeBIP0049Plus)

	tx, err := createTx(t, w)
	assert.NoError(t, err)
	assert.NotNil(t, tx)

	if mineTx {
		addUtxo(t, w, tx.Tx)
	} else {
		_ = w.PublishTransaction(tx.Tx, "")
	}

	// double spend the same redemptionId
	tx, err = createTx(t, w)

	assert.Error(t, err)
	assert.Nil(t, tx)
}

func createTx(t *testing.T, w *Wallet) (*txauthor.AuthoredTx, error) {

	var redemptionId uint32 = 1

	return w.CheckDoubleSpendAndCreateTxWithRedemptionId(
		NewBlockIdentifierFromHeight(0), NewBlockIdentifierFromHeight(testBlockHeight),
		redemptionId, &waddrmgr.KeyScopeBIP0049Plus, 0, []*wire.TxOut{getTxOut(t)}, 1, 100,
		CoinSelectionLargest, false)
}
