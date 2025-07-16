package gdb

import (
	"encoding/hex"
	"fmt"
)

func (g *GDB) memoryReadInternal(addr uint64, data []byte) error {
	resp, err := g.RawExchange(fmt.Appendf(nil, "m%x,%x", addr, len(data)))
	if err != nil {
		return err
	}

	buf, err := hex.DecodeString(string(resp))
	if err != nil {
		return err
	}

	copy(data, buf)
	return nil
}

func (g *GDB) MemoryRead(addr uint64, data []byte) error {
	for len(data) > 0 {
		l := min(g.cfg.MaxReadSize, len(data))
		buf := data[:l]
		data = data[l:]

		if err := g.memoryReadInternal(addr, buf); err != nil {
			return err
		}

		addr += uint64(len(buf))
	}

	return nil
}

func (g *GDB) MemoryWrite(addr uint64, data []byte) error {
	for len(data) > 0 {
		l := min(g.cfg.MaxWriteSize, len(data))
		buf := data[:l]
		data = data[l:]

		if _, err := g.RawExchange(fmt.Appendf(nil, "M%08x,%x:%x", addr, len(buf), buf)); err != nil {
			return err
		}

		addr += uint64(len(buf))
	}

	return nil
}

type Registers interface {
	String() string
	Encode() []byte
	Decode([]byte) error
}

func (g *GDB) RegistersRead(regs Registers) error {
	if resp, err := g.RawExchange([]byte{'g'}); err != nil {
		return err
	} else if bin, err := hex.DecodeString(string(resp)); err != nil {
		return err
	} else {
		return regs.Decode(bin)
	}
}

func (g *GDB) RegistersWrite(reg Registers) error {
	_, err := g.RawExchange(fmt.Appendf(nil, "G%x", reg.Encode()))
	return err
}
