package ethchain

import (
	"fmt"
	"math/big"
	"time"

	"github.com/geoffreyhinton/goeth_self_training/ethutil"
)

type BlockInfo struct {
	Number uint64
	Hash   []byte
	Parent []byte
}

func (bi *BlockInfo) RlpDecode(data []byte) {
	decoder := ethutil.NewValueFromBytes(data)

	bi.Number = decoder.Get(0).Uint()
	bi.Hash = decoder.Get(1).Bytes()
	bi.Parent = decoder.Get(2).Bytes()
}

func (bi *BlockInfo) RlpEncode() []byte {
	return ethutil.Encode([]interface{}{bi.Number, bi.Hash, bi.Parent})
}

type Block struct {
	// Hash to the previous block
	PrevHash []byte
	// Uncles of this block
	Uncles   []*Block
	UncleSha []byte
	// The coin base address
	Coinbase []byte
	// Block Trie state
	state *ethutil.Trie
	// Difficulty for the current block
	Difficulty *big.Int
	// Creation time
	Time int64
	// Extra data
	Extra string
	// Block Nonce for verification
	Nonce []byte
	// List of transactions and/or contracts
	transactions []*Transaction
	TxSha        []byte

	contractStates map[string]*ethutil.Trie
}

// New block takes a raw encoded string
// XXX DEPRICATED
func NewBlockFromData(raw []byte) *Block {
	return NewBlockFromBytes(raw)
}

func NewBlockFromBytes(raw []byte) *Block {
	block := &Block{}
	block.RlpDecode(raw)

	return block
}

// New block takes a raw encoded string
func NewBlockFromRlpValue(rlpValue *ethutil.Value) *Block {
	block := &Block{}
	block.RlpValueDecode(rlpValue)

	return block
}

func CreateBlock(root interface{},
	prevHash []byte,
	base []byte,
	Difficulty *big.Int,
	Nonce []byte,
	extra string,
	txes []*Transaction) *Block {

	block := &Block{
		// Slice of transactions to include in this block
		transactions:   txes,
		PrevHash:       prevHash,
		Coinbase:       base,
		Difficulty:     Difficulty,
		Nonce:          Nonce,
		Time:           time.Now().Unix(),
		Extra:          extra,
		UncleSha:       EmptyShaList,
		contractStates: make(map[string]*ethutil.Trie),
	}
	block.SetTransactions(txes)
	block.SetUncles([]*Block{})

	block.state = ethutil.NewTrie(ethutil.Config.Db, root)

	for _, tx := range txes {
		block.MakeContract(tx)
	}

	return block
}

// Returns a hash of the block
func (block *Block) Hash() []byte {
	return ethutil.Sha3Bin(block.Value().Encode())
}

func (block *Block) HashNoNonce() []byte {
	return ethutil.Sha3Bin(ethutil.Encode([]interface{}{block.PrevHash, block.UncleSha, block.Coinbase, block.state.Root, block.TxSha, block.Difficulty, block.Time, block.Extra}))
}

func (block *Block) PrintHash() {
	fmt.Println(block)
	fmt.Println(ethutil.NewValue(ethutil.Encode([]interface{}{block.PrevHash, block.UncleSha, block.Coinbase, block.state.Root, block.TxSha, block.Difficulty, block.Time, block.Extra, block.Nonce})))
}

func (block *Block) State() *ethutil.Trie {
	return block.state
}

func (block *Block) Transactions() []*Transaction {
	return block.transactions
}

func (block *Block) GetContract(addr []byte) *Contract {
	data := block.state.Get(string(addr))
	if data == "" {
		return nil
	}

	contract := &Contract{}
	contract.RlpDecode([]byte(data))

	cachedState := block.contractStates[string(addr)]
	if cachedState != nil {
		contract.state = cachedState
	} else {
		block.contractStates[string(addr)] = contract.state
	}

	return contract
}
func (block *Block) UpdateContract(addr []byte, contract *Contract) {
	// Make sure the state is synced
	//contract.State().Sync()

	block.state.Update(string(addr), string(contract.RlpEncode()))
}

func (block *Block) GetAddr(addr []byte) *Address {
	var address *Address

	data := block.State().Get(string(addr))
	if data == "" {
		address = NewAddress(big.NewInt(0))
	} else {
		address = NewAddressFromData([]byte(data))
	}

	return address
}
func (block *Block) UpdateAddr(addr []byte, address *Address) {
	block.state.Update(string(addr), string(address.RlpEncode()))
}

func (block *Block) PayFee(addr []byte, fee *big.Int) bool {
	contract := block.GetContract(addr)
	// If we can't pay the fee return
	if contract == nil || contract.Amount.Cmp(fee) < 0 /* amount < fee */ {
		fmt.Println("Contract has insufficient funds", contract.Amount, fee)

		return false
	}

	base := new(big.Int)
	contract.Amount = base.Sub(contract.Amount, fee)
	block.state.Update(string(addr), string(contract.RlpEncode()))

	data := block.state.Get(string(block.Coinbase))

	// Get the ether (Coinbase) and add the fee (gief fee to miner)
	ether := NewAddressFromData([]byte(data))

	base = new(big.Int)
	ether.Amount = base.Add(ether.Amount, fee)

	block.state.Update(string(block.Coinbase), string(ether.RlpEncode()))

	return true
}

func (block *Block) BlockInfo() BlockInfo {
	bi := BlockInfo{}
	data, _ := ethutil.Config.Db.Get(append(block.Hash(), []byte("Info")...))
	bi.RlpDecode(data)

	return bi
}

// Sync the block's state and contract respectively
func (block *Block) Sync() {
	// Sync all contracts currently in cache
	for _, val := range block.contractStates {
		val.Sync()
	}
	// Sync the block state itself
	block.state.Sync()
}

func (block *Block) Undo() {
	// Sync all contracts currently in cache
	for _, val := range block.contractStates {
		val.Undo()
	}
	// Sync the block state itself
	block.state.Undo()
}

func (block *Block) MakeContract(tx *Transaction) {
	contract := MakeContract(tx, NewState(block.state))

	if contract != nil {
		block.contractStates[string(tx.Hash()[12:])] = contract.state
	}
}

// ///// Block Encoding
func (block *Block) encodedUncles() interface{} {
	uncles := make([]interface{}, len(block.Uncles))
	for i, uncle := range block.Uncles {
		uncles[i] = uncle.RlpEncode()
	}

	return uncles
}

func (block *Block) encodedTxs() interface{} {
	// Marshal the transactions of this block
	encTx := make([]interface{}, len(block.transactions))
	for i, tx := range block.transactions {
		// Cast it to a string (safe)
		encTx[i] = tx.RlpData()
	}

	return encTx
}

func (block *Block) rlpTxs() interface{} {
	// Marshal the transactions of this block
	encTx := make([]interface{}, len(block.transactions))
	for i, tx := range block.transactions {
		// Cast it to a string (safe)
		encTx[i] = tx.RlpData()
	}

	return encTx
}

func (block *Block) rlpUncles() interface{} {
	// Marshal the transactions of this block
	uncles := make([]interface{}, len(block.Uncles))
	for i, uncle := range block.Uncles {
		// Cast it to a string (safe)
		uncles[i] = uncle.header()
	}

	return uncles
}

func (block *Block) SetUncles(uncles []*Block) {
	block.Uncles = uncles

	// Sha of the concatenated uncles
	block.UncleSha = ethutil.Sha3Bin(ethutil.Encode(block.rlpUncles()))
}

func (block *Block) SetTransactions(txs []*Transaction) {
	block.transactions = txs

	block.TxSha = ethutil.Sha3Bin(ethutil.Encode(block.rlpTxs()))
}

func (block *Block) Value() *ethutil.Value {
	return ethutil.NewValue([]interface{}{block.header(), block.rlpTxs(), block.rlpUncles()})
}

func (block *Block) RlpEncode() []byte {
	// Encode a slice interface which contains the header and the list of
	// transactions.
	return block.Value().Encode()
}

func (block *Block) RlpDecode(data []byte) {
	rlpValue := ethutil.NewValueFromBytes(data)
	block.RlpValueDecode(rlpValue)
}

func (block *Block) RlpValueDecode(decoder *ethutil.Value) {
	header := decoder.Get(0)

	block.PrevHash = header.Get(0).Bytes()
	block.UncleSha = header.Get(1).Bytes()
	block.Coinbase = header.Get(2).Bytes()
	block.state = ethutil.NewTrie(ethutil.Config.Db, header.Get(3).Val)
	block.TxSha = header.Get(4).Bytes()
	block.Difficulty = header.Get(5).BigInt()
	block.Time = int64(header.Get(6).BigInt().Uint64())
	block.Extra = header.Get(7).Str()
	block.Nonce = header.Get(8).Bytes()
	block.contractStates = make(map[string]*ethutil.Trie)

	// Tx list might be empty if this is an uncle. Uncles only have their
	// header set.
	if decoder.Get(1).IsNil() == false { // Yes explicitness
		txes := decoder.Get(1)
		block.transactions = make([]*Transaction, txes.Len())
		for i := 0; i < txes.Len(); i++ {
			tx := NewTransactionFromValue(txes.Get(i))

			block.transactions[i] = tx
		}

	}

	if decoder.Get(2).IsNil() == false { // Yes explicitness
		uncles := decoder.Get(2)
		block.Uncles = make([]*Block, uncles.Len())
		for i := 0; i < uncles.Len(); i++ {
			block.Uncles[i] = NewUncleBlockFromValue(uncles.Get(i))
		}
	}

}

func NewUncleBlockFromValue(header *ethutil.Value) *Block {
	block := &Block{}

	block.PrevHash = header.Get(0).Bytes()
	block.UncleSha = header.Get(1).Bytes()
	block.Coinbase = header.Get(2).Bytes()
	block.state = ethutil.NewTrie(ethutil.Config.Db, header.Get(3).Val)
	block.TxSha = header.Get(4).Bytes()
	block.Difficulty = header.Get(5).BigInt()
	block.Time = int64(header.Get(6).BigInt().Uint64())
	block.Extra = header.Get(7).Str()
	block.Nonce = header.Get(8).Bytes()

	return block
}

func (block *Block) String() string {
	return fmt.Sprintf("Block(%x):\nPrevHash:%x\nUncleSha:%x\nCoinbase:%x\nRoot:%x\nTxSha:%x\nDiff:%v\nTime:%d\nNonce:%x\nTxs:%d\n", block.Hash(), block.PrevHash, block.UncleSha, block.Coinbase, block.state.Root, block.TxSha, block.Difficulty, block.Time, block.Nonce, len(block.transactions))
}

// ////////// UNEXPORTED /////////////////
func (block *Block) header() []interface{} {
	return []interface{}{
		// Sha of the previous block
		block.PrevHash,
		// Sha of uncles
		block.UncleSha,
		// Coinbase address
		block.Coinbase,
		// root state
		block.state.Root,
		// Sha of tx
		block.TxSha,
		// Current block Difficulty
		block.Difficulty,
		// Time the block was found?
		block.Time,
		// Extra data
		block.Extra,
		// Block's Nonce for validation
		block.Nonce,
	}
}
