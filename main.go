package gdb

import (
	"io"
)

type ArchitectureType string

const (
	ARMThumb ArchitectureType = "thumb"
)

type GDBConfig struct {
	Architecture ArchitectureType

	MaxWriteSize int
	MaxReadSize  int
}

type CancelFunc func() error

type GDB struct {
	conn io.ReadWriter
	cfg  GDBConfig

	rxBuf      [2048]byte
	rxBufLen   int
	rxBufIndex int

	rxState int
	rxPkt   []byte
	rxSum   [2]byte

	thumbCallLastTop  uint64
	thumbCallBpCancel CancelFunc
}

func New(conn io.ReadWriter, cfg GDBConfig) *GDB {
	if cfg.Architecture != ARMThumb {
		panic("Unsupported architecture")
	}

	if cfg.MaxWriteSize <= 0 {
		cfg.MaxWriteSize = 1024
	}
	if cfg.MaxReadSize <= 0 {
		cfg.MaxReadSize = 1024
	}

	return &GDB{
		conn: conn,
		cfg:  cfg,
	}
}
