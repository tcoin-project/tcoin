package block

import (
	"crypto/ed25519"
	"encoding/binary"
	"errors"
	"io"

	"github.com/mcfx/tcoin/storage"
)

type Transaction struct {
	TxType       byte
	SenderPubkey PubkeyType
	SenderSig    SigType
	Receiver     AddressType
	Value        uint64
	GasLimit     uint64
	Fee          uint64
	Nonce        uint64
	Data         []byte
}

func DecodeTx(r io.Reader) (*Transaction, error) {
	tx := &Transaction{}
	buf := make([]byte, 8)
	_, err := r.Read(buf[:1])
	if err != nil {
		return tx, err
	}
	tx.TxType = buf[0]
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
	_, err = io.ReadFull(r, buf)
	if err != nil {
		return nil, err
	}
	tx.Value = binary.LittleEndian.Uint64(buf)
	_, err = io.ReadFull(r, buf)
	if err != nil {
		return nil, err
	}
	tx.GasLimit = binary.LittleEndian.Uint64(buf)
	_, err = io.ReadFull(r, buf)
	if err != nil {
		return nil, err
	}
	tx.Fee = binary.LittleEndian.Uint64(buf)
	_, err = io.ReadFull(r, buf)
	if err != nil {
		return nil, err
	}
	tx.Nonce = binary.LittleEndian.Uint64(buf)
	_, err = io.ReadFull(r, buf)
	if err != nil {
		return nil, err
	}
	dataLen := binary.LittleEndian.Uint64(buf)
	tx.Data = make([]byte, dataLen)
	_, err = io.ReadFull(r, tx.Data)
	if err != nil {
		return nil, err
	}
	return tx, nil
}

func EncodeTx(w io.Writer, tx *Transaction) error {
	buf := make([]byte, 8)
	buf[0] = tx.TxType
	_, err := w.Write(buf[:1])
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
	binary.LittleEndian.PutUint64(buf, tx.Value)
	_, err = w.Write(buf)
	if err != nil {
		return err
	}
	binary.LittleEndian.PutUint64(buf, tx.GasLimit)
	_, err = w.Write(buf)
	if err != nil {
		return err
	}
	binary.LittleEndian.PutUint64(buf, tx.Fee)
	_, err = w.Write(buf)
	if err != nil {
		return err
	}
	binary.LittleEndian.PutUint64(buf, tx.Nonce)
	_, err = w.Write(buf)
	if err != nil {
		return err
	}
	binary.LittleEndian.PutUint64(buf, uint64(len(tx.Data)))
	_, err = w.Write(buf)
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
