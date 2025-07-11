package ethchain

import (
	"bytes"
	"container/list"
	"errors"
	"fmt"
	"log"
	"math/big"
	"sync"

	"github.com/geoffreyhinton/goeth_self_training/ethutil"
	"github.com/geoffreyhinton/goeth_self_training/ethwire"
)

const (
	txPoolQueueSize = 50
)

type TxPoolHook chan *Transaction

func FindTx(pool *list.List, finder func(*Transaction, *list.Element) bool) *Transaction {
	for e := pool.Front(); e != nil; e = e.Next() {
		if tx, ok := e.Value.(*Transaction); ok {
			if finder(tx, e) {
				return tx
			}
		}
	}

	return nil
}

type PublicSpeaker interface {
	Broadcast(msgType ethwire.MsgType, data []interface{})
}

// The tx pool a thread safe transaction pool handler. In order to
// guarantee a non blocking pool we use a queue channel which can be
// independently read without needing access to the actual pool. If the
// pool is being drained or synced for whatever reason the transactions
// will simple queue up and handled when the mutex is freed.
type TxPool struct {
	//server *Server
	Speaker PublicSpeaker
	// The mutex for accessing the Tx pool.
	mutex sync.Mutex
	// Queueing channel for reading and writing incoming
	// transactions to
	queueChan chan *Transaction
	// Quiting channel
	quit chan bool
	// The actual pool
	pool *list.List

	BlockManager *BlockManager

	Hook TxPoolHook
}

func NewTxPool() *TxPool {
	return &TxPool{
		//server:    s,
		mutex:     sync.Mutex{},
		pool:      list.New(),
		queueChan: make(chan *Transaction, txPoolQueueSize),
		quit:      make(chan bool),
	}
}

// Blocking function. Don't use directly. Use QueueTransaction instead
func (pool *TxPool) addTransaction(tx *Transaction) {
	pool.mutex.Lock()
	pool.pool.PushBack(tx)
	pool.mutex.Unlock()

	// Broadcast the transaction to the rest of the peers
	pool.Speaker.Broadcast(ethwire.MsgTxTy, []interface{}{tx.RlpData()})
}

// Process transaction validates the Tx and processes funds from the
// sender to the recipient.
func (pool *TxPool) ProcessTransaction(tx *Transaction, block *Block) (err error) {
	log.Printf("[TXPL] Processing Tx %x\n", tx.Hash())

	defer func() {
		if r := recover(); r != nil {
			log.Println(r)
			err = fmt.Errorf("%v", r)
		}
	}()
	// Get the sender
	sender := block.GetAddr(tx.Sender())

	// Make sure there's enough in the sender's account. Having insufficient
	// funds won't invalidate this transaction but simple ignores it.
	totAmount := new(big.Int).Add(tx.Value, new(big.Int).Mul(TxFee, TxFeeRat))
	if sender.Amount.Cmp(totAmount) < 0 {
		return errors.New("Insufficient amount in sender's account")
	}

	if sender.Nonce != tx.Nonce {
		if ethutil.Config.Debug {
			return fmt.Errorf("Invalid nonce %d(%d) continueing anyway", tx.Nonce, sender.Nonce)
		} else {
			return fmt.Errorf("Invalid nonce %d(%d)", tx.Nonce, sender.Nonce)
		}
	}

	// Get the receiver
	receiver := block.GetAddr(tx.Recipient)
	sender.Nonce += 1

	// Send Tx to self
	if bytes.Compare(tx.Recipient, tx.Sender()) == 0 {
		// Subtract the fee
		sender.Amount.Sub(sender.Amount, new(big.Int).Mul(TxFee, TxFeeRat))
	} else {
		// Subtract the amount from the senders account
		sender.Amount.Sub(sender.Amount, totAmount)

		// Add the amount to receivers account which should conclude this transaction
		receiver.Amount.Add(receiver.Amount, tx.Value)

		block.UpdateAddr(tx.Recipient, receiver)
	}

	block.UpdateAddr(tx.Sender(), sender)

	return
}

func (pool *TxPool) ValidateTransaction(tx *Transaction) error {
	// Get the last block so we can retrieve the sender and receiver from
	// the merkle trie
	block := pool.BlockManager.BlockChain().CurrentBlock
	// Something has gone horribly wrong if this happens
	if block == nil {
		return errors.New("No last block on the block chain")
	}

	// Get the sender
	sender := block.GetAddr(tx.Sender())

	totAmount := new(big.Int).Add(tx.Value, new(big.Int).Mul(TxFee, TxFeeRat))
	// Make sure there's enough in the sender's account. Having insufficient
	// funds won't invalidate this transaction but simple ignores it.
	if sender.Amount.Cmp(totAmount) < 0 {
		return fmt.Errorf("Insufficient amount in sender's (%x) account", tx.Sender())
	}

	// Increment the nonce making each tx valid only once to prevent replay
	// attacks

	return nil
}

func (pool *TxPool) queueHandler() {
out:
	for {
		select {
		case tx := <-pool.queueChan:
			hash := tx.Hash()
			foundTx := FindTx(pool.pool, func(tx *Transaction, e *list.Element) bool {
				return bytes.Compare(tx.Hash(), hash) == 0
			})

			if foundTx != nil {
				break
			}

			// Validate the transaction
			err := pool.ValidateTransaction(tx)
			if err != nil {
				if ethutil.Config.Debug {
					log.Println("Validating Tx failed", err)
				}
			} else {
				// Call blocking version. At this point it
				// doesn't matter since this is a goroutine
				pool.addTransaction(tx)

				if pool.Hook != nil {
					pool.Hook <- tx
				}
			}
		case <-pool.quit:
			break out
		}
	}
}

func (pool *TxPool) QueueTransaction(tx *Transaction) {
	pool.queueChan <- tx
}

func (pool *TxPool) Flush() []*Transaction {
	pool.mutex.Lock()
	defer pool.mutex.Unlock()

	txList := make([]*Transaction, pool.pool.Len())
	i := 0
	for e := pool.pool.Front(); e != nil; e = e.Next() {
		if tx, ok := e.Value.(*Transaction); ok {
			txList[i] = tx
		}

		i++
	}

	// Recreate a new list all together
	// XXX Is this the fastest way?
	pool.pool = list.New()

	return txList
}

func (pool *TxPool) Start() {
	go pool.queueHandler()
}

func (pool *TxPool) Stop() {
	log.Println("[TXP] Stopping...")

	close(pool.quit)

	pool.Flush()
}
