package ethchain

import (
	_ "fmt"
	"math/big"
	"testing"

	"github.com/geoffreyhinton/goeth_self_training/ethdb"
	"github.com/geoffreyhinton/goeth_self_training/ethutil"
)

func TestVm(t *testing.T) {
	InitFees()
	ethutil.ReadConfig("")

	db, _ := ethdb.NewMemDatabase()
	ethutil.Config.Db = db
	bm := NewBlockManager(nil)

	block := bm.bc.genesisBlock
	ctrct := NewTransaction(ContractAddr, big.NewInt(200000000), []string{
		"PUSH",
		"1",
		"PUSH",
		"2",

		"STOP",
	})
	bm.ApplyTransactions(block, []*Transaction{ctrct})
}
