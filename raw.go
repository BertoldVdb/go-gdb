package gdb

import (
	"bytes"
	"fmt"
	"strconv"
)

func (g *GDB) rawSendPacket(data []byte) error {
	var csum uint8
	for _, m := range data {
		csum += uint8(m)
	}

	for {
		/* Send command */
		if _, err := fmt.Fprintf(g.conn, "$%s#%02x", data, csum); err != nil {
			return err
		}

		/* Read acknowledgment */
		var ackBuf [1]byte
		if _, err := g.conn.Read(ackBuf[:]); err != nil {
			return err
		}

		if ackBuf[0] == '+' {
			return nil
		} else if ackBuf[0] != '-' {
			return fmt.Errorf("unexpected acknowledgment: %02x", ackBuf[0])
		}
	}
}

func (g *GDB) rawSendAck(ok bool) error {
	var ack byte
	if ok {
		ack = '+'
	} else {
		ack = '-'
	}
	_, err := g.conn.Write([]byte{ack})
	return err
}

func (g *GDB) rawRecvPacket() ([]byte, error) {
	for {
		if g.rxBufIndex >= g.rxBufLen {
			n, err := g.conn.Read(g.rxBuf[:])
			if err != nil {
				return nil, err
			}
			g.rxBufLen = n
			g.rxBufIndex = 0
		}

		for i, m := range g.rxBuf[g.rxBufIndex:g.rxBufLen] {
			switch g.rxState {
			case 0: /* Wait for start of packet */
				if m == '$' {
					g.rxState = 1
					g.rxPkt = g.rxPkt[:0]
				}
			case 1: /* Read packet data */
				if m == '#' {
					g.rxState = 2
				} else {
					g.rxPkt = append(g.rxPkt, m)
				}
			case 2: /* Read checksum MSB */
				g.rxSum[0] = m
				g.rxState = 3
			case 3: /* Read checksum LSB */
				g.rxSum[1] = m
				g.rxState = 0

				/* Verify checksum */
				var csumLocal uint8
				for _, k := range g.rxPkt {
					csumLocal += uint8(k)
				}
				if csumRemote, err := strconv.ParseUint(string(g.rxSum[:]), 16, 8); err == nil && csumLocal == uint8(csumRemote) {
					g.rxBufIndex += i + 1
					/* Send + */
					return g.rxPkt, g.rawSendAck(true)
				}

				if err := g.rawSendAck(false); err != nil {
					return nil, err
				}
			}
		}

		/* We consumed the entire buffer, reset it */
		g.rxBufLen = 0
	}
}

func rawRLEDecode(in []byte) ([]byte, error) {
	var out []byte
	for i := 0; i < len(in); i++ {
		if in[i] == '*' {
			if i == 0 || i+1 >= len(in) {
				return nil, fmt.Errorf("invalid RLE encoding: %s", in)
			}

			v := in[i-1]
			i++
			rep := in[i] - 29

			for j := 0; j < int(rep); j++ {
				out = append(out, v)
			}
		} else {
			out = append(out, in[i])
		}
	}

	return out, nil
}

func rawEscapeEncode(in []byte) []byte {
	out := make([]byte, 0, len(in)*2)
	for _, m := range in {
		if m == '$' || m == '#' || m == 0x7d {
			out = append(out, 0x7d)
		}
		out = append(out, byte(m))
	}
	return out
}

func (g *GDB) RawExchange(out []byte) ([]byte, error) {
	if len(out) > 0 {
		if out[0] == 'x' { /* Binary write requires escaping */
			out = rawEscapeEncode(out)
		}
	}

	if err := g.rawSendPacket(out); err != nil {
		return nil, err
	}

	result, err := g.rawRecvPacket()
	if err != nil {
		return nil, err
	}

	if err := rawIsError(result); err != nil {
		return nil, err
	}

	/* Does the packet contain run length encoding */
	if bytes.Contains(result, []byte{'*'}) {
		return rawRLEDecode(result)
	}

	return result, nil
}

type GDBError struct {
	Code uint8
}

func (e *GDBError) Error() string {
	return fmt.Sprintf("GDB error code %d", e.Code)
}

func rawIsError(out []byte) error {
	if len(out) < 2 || out[0] != 'E' {
		return nil
	}

	code, err := strconv.ParseUint(string(out[1:]), 10, 8)
	if err != nil {
		return fmt.Errorf("invalid GDB error code: %s", out)
	}

	return &GDBError{Code: uint8(code)}
}
