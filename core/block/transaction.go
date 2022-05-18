package block

import (
	"crypto/ed25519"
	"encoding/binary"
	"errors"
	"io"

	"github.com/mcfx/tcoin/storage"
	"github.com/mcfx/tcoin/utils"
)

type Transaction struct {
	TxType       byte        `json:"tx_type"`
	SenderPubkey PubkeyType  `json:"sender_pubkey"`
	SenderSig    SigType     `json:"sender_sig"`
	Receiver     AddressType `json:"receiver"`
	Value        uint64      `json:"value"`
	GasLimit     uint64      `json:"gas_limit"`
	Fee          uint64      `json:"fee"`
	Nonce        uint64      `json:"nonce"`
	Data         []byte      `json:"data"`
}

func DecodeTx(r utils.Reader) (*Transaction, error) {
	var err error
	tx := &Transaction{}
	tx.TxType, err = r.ReadByte()
	if err != nil {
		return tx, err
	}
	_, err = io.ReadFull(r, tx.SenderPubkey[:])
	if err != nil {
		return nil, err
	}
	_, err = io.ReadFull(r, tx.SenderSig[:])
	if err != nil {
		return nil, err
	}
	_, err = io.ReadFull(r, tx.Receiver[:])
	if err != nil {
		return nil, err
	}
	tx.Value, err = binary.ReadUvarint(r)
	if err != nil {
		return nil, err
	}
	tx.GasLimit, err = binary.ReadUvarint(r)
	if err != nil {
		return nil, err
	}
	tx.Fee, err = binary.ReadUvarint(r)
	if err != nil {
		return nil, err
	}
	tx.Nonce, err = binary.ReadUvarint(r)
	if err != nil {
		return nil, err
	}
	dataLen, err := binary.ReadUvarint(r)
	if err != nil {
		return nil, err
	}
	if dataLen > (1 << 20) {
		return nil, errors.New("invalid data length")
	}
	tx.Data = make([]byte, dataLen)
	_, err = io.ReadFull(r, tx.Data)
	if err != nil {
		return nil, err
	}
	return tx, nil
}

func EncodeTx(w utils.Writer, tx *Transaction) error {
	err := w.WriteByte(tx.TxType)
	if err != nil {
		return err
	}
	_, err = w.Write(tx.SenderPubkey[:])
	if err != nil {
		return err
	}
	_, err = w.Write(tx.SenderSig[:])
	if err != nil {
		return err
	}
	_, err = w.Write(tx.Receiver[:])
	if err != nil {
		return err
	}
	buf := make([]byte, binary.MaxVarintLen64*5)
	cur := 0
	cur += binary.PutUvarint(buf[cur:], tx.Value)
	cur += binary.PutUvarint(buf[cur:], tx.GasLimit)
	cur += binary.PutUvarint(buf[cur:], tx.Fee)
	cur += binary.PutUvarint(buf[cur:], tx.Nonce)
	cur += binary.PutUvarint(buf[cur:], uint64(len(tx.Data)))
	_, err = w.Write(buf[:cur])
	if err != nil {
		return err
	}
	_, err = w.Write(tx.Data)
	if err != nil {
		return err
	}
	return nil
}

func (tx *Transaction) prepareSignData() []byte {
	sbuf := make([]byte, AddressLen+8*4)
	copy(sbuf[:AddressLen], tx.Receiver[:])
	binary.BigEndian.PutUint64(sbuf[AddressLen:AddressLen+8], tx.Value)
	binary.BigEndian.PutUint64(sbuf[AddressLen+8:AddressLen+16], tx.GasLimit)
	binary.BigEndian.PutUint64(sbuf[AddressLen+16:AddressLen+24], tx.Fee)
	binary.BigEndian.PutUint64(sbuf[AddressLen+24:AddressLen+32], tx.Nonce)
	return append(sbuf, tx.Data...)
}

func (tx *Transaction) Sign(privKey PrivkeyType) {
	data := tx.prepareSignData()
	copy(tx.SenderSig[:], ed25519.Sign(privKey[:], data))
}

func ExecuteTx(tx *Transaction, s *storage.Slice) error {
	if tx.TxType != 1 {
		return errors.New("wrong tx type")
	}
	sbuf := tx.prepareSignData()
	if !ed25519.Verify(tx.SenderPubkey[:], sbuf, tx.SenderSig[:]) {
		return errors.New("signature mismatch")
	}
	senderAddr := PubkeyToAddress(tx.SenderPubkey)
	senderAccount := GetAccountInfo(s, senderAddr)
	totalValue := tx.Value + tx.Fee
	if totalValue < tx.Value {
		return errors.New("integer overflow")
	}
	if senderAccount.Balance < totalValue {
		return errors.New("balance not enought")
	}
	if senderAccount.Nonce != tx.Nonce {
		return errors.New("nonce mismatch")
	}
	// todo: smart contracts
	senderAccount.Balance -= totalValue
	senderAccount.Nonce++
	SetAccountInfo(s, senderAddr, senderAccount)
	receiverAccount := GetAccountInfo(s, tx.Receiver)
	receiverAccount.Balance += tx.Value
	SetAccountInfo(s, tx.Receiver, receiverAccount)
	return nil
}
