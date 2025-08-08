package ethchain

// import (
// 	_ "fmt"
// 	"math/big"
// 	"testing"

// 	"github.com/geoffreyhinton/goeth_self_training/ethdb"
// 	"github.com/geoffreyhinton/goeth_self_training/ethutil"
// )

// func TestVm(t *testing.T) {
// 	InitFees()
// 	ethutil.ReadConfig("")

// 	db, _ := ethdb.NewMemDatabase()
// 	ethutil.Config.Db = db
// 	bm := NewBlockManager(nil)

// 	block := bm.bc.genesisBlock
// 	script := Compile([]string{
// 		"PUSH",
// 		"1",
// 		"PUSH",
// 		"2",
// 	})
// 	tx := NewTransaction(ContractAddr, big.NewInt(200000000), script)
// 	addr := tx.Hash()[12:]
// 	bm.ApplyTransactions(block, []*Transaction{tx})

// 	tx2 := NewTransaction(addr, big.NewInt(1e17), nil)
// 	tx2.Sign([]byte("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"))
// 	bm.ApplyTransactions(block, []*Transaction{tx2})
// }
