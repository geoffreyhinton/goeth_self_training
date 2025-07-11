package ethchain

import (
	"bytes"
	"fmt"
	"log"
	"math"
	"math/big"

	"github.com/geoffreyhinton/goeth_self_training/ethutil"
	"github.com/obscuren/secp256k1-go"
)

type Vm struct {
	txPool *TxPool
	// Stack for processing contracts
	stack *Stack
	// non-persistent key/value memory storage
	mem map[string]*big.Int

	vars RuntimeVars
}

type RuntimeVars struct {
	address     []byte
	blockNumber uint64
	sender      []byte
	prevHash    []byte
	coinbase    []byte
	time        int64
	diff        *big.Int
	txValue     *big.Int
	txData      []string
}

func (vm *Vm) Process(contract *Contract, state *State, vars RuntimeVars) {
	vm.mem = make(map[string]*big.Int)
	vm.stack = NewStack()

	addr := vars.address // tx.Hash()[12:]
	// Instruction pointer
	pc := 0

	if contract == nil {
		fmt.Println("Contract not found")
		return
	}

	Pow256 := ethutil.BigPow(2, 256)

	if ethutil.Config.Debug {
		fmt.Printf("#   op\n")
	}

	stepcount := 0
	totalFee := new(big.Int)

out:
	for {
		stepcount++
		// The base big int for all calculations. Use this for any results.
		base := new(big.Int)
		val := contract.GetMem(pc)
		//fmt.Printf("%x = %d, %v %x\n", r, len(r), v, nb)
		op := OpCode(val.Uint())

		var fee *big.Int = new(big.Int)
		var fee2 *big.Int = new(big.Int)
		if stepcount > 16 {
			fee.Add(fee, StepFee)
		}

		// Calculate the fees
		switch op {
		case oSSTORE:
			y, x := vm.stack.Peekn()
			val := contract.Addr(ethutil.BigToBytes(x, 256))
			if val.IsEmpty() && len(y.Bytes()) > 0 {
				fee2.Add(DataFee, StoreFee)
			} else {
				fee2.Sub(DataFee, StoreFee)
			}
		case oSLOAD:
			fee.Add(fee, StoreFee)
		case oEXTRO, oBALANCE:
			fee.Add(fee, ExtroFee)
		case oSHA256, oRIPEMD160, oECMUL, oECADD, oECSIGN, oECRECOVER, oECVALID:
			fee.Add(fee, CryptoFee)
		case oMKTX:
			fee.Add(fee, ContractFee)
		}

		tf := new(big.Int).Add(fee, fee2)
		if contract.Amount.Cmp(tf) < 0 {
			fmt.Println("Insufficient fees to continue running the contract", tf, contract.Amount)
			break
		}
		// Add the fee to the total fee. It's subtracted when we're done looping
		totalFee.Add(totalFee, tf)

		if ethutil.Config.Debug {
			fmt.Printf("%-3d %-4s", pc, op.String())
		}

		switch op {
		case oSTOP:
			fmt.Println("")
			break out
		case oADD:
			x, y := vm.stack.Popn()
			// (x + y) % 2 ** 256
			base.Add(x, y)
			base.Mod(base, Pow256)
			// Pop result back on the stack
			vm.stack.Push(base)
		case oSUB:
			x, y := vm.stack.Popn()
			// (x - y) % 2 ** 256
			base.Sub(x, y)
			base.Mod(base, Pow256)
			// Pop result back on the stack
			vm.stack.Push(base)
		case oMUL:
			x, y := vm.stack.Popn()
			// (x * y) % 2 ** 256
			base.Mul(x, y)
			base.Mod(base, Pow256)
			// Pop result back on the stack
			vm.stack.Push(base)
		case oDIV:
			x, y := vm.stack.Popn()
			// floor(x / y)
			base.Div(x, y)
			// Pop result back on the stack
			vm.stack.Push(base)
		case oSDIV:
			x, y := vm.stack.Popn()
			// n > 2**255
			if x.Cmp(Pow256) > 0 {
				x.Sub(Pow256, x)
			}
			if y.Cmp(Pow256) > 0 {
				y.Sub(Pow256, y)
			}
			z := new(big.Int)
			z.Div(x, y)
			if z.Cmp(Pow256) > 0 {
				z.Sub(Pow256, z)
			}
			// Push result on to the stack
			vm.stack.Push(z)
		case oMOD:
			x, y := vm.stack.Popn()
			base.Mod(x, y)
			vm.stack.Push(base)
		case oSMOD:
			x, y := vm.stack.Popn()
			// n > 2**255
			if x.Cmp(Pow256) > 0 {
				x.Sub(Pow256, x)
			}
			if y.Cmp(Pow256) > 0 {
				y.Sub(Pow256, y)
			}
			z := new(big.Int)
			z.Mod(x, y)
			if z.Cmp(Pow256) > 0 {
				z.Sub(Pow256, z)
			}
			// Push result on to the stack
			vm.stack.Push(z)
		case oEXP:
			x, y := vm.stack.Popn()
			base.Exp(x, y, Pow256)

			vm.stack.Push(base)
		case oNEG:
			base.Sub(Pow256, vm.stack.Pop())
			vm.stack.Push(base)
		case oLT:
			x, y := vm.stack.Popn()
			// x < y
			if x.Cmp(y) < 0 {
				vm.stack.Push(ethutil.BigTrue)
			} else {
				vm.stack.Push(ethutil.BigFalse)
			}
		case oLE:
			x, y := vm.stack.Popn()
			// x <= y
			if x.Cmp(y) < 1 {
				vm.stack.Push(ethutil.BigTrue)
			} else {
				vm.stack.Push(ethutil.BigFalse)
			}
		case oGT:
			x, y := vm.stack.Popn()
			// x > y
			if x.Cmp(y) > 0 {
				vm.stack.Push(ethutil.BigTrue)
			} else {
				vm.stack.Push(ethutil.BigFalse)
			}
		case oGE:
			x, y := vm.stack.Popn()
			// x >= y
			if x.Cmp(y) > -1 {
				vm.stack.Push(ethutil.BigTrue)
			} else {
				vm.stack.Push(ethutil.BigFalse)
			}
		case oNOT:
			x, y := vm.stack.Popn()
			// x != y
			if x.Cmp(y) != 0 {
				vm.stack.Push(ethutil.BigTrue)
			} else {
				vm.stack.Push(ethutil.BigFalse)
			}
		case oMYADDRESS:
			vm.stack.Push(ethutil.BigD(addr))
		case oTXSENDER:
			vm.stack.Push(ethutil.BigD(vars.sender))
		case oTXVALUE:
			vm.stack.Push(vars.txValue)
		case oTXDATAN:
			vm.stack.Push(big.NewInt(int64(len(vars.txData))))
		case oTXDATA:
			v := vm.stack.Pop()
			// v >= len(data)
			if v.Cmp(big.NewInt(int64(len(vars.txData)))) >= 0 {
				vm.stack.Push(ethutil.Big("0"))
			} else {
				vm.stack.Push(ethutil.Big(vars.txData[v.Uint64()]))
			}
		case oBLK_PREVHASH:
			vm.stack.Push(ethutil.BigD(vars.prevHash))
		case oBLK_COINBASE:
			vm.stack.Push(ethutil.BigD(vars.coinbase))
		case oBLK_TIMESTAMP:
			vm.stack.Push(big.NewInt(vars.time))
		case oBLK_NUMBER:
			vm.stack.Push(big.NewInt(int64(vars.blockNumber)))
		case oBLK_DIFFICULTY:
			vm.stack.Push(vars.diff)
		case oBASEFEE:
			// e = 10^21
			e := big.NewInt(0).Exp(big.NewInt(10), big.NewInt(21), big.NewInt(0))
			d := new(big.Rat)
			d.SetInt(vars.diff)
			c := new(big.Rat)
			c.SetFloat64(0.5)
			// d = diff / 0.5
			d.Quo(d, c)
			// base = floor(d)
			base.Div(d.Num(), d.Denom())

			x := new(big.Int)
			x.Div(e, base)

			// x = floor(10^21 / floor(diff^0.5))
			vm.stack.Push(x)
		case oSHA256, oSHA3, oRIPEMD160:
			// This is probably save
			// ceil(pop / 32)
			length := int(math.Ceil(float64(vm.stack.Pop().Uint64()) / 32.0))
			// New buffer which will contain the concatenated popped items
			data := new(bytes.Buffer)
			for i := 0; i < length; i++ {
				// Encode the number to bytes and have it 32bytes long
				num := ethutil.NumberToBytes(vm.stack.Pop().Bytes(), 256)
				data.WriteString(string(num))
			}

			if op == oSHA256 {
				vm.stack.Push(base.SetBytes(ethutil.Sha256Bin(data.Bytes())))
			} else if op == oSHA3 {
				vm.stack.Push(base.SetBytes(ethutil.Sha3Bin(data.Bytes())))
			} else {
				vm.stack.Push(base.SetBytes(ethutil.Ripemd160(data.Bytes())))
			}
		case oECMUL:
			y := vm.stack.Pop()
			x := vm.stack.Pop()
			//n := vm.stack.Pop()

			//if ethutil.Big(x).Cmp(ethutil.Big(y)) {
			data := new(bytes.Buffer)
			data.WriteString(x.String())
			data.WriteString(y.String())
			if secp256k1.VerifyPubkeyValidity(data.Bytes()) == 1 {
				// TODO
			} else {
				// Invalid, push infinity
				vm.stack.Push(ethutil.Big("0"))
				vm.stack.Push(ethutil.Big("0"))
			}
			//} else {
			//	// Invalid, push infinity
			//	vm.stack.Push("0")
			//	vm.stack.Push("0")
			//}

		case oECADD:
		case oECSIGN:
		case oECRECOVER:
		case oECVALID:
		case oPUSH:
			pc++
			vm.stack.Push(contract.GetMem(pc).BigInt())
		case oPOP:
			// Pop current value of the stack
			vm.stack.Pop()
		case oDUP:
			// Dup top stack
			x := vm.stack.Pop()
			vm.stack.Push(x)
			vm.stack.Push(x)
		case oSWAP:
			// Swap two top most values
			x, y := vm.stack.Popn()
			vm.stack.Push(y)
			vm.stack.Push(x)
		case oMLOAD:
			x := vm.stack.Pop()
			vm.stack.Push(vm.mem[x.String()])
		case oMSTORE:
			x, y := vm.stack.Popn()
			vm.mem[x.String()] = y
		case oSLOAD:
			// Load the value in storage and push it on the stack
			x := vm.stack.Pop()
			// decode the object as a big integer
			decoder := ethutil.NewValueFromBytes([]byte(contract.State().Get(x.String())))
			if !decoder.IsNil() {
				vm.stack.Push(decoder.BigInt())
			} else {
				vm.stack.Push(ethutil.BigFalse)
			}
		case oSSTORE:
			// Store Y at index X
			y, x := vm.stack.Popn()
			addr := ethutil.BigToBytes(x, 256)
			fmt.Printf(" => %x (%v) @ %v", y.Bytes(), y, ethutil.BigD(addr))
			contract.SetAddr(addr, y)
			//contract.State().Update(string(idx), string(y))
		case oJMP:
			x := int(vm.stack.Pop().Uint64())
			// Set pc to x - 1 (minus one so the incrementing at the end won't effect it)
			pc = x
			pc--
		case oJMPI:
			x := vm.stack.Pop()
			// Set pc to x if it's non zero
			if x.Cmp(ethutil.BigFalse) != 0 {
				pc = int(x.Uint64())
				pc--
			}
		case oIND:
			vm.stack.Push(big.NewInt(int64(pc)))
		case oEXTRO:
			memAddr := vm.stack.Pop()
			contractAddr := vm.stack.Pop().Bytes()

			// Push the contract's memory on to the stack
			vm.stack.Push(contractMemory(state, contractAddr, memAddr))
		case oBALANCE:
			// Pushes the balance of the popped value on to the stack
			account := state.GetAccount(vm.stack.Pop().Bytes())
			vm.stack.Push(account.Amount)
		case oMKTX:
			addr, value := vm.stack.Popn()
			from, length := vm.stack.Popn()

			makeInlineTx(addr.Bytes(), value, from, length, contract, state)
		case oSUICIDE:
			recAddr := vm.stack.Pop().Bytes()
			// Purge all memory
			deletedMemory := contract.state.NewIterator().Purge()
			// Add refunds to the pop'ed address
			refund := new(big.Int).Mul(StoreFee, big.NewInt(int64(deletedMemory)))
			account := state.GetAccount(recAddr)
			account.Amount.Add(account.Amount, refund)
			// Update the refunding address
			state.UpdateAccount(recAddr, account)
			// Delete the contract
			state.trie.Update(string(addr), "")

			fmt.Printf("(%d) => %x\n", deletedMemory, recAddr)
			break out
		default:
			fmt.Printf("Invalid OPCODE: %x\n", op)
		}
		fmt.Println("")
		//vm.stack.Print()
		pc++
	}

	state.UpdateContract(addr, contract)
}

func makeInlineTx(addr []byte, value, from, length *big.Int, contract *Contract, state *State) {
	fmt.Printf(" => creating inline tx %x %v %v %v", addr, value, from, length)
	j := 0
	dataItems := make([]string, int(length.Uint64()))
	for i := from.Uint64(); i < length.Uint64(); i++ {
		dataItems[j] = contract.GetMem(j).Str()
		j++
	}

	tx := NewTransaction(addr, value, dataItems)
	if tx.IsContract() {
		contract := MakeContract(tx, state)
		state.UpdateContract(tx.Hash()[12:], contract)
	} else {
		account := state.GetAccount(tx.Recipient)
		account.Amount.Add(account.Amount, tx.Value)
		state.UpdateAccount(tx.Recipient, account)
	}
}

// Returns an address from the specified contract's address
func contractMemory(state *State, contractAddr []byte, memAddr *big.Int) *big.Int {
	contract := state.GetContract(contractAddr)
	if contract == nil {
		log.Panicf("invalid contract addr %x", contractAddr)
	}
	val := state.trie.Get(memAddr.String())

	// decode the object as a big integer
	decoder := ethutil.NewValueFromBytes([]byte(val))
	if decoder.IsNil() {
		return ethutil.BigFalse
	}

	return decoder.BigInt()
}
