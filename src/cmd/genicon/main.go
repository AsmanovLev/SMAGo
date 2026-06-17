package main

import (
	"bytes"
	"encoding/binary"
	"os"
)

var letterMasks = map[byte][7][5]bool{
	'S': {{false, true, true, true, false}, {true, false, false, false, true}, {true, false, false, false, false}, {false, true, true, true, false}, {false, false, false, false, true}, {true, false, false, false, true}, {false, true, true, true, false}},
	'M': {{true, false, false, false, true}, {true, true, false, true, true}, {true, false, true, false, true}, {true, false, false, false, true}, {true, false, false, false, true}, {true, false, false, false, true}, {true, false, false, false, true}},
	'A': {{false, true, true, true, false}, {true, false, false, false, true}, {true, false, false, false, true}, {true, true, true, true, true}, {true, false, false, false, true}, {true, false, false, false, true}, {true, false, false, false, true}},
	'G': {{false, true, true, true, false}, {true, false, false, false, true}, {true, false, false, false, false}, {true, false, false, true, true}, {true, false, false, false, true}, {true, false, false, false, true}, {false, true, true, true, false}},
}

const letterW, letterH = 5, 7

func genIcon(path string) error {
	const w, h = 16, 16
	const qw, qh = 8, 8 // quadrant size

	pixels := make([]byte, w*h*4)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			i := (y*w + x) * 4

			t := float64(x+y) / float64(w+h-2)
			r := uint8(56*(1-t) + 217*t)
			g := uint8(189*(1-t) + 70*t)
			b := uint8(248*(1-t) + 239*t)

			qx, qy := x/qw, y/qh
			lx, ly := x%qw, y%qh

			letters := [4]byte{'S', 'M', 'A', 'G'}
			letter := letters[qy*2+qx]
			mask := letterMasks[letter]

			lxOff := (qw - letterW) / 2
			lyOff := (qh - letterH) / 2
			mx, my := lx-lxOff, ly-lyOff

			if mx >= 0 && mx < letterW && my >= 0 && my < letterH && mask[my][mx] {
				pixels[i+0] = 0xFF
				pixels[i+1] = 0xFF
				pixels[i+2] = 0xFF
				pixels[i+3] = 0xFF
			} else {
				pixels[i+0] = b
				pixels[i+1] = g
				pixels[i+2] = r
				pixels[i+3] = 0xFF
			}
		}
	}

	flipped := make([]byte, len(pixels))
	for y := 0; y < h; y++ {
		copy(flipped[y*w*4:], pixels[(h-1-y)*w*4:(h-y)*w*4])
	}

	andMask := make([]byte, w*h/8)

	var buf bytes.Buffer
	_ = binary.Write(&buf, binary.LittleEndian, uint16(0))
	_ = binary.Write(&buf, binary.LittleEndian, uint16(1))
	_ = binary.Write(&buf, binary.LittleEndian, uint16(1))

	buf.WriteByte(byte(w))
	buf.WriteByte(byte(h))
	buf.WriteByte(0)
	buf.WriteByte(0)
	_ = binary.Write(&buf, binary.LittleEndian, uint16(1))
	_ = binary.Write(&buf, binary.LittleEndian, uint16(32))

	imgSize := uint32(40 + len(flipped) + len(andMask))
	_ = binary.Write(&buf, binary.LittleEndian, imgSize)
	_ = binary.Write(&buf, binary.LittleEndian, uint32(22))

	_ = binary.Write(&buf, binary.LittleEndian, uint32(40))
	_ = binary.Write(&buf, binary.LittleEndian, int32(w))
	_ = binary.Write(&buf, binary.LittleEndian, int32(h*2))
	_ = binary.Write(&buf, binary.LittleEndian, uint16(1))
	_ = binary.Write(&buf, binary.LittleEndian, uint16(32))
	_ = binary.Write(&buf, binary.LittleEndian, uint32(0))
	_ = binary.Write(&buf, binary.LittleEndian, uint32(len(flipped)+len(andMask)))
	_ = binary.Write(&buf, binary.LittleEndian, int32(0))
	_ = binary.Write(&buf, binary.LittleEndian, int32(0))
	_ = binary.Write(&buf, binary.LittleEndian, uint32(0))
	_ = binary.Write(&buf, binary.LittleEndian, uint32(0))

	buf.Write(flipped)
	buf.Write(andMask)
	return os.WriteFile(path, buf.Bytes(), 0644)
}

func main() {
	out := "smago.ico"
	if len(os.Args) > 1 {
		out = os.Args[1]
	}
	if err := genIcon(out); err != nil {
		panic(err)
	}
}
