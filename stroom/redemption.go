package stroom

import (
	"fmt"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/wire"
	"github.com/stroomnetwork/btcwallet/waddrmgr"
	"github.com/stroomnetwork/btcwallet/wallet"
	"github.com/stroomnetwork/btcwallet/wallet/txauthor"
	"golang.org/x/net/context"
	"time"
)

func CreateSimpleTxWithRedemptionId(w *wallet.Wallet, coinSelectKeyScope *waddrmgr.KeyScope,
	account uint32, outputs []*wire.TxOut, minconf int32,
	satPerKb btcutil.Amount, coinSelectionStrategy wallet.CoinSelectionStrategy,
	dryRun bool, redemptionId uint32, optFuncs ...wallet.TxCreateOption) (*txauthor.AuthoredTx, error) {

	spent, hash, err := IsAlreadySpent(w, redemptionId, nil, nil)

	if err != nil {
		return nil, err
	}

	if spent {
		return nil, fmt.Errorf("redemption id %d already spent in tx %s", redemptionId, hash)
	}

	return w.CreateSimpleTxWithRedemptionId(coinSelectKeyScope, account, outputs, minconf, satPerKb, coinSelectionStrategy, dryRun, redemptionId, optFuncs...)
}
func IsAlreadySpent(w *wallet.Wallet, redemptionId uint32, start *wallet.BlockIdentifier, end *wallet.BlockIdentifier) (bool, chainhash.Hash, error) {

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	gtr, err := w.GetTransactions(start, end, "", ctx.Done())
	if err != nil {
		return false, nilHash(), err
	}

	for _, block := range gtr.MinedTransactions {
		isPresent, hash, err := isRedemptionIdPresent(w, redemptionId, block.Transactions)
		if isPresent {
			return isPresent, hash, err
		}
	}

	gtr, err = w.GetTransactions(wallet.NewBlockIdentifierFromHeight(-1), end, "", ctx.Done())
	if err != nil {
		return false, nilHash(), err
	}
	return isRedemptionIdPresent(w, redemptionId, gtr.UnminedTransactions)
}

func isRedemptionIdPresent(w *wallet.Wallet, redemptionId uint32, txs []wallet.TransactionSummary) (bool, chainhash.Hash, error) {
	for _, tx := range txs {
		if tx.MyInputs != nil {
			txDetails, _ := wallet.UnstableAPI(w).TxDetails(tx.Hash)
			for _, input := range txDetails.MsgTx.TxIn {
				if input.Sequence == redemptionId {
					return true, txDetails.Hash, nil
				}
			}
		}
	}
	return false, nilHash(), nil
}

func nilHash() chainhash.Hash {
	return chainhash.Hash{}
}
