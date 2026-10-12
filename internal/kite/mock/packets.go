package mock

import (
	"encoding/binary"
	"math"
	"time"
)

// Packet is one tick in Kite's binary WebSocket format (A07 §A6; parsed by gokiteconnect
// ticker.parsePacket). Prices are rupees; the encoder writes paise. Token&0xFF == 9 marks an index.
type Packet struct {
	Token         uint32
	Mode          string // "ltp", "quote" or "full"
	LastPrice     float64
	LastQty       uint32
	AvgPrice      float64
	Volume        uint32
	BuyQty        uint32
	SellQty       uint32
	Open          float64
	High          float64
	Low           float64
	Close         float64
	LastTradeTime time.Time
	OI            uint32
	Timestamp     time.Time // exchange timestamp; full mode only
}

func paise(v float64) uint32 { return uint32(math.Round(v * 100)) }

func unix(t time.Time) uint32 {
	if t.IsZero() {
		return 0
	}
	return uint32(t.Unix())
}

// Encode returns the packet bytes: ltp 8; index quote 28 / full 32; equity quote 44 / full 184.
func (p Packet) Encode() []byte {
	be := binary.BigEndian
	if p.Mode == "ltp" {
		b := make([]byte, 8)
		be.PutUint32(b[0:], p.Token)
		be.PutUint32(b[4:], paise(p.LastPrice))
		return b
	}
	if p.Token&0xFF == 9 { // index
		n := 28
		if p.Mode == "full" {
			n = 32
		}
		b := make([]byte, n)
		be.PutUint32(b[0:], p.Token)
		be.PutUint32(b[4:], paise(p.LastPrice))
		be.PutUint32(b[8:], paise(p.High))
		be.PutUint32(b[12:], paise(p.Low))
		be.PutUint32(b[16:], paise(p.Open))
		be.PutUint32(b[20:], paise(p.Close))
		be.PutUint32(b[24:], paise(p.LastPrice-p.Close))
		if n == 32 {
			be.PutUint32(b[28:], unix(p.Timestamp))
		}
		return b
	}
	n := 44
	if p.Mode == "full" {
		n = 184
	}
	b := make([]byte, n)
	be.PutUint32(b[0:], p.Token)
	be.PutUint32(b[4:], paise(p.LastPrice))
	be.PutUint32(b[8:], p.LastQty)
	be.PutUint32(b[12:], paise(p.AvgPrice))
	be.PutUint32(b[16:], p.Volume)
	be.PutUint32(b[20:], p.BuyQty)
	be.PutUint32(b[24:], p.SellQty)
	be.PutUint32(b[28:], paise(p.Open))
	be.PutUint32(b[32:], paise(p.High))
	be.PutUint32(b[36:], paise(p.Low))
	be.PutUint32(b[40:], paise(p.Close))
	if n == 184 {
		be.PutUint32(b[44:], unix(p.LastTradeTime))
		be.PutUint32(b[48:], p.OI)
		be.PutUint32(b[60:], unix(p.Timestamp))
	}
	return b
}

// Frame packs packets into one binary WebSocket message: uint16 count, then per packet a
// uint16 length followed by the packet.
func Frame(ps ...Packet) []byte {
	out := make([]byte, 2)
	binary.BigEndian.PutUint16(out, uint16(len(ps)))
	for _, p := range ps {
		b := p.Encode()
		l := make([]byte, 2)
		binary.BigEndian.PutUint16(l, uint16(len(b)))
		out = append(out, l...)
		out = append(out, b...)
	}
	return out
}
