package ethchain

import (
	"fmt"
	"math/big"

	"github.com/geoffreyhinton/goeth_self_training/ethutil"
)

func Disassemble(script []byte) (asm []string) {
	pc := new(big.Int)

	for {
		if pc.Cmp(big.NewInt(int64(len(script)))) >= 0 {
			return
		}

		val := script[pc.Int64()]
		op := OpCode(val)
		asm = append(asm, fmt.Sprintf("%v", op))
		switch op {
		case oPUSH:
			pc.Add(pc, ethutil.Big1)
			data := script[pc.Int64() : pc.Int64()+32]
			val := ethutil.BigD(data)

			var b []byte
			if val.Int64() == 0 {
				b = []byte{0}
			} else {
				b = val.Bytes()
			}

			asm = append(asm, fmt.Sprintf("0x%x", b))

			pc.Add(pc, big.NewInt(31))
		case oPUSH20:
			pc.Add(pc, ethutil.Big1)
			data := script[pc.Int64() : pc.Int64()+20]
			val := ethutil.BigD(data)
			var b []byte
			if val.Int64() == 0 {
				b = []byte{0}
			} else {
				b = val.Bytes()
			}

			asm = append(asm, fmt.Sprintf("0x%x", b))

			pc.Add(pc, big.NewInt(19))
		}
		pc.Add(pc, ethutil.Big1)
	}
	return
}
