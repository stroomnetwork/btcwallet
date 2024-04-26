package wallet

import (
	"fmt"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stroomnetwork/btcwallet/waddrmgr"
	"github.com/stroomnetwork/frost/crypto"
)

func (w *Wallet) ImportBtcAddressWithEthAddr(btcAddr, ethAddr string) error {

	lc, err := w.lcFromEthAddr(ethAddr)
	if err != nil {
		return err
	}

	pubKey := lc.GetCombinedPubKey()
	importedAddress, err := w.ImportPublicKeyReturnAddress(pubKey, waddrmgr.TaprootPubKey)
	if err != nil {
		return err
	}

	if btcAddr == "" && importedAddress != nil {
		if importedAddress.Address().EncodeAddress() != btcAddr {
			return fmt.Errorf("address mismatch: %s != %s", importedAddress, btcAddr)
		}
	}

	w.btcAddrToLc[btcAddr] = lc
	w.btcAddrToEthAddr[btcAddr] = ethAddr

	return nil
}

func (w *Wallet) lcFromEthAddr(ethAddrStr string) (*crypto.LinearCombination, error) {
	ethAddr := common.HexToAddress(ethAddrStr)

	uint256Ty, _ := abi.NewType("uint256", "uint256", nil)
	addressTy, _ := abi.NewType("address", "address", nil)

	arguments := abi.Arguments{
		{
			Type: uint256Ty,
		},
		{
			Type: uint256Ty,
		},
		{
			Type: addressTy,
		},
	}

	b1, _ := arguments.Pack(
		w.pk1.X(),
		w.pk1.Y(),
		ethAddr,
	)
	h1 := crypto.Sha256(b1)
	c1FromAddr, _ := crypto.PrivkeyFromBytes(h1[:])

	b2, _ := arguments.Pack(
		w.pk2.X(),
		w.pk2.Y(),
		ethAddr,
	)
	h2 := crypto.Sha256(b2)
	c2FromAddr, _ := crypto.PrivkeyFromBytes(h2[:])

	lc, err := crypto.NewLinearCombination(
		[]*btcec.PublicKey{w.pk1, w.pk2},
		[]*btcec.PrivateKey{c1FromAddr, c2FromAddr},
		crypto.PrivKeyFromInt(0),
	)
	if err != nil {
		return nil, err
	}

	return lc, nil
}
