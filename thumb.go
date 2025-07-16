package gdb

import (
	"encoding/binary"
	"fmt"
)

type ThumbRegisters struct {
	Reg            [13]uint32 // R0-R12
	StackPointer   uint32     // R13
	LinkRegister   uint32     // R14
	ProgramCounter uint32     // R15
	XPSR           uint32
}

func (a ThumbRegisters) Encode() []byte {
	data := make([]byte, 168)

	for i := range a.Reg {
		binary.LittleEndian.PutUint32(data[i*4:(i+1)*4], a.Reg[i])
	}

	binary.LittleEndian.PutUint32(data[52:56], a.StackPointer)
	binary.LittleEndian.PutUint32(data[56:60], a.LinkRegister)
	binary.LittleEndian.PutUint32(data[60:64], a.ProgramCounter)

	binary.LittleEndian.PutUint32(data[164:], a.XPSR)

	return data
}

func (a *ThumbRegisters) Decode(data []byte) error {
	if len(data) != 168 {
		return fmt.Errorf("invalid ARM32 register data length: %d", len(data))
	}

	for i := range a.Reg {
		a.Reg[i] = binary.LittleEndian.Uint32(data[i*4 : (i+1)*4])
	}

	a.StackPointer = binary.LittleEndian.Uint32(data[52:56])
	a.LinkRegister = binary.LittleEndian.Uint32(data[56:60])
	a.ProgramCounter = binary.LittleEndian.Uint32(data[60:64])

	a.XPSR = binary.LittleEndian.Uint32(data[164:])

	return nil
}

func (a ThumbRegisters) String() string {
	return fmt.Sprintf("R0: %08x R1: %08x R2: %08x R3: %08x R4: %08x R5: %08x R6: %08x R7: %08x R8: %08x R9: %08x R10: %08x R11: %08x R12: %08x SP: %08x LR: %08x PC: %08x XPSR: %08x",
		a.Reg[0], a.Reg[1], a.Reg[2], a.Reg[3], a.Reg[4], a.Reg[5], a.Reg[6], a.Reg[7], a.Reg[8], a.Reg[9], a.Reg[10], a.Reg[11], a.Reg[12],
		a.StackPointer, a.LinkRegister, a.ProgramCounter, a.XPSR)
}

func (g *GDB) thumbCall(params CallParameters) (uint64, error) {
	if len(params.Params) > 4 {
		return 0, fmt.Errorf("too many parameters for Thumb32 call: %d", len(params.Params))
	}

	/* Convert top to address of last word in work area */
	params.WorkAreaTop -= 4

	if g.thumbCallLastTop != params.WorkAreaTop || g.thumbCallLastTop == 0 {
		if g.thumbCallBpCancel != nil {
			g.thumbCallBpCancel()
			g.thumbCallBpCancel = nil
		}

		/* Infinite loop at the end of the routine, on which we break */
		if err := g.MemoryWrite(params.WorkAreaTop, []byte{0xfe, 0xe7, 0xfe, 0xe7}); err != nil {
			return 0, err
		}

		g.thumbCallLastTop = params.WorkAreaTop

		if cancel, err := g.BreakpointSet(params.WorkAreaTop); err != nil {
			return 0, err
		} else {
			g.thumbCallBpCancel = cancel
		}
	}

	regs := ThumbRegisters{
		StackPointer:   uint32(params.WorkAreaTop) - 4,
		LinkRegister:   uint32(params.WorkAreaTop) | 1,
		ProgramCounter: uint32(params.Addr) | 1,
		XPSR:           0x41000000,
	}

	if params.ReturnAddr != 0 {
		regs.LinkRegister = uint32(params.ReturnAddr) | 1
	}

	for i, param := range params.Params {
		regs.Reg[i] = uint32(param)
	}

	if err := g.RegistersWrite(&regs); err != nil {
		return 0, err
	}

	if err := g.Run(false, 0); err != nil {
		return 0, err
	}

	if params.IgnoreReturnValue {
		return 0, nil
	}

	err := g.RegistersRead(&regs)
	return uint64(regs.Reg[0]), err
}
