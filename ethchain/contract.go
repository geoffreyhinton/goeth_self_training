package ethchain

import (
	"math/big"

	"github.com/geoffreyhinton/goeth_self_training/ethutil"
)

type Contract struct {
	Amount *big.Int
	Nonce  uint64
	state  *ethutil.Trie
}

func NewContract(Amount *big.Int, root []byte) *Contract {
	contract := &Contract{Amount: Amount, Nonce: 0}
	contract.state = ethutil.NewTrie(ethutil.Config.Db, string(root))

	return contract
}

func (c *Contract) RlpEncode() []byte {
	return ethutil.Encode([]interface{}{c.Amount, c.Nonce, c.state.Root})
}

func (c *Contract) RlpDecode(data []byte) {
	decoder := ethutil.NewValueFromBytes(data)

	c.Amount = decoder.Get(0).BigInt()
	c.Nonce = decoder.Get(1).Uint()
	c.state = ethutil.NewTrie(ethutil.Config.Db, decoder.Get(2).Interface())
}

func (c *Contract) Addr(addr []byte) *ethutil.Value {
	return ethutil.NewValueFromBytes([]byte(c.state.Get(string(addr))))
}

func (c *Contract) SetAddr(addr []byte, value interface{}) {
	c.state.Update(string(addr), string(ethutil.NewValue(value).Encode()))
}

func (c *Contract) State() *ethutil.Trie {
	return c.state
}

func (c *Contract) GetMem(num int) *ethutil.Value {
	nb := ethutil.BigToBytes(big.NewInt(int64(num)), 256)

	return c.Addr(nb)
}

func MakeContract(tx *Transaction, state *State) *Contract {
	// Create contract if there's no recipient
	if tx.IsContract() {
		addr := tx.Hash()[12:]

		value := tx.Value
		contract := NewContract(value, []byte(""))
		state.trie.Update(string(addr), string(contract.RlpEncode()))
		for i, val := range tx.Data {
			if len(val) > 0 {
				bytNum := ethutil.BigToBytes(big.NewInt(int64(i)), 256)
				contract.state.Update(string(bytNum), string(ethutil.Encode(val)))
			}
		}
		state.trie.Update(string(addr), string(contract.RlpEncode()))

		return contract
	}

	return nil
}

type Address struct {
	Amount *big.Int
	Nonce  uint64
}

func NewAddress(amount *big.Int) *Address {
	return &Address{Amount: amount, Nonce: 0}
}

func NewAddressFromData(data []byte) *Address {
	address := &Address{}
	address.RlpDecode(data)

	return address
}

func (a *Address) AddFee(fee *big.Int) {
	a.Amount.Add(a.Amount, fee)
}

func (a *Address) RlpEncode() []byte {
	return ethutil.Encode([]interface{}{a.Amount, a.Nonce})
}

func (a *Address) RlpDecode(data []byte) {
	decoder := ethutil.NewValueFromBytes(data)

	a.Amount = decoder.Get(0).BigInt()
	a.Nonce = decoder.Get(1).Uint()
}
