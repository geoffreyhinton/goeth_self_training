package ethchain

import (
	"testing"

	"github.com/geoffreyhinton/goeth_self_training/ethdb"
	"github.com/geoffreyhinton/goeth_self_training/ethutil"
)

func TestSnapshot(t *testing.T) {
	ethutil.ReadConfig("", ethutil.LogStd, "")

	db, _ := ethdb.NewMemDatabase()
	state := NewState(ethutil.NewTrie(db, ""))

	stateObject := NewContract([]byte("aa"), ethutil.Big1, ZeroHash256)
	state.UpdateStateObject(stateObject)
	stateObject.SetStorage(ethutil.Big("0"), ethutil.NewValue(42))

	snapshot := state.Snapshot()

	stateObject = state.GetStateObject([]byte("aa"))
	stateObject.SetStorage(ethutil.Big("0"), ethutil.NewValue(43))

	state.Revert(snapshot)

	stateObject = state.GetStateObject([]byte("aa"))
	if !stateObject.GetStorage(ethutil.Big("0")).Cmp(ethutil.NewValue(42)) {
		t.Error("Expected storage 0 to be 42")
	}
}
