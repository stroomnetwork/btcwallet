// Copyright (c) 2016 The btcsuite developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

// Package txauthor provides transaction creation code for wallets.
package txauthor

import (
	"bytes"
	"encoding/gob"
	"errors"
	"fmt"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
	"github.com/btcsuite/btcwallet/wallet/txrules"
	"github.com/btcsuite/btcwallet/wallet/txsizes"
	"github.com/stroomnetwork/frost"
	"github.com/stroomnetwork/frost/crypto"
)

// SumOutputValues sums up the list of TxOuts and returns an Amount.
func SumOutputValues(outputs []*wire.TxOut) (totalOutput btcutil.Amount) {
	for _, txOut := range outputs {
		totalOutput += btcutil.Amount(txOut.Value)
	}
	return totalOutput
}

// InputSource provides transaction inputs referencing spendable outputs to
// construct a transaction outputting some target amount.  If the target amount
// can not be satisified, this can be signaled by returning a total amount less
// than the target or by returning a more detailed error implementing
// InputSourceError.
type InputSource func(target btcutil.Amount) (total btcutil.Amount, inputs []*wire.TxIn,
	inputValues []btcutil.Amount, scripts [][]byte, err error)

// InputSourceError describes the failure to provide enough input value from
// unspent transaction outputs to meet a target amount.  A typed error is used
// so input sources can provide their own implementations describing the reason
// for the error, for example, due to spendable policies or locked coins rather
// than the wallet not having enough available input value.
type InputSourceError interface {
	error
	InputSourceError()
}

// Default implementation of InputSourceError.
type insufficientFundsError struct{}

func (insufficientFundsError) InputSourceError() {}
func (insufficientFundsError) Error() string {
	return "insufficient funds available to construct transaction"
}

// AuthoredTx holds the state of a newly-created transaction and the change
// output (if one was added).
type AuthoredTx struct {
	Tx              *wire.MsgTx
	PrevScripts     [][]byte
	PrevInputValues []btcutil.Amount
	TotalInput      btcutil.Amount
	ChangeIndex     int // negative if no change
}

// ChangeSource provides change output scripts for transaction creation.
type ChangeSource struct {
	// NewScript is a closure that produces unique change output scripts per
	// invocation.
	NewScript func() ([]byte, error)

	// ScriptSize is the size in bytes of scripts produced by `NewScript`.
	ScriptSize int
}

// NewUnsignedTransaction creates an unsigned transaction paying to one or more
// non-change outputs.  An appropriate transaction fee is included based on the
// transaction size.
//
// Transaction inputs are chosen from repeated calls to fetchInputs with
// increasing targets amounts.
//
// If any remaining output value can be returned to the wallet via a change
// output without violating mempool dust rules, a P2WPKH change output is
// appended to the transaction outputs.  Since the change output may not be
// necessary, fetchChange is called zero or one times to generate this script.
// This function must return a P2WPKH script or smaller, otherwise fee estimation
// will be incorrect.
//
// If successful, the transaction, total input value spent, and all previous
// output scripts are returned.  If the input source was unable to provide
// enough input value to pay for every output any any necessary fees, an
// InputSourceError is returned.
//
// BUGS: Fee estimation may be off when redeeming non-compressed P2PKH outputs.
func NewUnsignedTransaction(outputs []*wire.TxOut, feeRatePerKb btcutil.Amount,
	fetchInputs InputSource, changeSource *ChangeSource) (*AuthoredTx, error) {
	return NewUnsignedTransactionWithAddedStroomFee(outputs, feeRatePerKb, fetchInputs, changeSource, 1)
}

func NewUnsignedTransactionWithAddedStroomFee(outputs []*wire.TxOut, feeRatePerKb btcutil.Amount,
	fetchInputs InputSource, changeSource *ChangeSource, feeCoefficient float64) (*AuthoredTx, error) {

	if len(outputs) == 0 {
		return nil, errors.New("no transaction outputs provided")
	}

	targetAmount := SumOutputValues(outputs)
	fmt.Printf("targetAmount: %v\n", targetAmount)
	estimatedSize := txsizes.EstimateVirtualSize(
		0, 0, 1, 0, outputs, changeSource.ScriptSize,
	)
	targetFee := txrules.FeeForSerializeSize(feeRatePerKb, estimatedSize).MulF64(feeCoefficient)
	fmt.Printf("targetFee: %v, estimatedSize: %v\n", targetFee, estimatedSize)

	if targetAmount < targetFee {
		return nil, fmt.Errorf("redeem amount(%v) < targetFee(%v) \n", int64(targetAmount), int64(targetFee))
	}

	for {
		inputAmount, inputs, inputValues, scripts, err := fetchInputs(targetAmount + targetFee)
		if err != nil {
			fmt.Errorf("fetchInputs error: %v\n", err)
			return nil, err
		}
		if inputAmount < targetAmount+targetFee {
			fmt.Errorf("insufficientFundsError: inputAmout(%d) < targetAmount(%d) + targetFee(%d)\n",
				inputAmount, targetAmount, targetFee)
			return nil, insufficientFundsError{}
		}
		fmt.Printf("inputAmount: %v, lengths: %v, %v, %v\n", inputAmount, len(inputs), len(inputValues), len(scripts))

		// We count the types of inputs, which we'll use to estimate
		// the vsize of the transaction.
		var nested, p2wpkh, p2tr, p2pkh int
		for _, pkScript := range scripts {
			switch {
			// If this is a p2sh output, we assume this is a
			// nested P2WKH.
			case txscript.IsPayToScriptHash(pkScript):
				nested++
			case txscript.IsPayToWitnessPubKeyHash(pkScript):
				p2wpkh++
			case txscript.IsPayToTaproot(pkScript):
				p2tr++
			default:
				p2pkh++
			}
		}

		maxSignedSize := txsizes.EstimateVirtualSize(
			p2pkh, p2tr, p2wpkh, nested, outputs, changeSource.ScriptSize,
		)
		maxRequiredFee := txrules.FeeForSerializeSize(feeRatePerKb, maxSignedSize)

		var totalFee btcutil.Amount
		if feeCoefficient == 0 {
			totalFee = maxRequiredFee
		} else {
			totalFee = maxRequiredFee.MulF64(feeCoefficient)
		}
		remainingAmount := inputAmount - targetAmount
		fmt.Printf("maxRequiredFee: %v, totalFee: %v, remainingAmount: %v\n", maxRequiredFee, totalFee, remainingAmount)
		if remainingAmount < totalFee {
			targetFee = totalFee
			fmt.Printf("remainingAmount(%v) < totalFee(%v), continue\n", remainingAmount, totalFee)
			continue
		}

		fmt.Printf("inputs size: %v\n", len(inputs))
		for _, input := range inputs {
			fmt.Printf("input: %v, amount: %v\n", input.PreviousOutPoint, inputAmount)
		}

		unsignedTransaction := &wire.MsgTx{
			Version:  wire.TxVersion,
			TxIn:     inputs,
			TxOut:    outputs,
			LockTime: 0,
		}

		if targetAmount < totalFee {
			return nil, fmt.Errorf("redeem amount(%v) < totalFee(%v) \n", int(targetAmount), int64(totalFee))
		}

		// fees should be taken away from the output amount
		outputs[0].Value -= int64(totalFee)

		changeIndex := -1
		// the change includes stroom fees
		// TODO shall we check for dust change amount?
		changeAmount := inputAmount - targetAmount - maxRequiredFee + totalFee
		changeScript, err := changeSource.NewScript()
		if err != nil {
			fmt.Errorf("changeSource.NewScript error: %v\n", err)
			return nil, err
		}
		change := wire.NewTxOut(int64(changeAmount), changeScript)
		if changeAmount != 0 && !txrules.IsDustOutput(change, txrules.DefaultRelayFeePerKb) {
			l := len(outputs)
			unsignedTransaction.TxOut = append(outputs[:l:l], change)
			changeIndex = l
		}

		return &AuthoredTx{
			Tx:              unsignedTransaction,
			PrevScripts:     scripts,
			PrevInputValues: inputValues,
			TotalInput:      inputAmount,
			ChangeIndex:     changeIndex,
		}, nil
	}
}

// RandomizeOutputPosition randomizes the position of a transaction's output by
// swapping it with a random output.  The new index is returned.  This should be
// done before signing.
func RandomizeOutputPosition(outputs []*wire.TxOut, index int) int {
	r := cprng.Int31n(int32(len(outputs)))
	outputs[r], outputs[index] = outputs[index], outputs[r]
	return int(r)
}

// RandomizeChangePosition randomizes the position of an authored transaction's
// change output.  This should be done before signing.
func (tx *AuthoredTx) RandomizeChangePosition() {
	tx.ChangeIndex = RandomizeOutputPosition(tx.Tx.TxOut, tx.ChangeIndex)
}

// SecretsSource provides private keys and redeem scripts necessary for
// constructing transaction input signatures.  Secrets are looked up by the
// corresponding Address for the previous output script.  Addresses for lookup
// are created using the source's blockchain parameters and means a single
// SecretsSource can only manage secrets for a single chain.
//
// TODO: Rewrite this interface to look up private keys and redeem scripts for
// pubkeys, pubkey hashes, script hashes, etc. as separate interface methods.
// This would remove the ChainParams requirement of the interface and could
// avoid unnecessary conversions from previous output scripts to Addresses.
// This can not be done without modifications to the txscript package.
type SecretsSource interface {
	txscript.KeyDB
	txscript.ScriptDB
	ChainParams() *chaincfg.Params
}

// AddAllInputScripts modifies transaction a transaction by adding inputs
// scripts for each input.  Previous output scripts being redeemed by each input
// are passed in prevPkScripts and the slice length must match the number of
// inputs.  Private keys and redeem scripts are looked up using a SecretsSource
// based on the previous output script.
func AddAllInputScripts(signer frost.Signer, linearCombinations map[string]*crypto.LinearCombination, data []byte,
	tx *wire.MsgTx, prevPkScripts [][]byte, inputValues []btcutil.Amount, secrets SecretsSource) error {

	inputFetcher, err := TXPrevOutFetcher(tx, prevPkScripts, inputValues)
	if err != nil {
		return err
	}

	inputs := tx.TxIn
	hashCache := txscript.NewTxSigHashes(tx, inputFetcher)
	chainParams := secrets.ChainParams()

	if len(inputs) != len(prevPkScripts) {
		return errors.New("tx.TxIn and prevPkScripts slices must " +
			"have equal length")
	}

	signDescriptors := make([]*crypto.LinearSignDescriptor, 0)
	dataPerInput := make([]*InputData, 0)

	for i := range inputs {
		pkScript := prevPkScripts[i]

		switch {
		// If this is a p2sh output, who's script hash pre-image is a
		// witness program, then we'll need to use a modified signing
		// function which generates both the sigScript, and the witness
		// script.
		case txscript.IsPayToScriptHash(pkScript):
			err := spendNestedWitnessPubKeyHash(
				inputs[i], pkScript, int64(inputValues[i]),
				chainParams, secrets, tx, hashCache, i,
			)
			if err != nil {
				return err
			}

		case txscript.IsPayToWitnessPubKeyHash(pkScript):
			err := spendWitnessKeyHash(
				inputs[i], pkScript, int64(inputValues[i]),
				chainParams, secrets, tx, hashCache, i,
			)
			if err != nil {
				return err
			}

		case txscript.IsPayToTaproot(pkScript):
			descriptor, inputData, err := spendTaprootKey(linearCombinations, pkScript, int64(inputValues[i]),
				chainParams, tx, hashCache, i,
			)
			if err != nil {
				return err
			}

			signDescriptors = append(signDescriptors, descriptor)
			dataPerInput = append(dataPerInput, inputData)

		default:
			sigScript := inputs[i].SignatureScript
			script, err := txscript.SignTxOutput(chainParams, tx, i,
				pkScript, txscript.SigHashAll, secrets, secrets,
				sigScript)
			if err != nil {
				return err
			}
			inputs[i].SignatureScript = script
		}
	}

	if len(signDescriptors) > 0 {
		txData, err := SerializeTxData(NewTxData(data, tx, dataPerInput))
		if err != nil {
			return err
		}

		sd := &crypto.MultiSignatureDescriptor{
			Data:            txData,
			SignDescriptors: signDescriptors,
		}

		signatures, err := signer.SignAdvanced(sd)
		if err != nil {
			return err
		}

		for i := range inputs {
			tx.TxIn[i].Witness = wire.TxWitness{signatures[i].Serialize()}
		}
	}

	return nil
}

// spendWitnessKeyHash generates, and sets a valid witness for spending the
// passed pkScript with the specified input amount. The input amount *must*
// correspond to the output value of the previous pkScript, or else verification
// will fail since the new sighash digest algorithm defined in BIP0143 includes
// the input value in the sighash.
func spendWitnessKeyHash(txIn *wire.TxIn, pkScript []byte,
	inputValue int64, chainParams *chaincfg.Params, secrets SecretsSource,
	tx *wire.MsgTx, hashCache *txscript.TxSigHashes, idx int) error {

	// First obtain the key pair associated with this p2wkh address.
	_, addrs, _, err := txscript.ExtractPkScriptAddrs(pkScript,
		chainParams)
	if err != nil {
		return err
	}
	privKey, compressed, err := secrets.GetKey(addrs[0])
	if err != nil {
		return err
	}
	pubKey := privKey.PubKey()

	// Once we have the key pair, generate a p2wkh address type, respecting
	// the compression type of the generated key.
	var pubKeyHash []byte
	if compressed {
		pubKeyHash = btcutil.Hash160(pubKey.SerializeCompressed())
	} else {
		pubKeyHash = btcutil.Hash160(pubKey.SerializeUncompressed())
	}
	p2wkhAddr, err := btcutil.NewAddressWitnessPubKeyHash(pubKeyHash, chainParams)
	if err != nil {
		return err
	}

	// With the concrete address type, we can now generate the
	// corresponding witness program to be used to generate a valid witness
	// which will allow us to spend this output.
	witnessProgram, err := txscript.PayToAddrScript(p2wkhAddr)
	if err != nil {
		return err
	}
	witnessScript, err := txscript.WitnessSignature(tx, hashCache, idx,
		inputValue, witnessProgram, txscript.SigHashAll, privKey, true)
	if err != nil {
		return err
	}

	txIn.Witness = witnessScript

	return nil
}

// spendTaprootKey generates, and sets a valid witness for spending the passed
// pkScript with the specified input amount. The input amount *must*
// correspond to the output value of the previous pkScript, or else verification
// will fail since the new sighash digest algorithm defined in BIP0341 includes
// the input value in the sighash.
func spendTaprootKey(linearCombinations map[string]*crypto.LinearCombination, pkScript []byte, inputValue int64,
	params *chaincfg.Params, tx *wire.MsgTx, sigHashes *txscript.TxSigHashes, idx int,
) (*crypto.LinearSignDescriptor, *InputData, error) {

	// First obtain the key pair associated with this p2tr address. If the
	// pkScript is incorrect or derived from a different internal key or
	// with a script root, we simply won't find a corresponding private key
	// here.

	sigHash, err := txscript.CalcTaprootSignatureHash(
		sigHashes, txscript.SigHashDefault, tx, idx,
		txscript.NewCannedPrevOutputFetcher(pkScript, inputValue),
	)
	if err != nil {
		return nil, nil, nil
	}

	_, addrs, _, err := txscript.ExtractPkScriptAddrs(pkScript, params)
	if err != nil {
		return nil, nil, err
	}
	lc, ok := linearCombinations[addrs[0].String()]
	if !ok {
		return nil, nil, fmt.Errorf("key not found for address %v", addrs[0].String())
	}

	inputData := NewInputData(pkScript, inputValue, sigHashes, idx)
	descriptor := &crypto.LinearSignDescriptor{
		MsgHash: sigHash,
		LC:      lc,
	}

	return descriptor, inputData, nil
}

type TxData struct {
	SignatureData []byte
	Tx            *wire.MsgTx
	InputData     []*InputData
}

func NewTxData(signatureData []byte, tx *wire.MsgTx, inputData []*InputData) *TxData {
	return &TxData{
		SignatureData: signatureData,
		Tx:            tx,
		InputData:     inputData,
	}
}

type InputData struct {
	PkScript   []byte
	InputValue int64
	SigHashes  *txscript.TxSigHashes
	Idx        int
}

func NewInputData(pkScript []byte, inputValue int64, sigHashes *txscript.TxSigHashes, idx int) *InputData {
	return &InputData{
		PkScript:   pkScript,
		InputValue: inputValue,
		SigHashes:  sigHashes,
		Idx:        idx,
	}
}

func NewTxDataWithSignatureDataOnly(signatureData []byte) *TxData {
	return &TxData{
		SignatureData: signatureData,
	}
}

func SerializeTxData(txData *TxData) ([]byte, error) {
	var buffer bytes.Buffer
	encoder := gob.NewEncoder(&buffer)

	err := encoder.Encode(txData)
	if err != nil {
		return nil, fmt.Errorf("failed to encode TxData: %w", err)
	}

	return buffer.Bytes(), nil
}

func DeserializeTxData(data []byte) (*TxData, error) {
	var txData TxData
	decoder := gob.NewDecoder(bytes.NewBuffer(data))
	err := decoder.Decode(&txData)
	if err != nil {
		return nil, fmt.Errorf("failed to decode TxData: %w", err)
	}
	return &txData, nil
}

// spendNestedWitnessPubKey generates both a sigScript, and valid witness for
// spending the passed pkScript with the specified input amount. The generated
// sigScript is the version 0 p2wkh witness program corresponding to the queried
// key. The witness stack is identical to that of one which spends a regular
// p2wkh output. The input amount *must* correspond to the output value of the
// previous pkScript, or else verification will fail since the new sighash
// digest algorithm defined in BIP0143 includes the input value in the sighash.
func spendNestedWitnessPubKeyHash(txIn *wire.TxIn, pkScript []byte,
	inputValue int64, chainParams *chaincfg.Params, secrets SecretsSource,
	tx *wire.MsgTx, hashCache *txscript.TxSigHashes, idx int) error {

	// First we need to obtain the key pair related to this p2sh output.
	_, addrs, _, err := txscript.ExtractPkScriptAddrs(pkScript,
		chainParams)
	if err != nil {
		return err
	}
	privKey, compressed, err := secrets.GetKey(addrs[0])
	if err != nil {
		return err
	}
	pubKey := privKey.PubKey()

	var pubKeyHash []byte
	if compressed {
		pubKeyHash = btcutil.Hash160(pubKey.SerializeCompressed())
	} else {
		pubKeyHash = btcutil.Hash160(pubKey.SerializeUncompressed())
	}

	// Next, we'll generate a valid sigScript that'll allow us to spend
	// the p2sh output. The sigScript will contain only a single push of
	// the p2wkh witness program corresponding to the matching public key
	// of this address.
	p2wkhAddr, err := btcutil.NewAddressWitnessPubKeyHash(pubKeyHash, chainParams)
	if err != nil {
		return err
	}
	witnessProgram, err := txscript.PayToAddrScript(p2wkhAddr)
	if err != nil {
		return err
	}
	bldr := txscript.NewScriptBuilder()
	bldr.AddData(witnessProgram)
	sigScript, err := bldr.Script()
	if err != nil {
		return err
	}
	txIn.SignatureScript = sigScript

	// With the sigScript in place, we'll next generate the proper witness
	// that'll allow us to spend the p2wkh output.
	witnessScript, err := txscript.WitnessSignature(tx, hashCache, idx,
		inputValue, witnessProgram, txscript.SigHashAll, privKey, compressed)
	if err != nil {
		return err
	}

	txIn.Witness = witnessScript

	return nil
}

// AddAllInputScripts modifies an authored transaction by adding inputs scripts
// for each input of an authored transaction.  Private keys and redeem scripts
// are looked up using a SecretsSource based on the previous output script.
func (tx *AuthoredTx) AddAllInputScripts(signer frost.Signer, linearCombinations map[string]*crypto.LinearCombination,
	data []byte, secrets SecretsSource) error {
	return AddAllInputScripts(
		signer, linearCombinations, data, tx.Tx, tx.PrevScripts, tx.PrevInputValues, secrets,
	)
}

// TXPrevOutFetcher creates a txscript.PrevOutFetcher from a given slice of
// previous pk scripts and input values.
func TXPrevOutFetcher(tx *wire.MsgTx, prevPkScripts [][]byte,
	inputValues []btcutil.Amount) (*txscript.MultiPrevOutFetcher, error) {

	if len(tx.TxIn) != len(prevPkScripts) {
		return nil, errors.New("tx.TxIn and prevPkScripts slices " +
			"must have equal length")
	}
	if len(tx.TxIn) != len(inputValues) {
		return nil, errors.New("tx.TxIn and inputValues slices " +
			"must have equal length")
	}

	fetcher := txscript.NewMultiPrevOutFetcher(nil)
	for idx, txin := range tx.TxIn {
		fetcher.AddPrevOut(txin.PreviousOutPoint, &wire.TxOut{
			Value:    int64(inputValues[idx]),
			PkScript: prevPkScripts[idx],
		})
	}

	return fetcher, nil
}
