package wallet

import (
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
	"github.com/stretchr/testify/assert"
	"github.com/stroomnetwork/btcwallet/waddrmgr"
	"testing"
)

func addUtxoToWallet(t *testing.T, w *Wallet, keyScope waddrmgr.KeyScope) {
	addr, err := w.CurrentAddress(0, keyScope)
	assert.NoError(t, err)

	p2shAddr, err := txscript.PayToAddrScript(addr)
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
}

func getTxOut(t *testing.T) *wire.TxOut {
	addr, err := btcutil.DecodeAddress("SR9zEMt5qG7o1Q7nGcLPCMqv5BrNHcw2zi", &chaincfg.SimNetParams)
	assert.NoError(t, err)

	p2shAddr, err := txscript.PayToAddrScript(addr)
	assert.NoError(t, err)

	return wire.NewTxOut(10000, p2shAddr)
}
