package gdb

import (
	"fmt"
	"log"
)

func (g *GDB) Run(setAddr bool, addr uint64) error {
	var err error

	addr |= 1

	if setAddr {
		_, err = g.RawExchange(fmt.Appendf(nil, "c%x", addr))
	} else {
		_, err = g.RawExchange([]byte{'c'})
	}
	return err
}

func (g *GDB) breakpointGetSize() int {
	/* Note: on some architectures, the breakpoint size may vary depending on the instruction,
	 * 2 is valid for thumb */
	if g.cfg.Architecture == ARMThumb {
		return 2
	}
	return 0
}

func (g *GDB) BreakpointSet(addr uint64) (CancelFunc, error) {
	cmd := fmt.Appendf(nil, "Z1,%x,%x", addr, g.breakpointGetSize())

	if _, err := g.RawExchange(cmd); err != nil {
		return nil, err
	}

	log.Printf("Breakpoint set at 0x%x", addr)

	return func() error {
		cmd[0] = 'z'
		_, err := g.RawExchange(cmd)
		return err
	}, nil
}

type CallParameters struct {
	Addr               uint64
	WorkAreaTop        uint64
	Params             []uint64
	SkipRestoreContext bool
	IgnoreReturnValue  bool
	ReturnAddr         uint64
}

func (g *GDB) Call(params CallParameters) (uint64, error) {
	/* Store current registers */
	var orig ThumbRegisters
	if !params.SkipRestoreContext {
		if err := g.RegistersRead(&orig); err != nil {
			return 0, err
		}
	}

	var retVal uint64
	var err error
	if g.cfg.Architecture == ARMThumb {
		retVal, err = g.thumbCall(params)
	}

	if err != nil {
		return 0, err
	}

	/* Put registers back if needed */
	if params.SkipRestoreContext {
		return retVal, nil
	}
	return retVal, g.RegistersWrite(&orig)
}
