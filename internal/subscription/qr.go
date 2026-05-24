package subscription

import (
	"errors"
	"fmt"
	"strings"
)

const (
	qrSize          = 33
	qrDataCodewords = 80
	qrECCCodewords  = 20
	qrMaxBytes      = 78
)

var errQRDataTooLong = errors.New("qr payload is too long")

type qrMatrix struct {
	dark     [qrSize][qrSize]bool
	reserved [qrSize][qrSize]bool
}

func qrSVG(payload string) ([]byte, error) {
	if len([]byte(payload)) > qrMaxBytes {
		return nil, errQRDataTooLong
	}
	codewords := qrCodewords([]byte(payload))
	matrix := newQRMatrix()
	matrix.drawCodewords(codewords)
	matrix.applyMask0()
	matrix.drawFormatBits()
	return []byte(matrix.svg()), nil
}

func qrCodewords(payload []byte) []byte {
	var bits []bool
	appendBits := func(value int, length int) {
		for i := length - 1; i >= 0; i-- {
			bits = append(bits, ((value>>i)&1) != 0)
		}
	}
	appendBits(0x4, 4)
	appendBits(len(payload), 8)
	for _, b := range payload {
		appendBits(int(b), 8)
	}
	remaining := qrDataCodewords*8 - len(bits)
	if remaining > 4 {
		remaining = 4
	}
	appendBits(0, remaining)
	for len(bits)%8 != 0 {
		bits = append(bits, false)
	}
	data := make([]byte, 0, qrDataCodewords)
	for i := 0; i < len(bits); i += 8 {
		var value byte
		for j := 0; j < 8; j++ {
			value <<= 1
			if bits[i+j] {
				value |= 1
			}
		}
		data = append(data, value)
	}
	for len(data) < qrDataCodewords {
		if len(data)%2 == 0 {
			data = append(data, 0xec)
		} else {
			data = append(data, 0x11)
		}
	}
	ecc := reedSolomonRemainder(data, qrECCCodewords)
	return append(data, ecc...)
}

func newQRMatrix() *qrMatrix {
	matrix := &qrMatrix{}
	matrix.drawFinder(0, 0)
	matrix.drawFinder(0, qrSize-7)
	matrix.drawFinder(qrSize-7, 0)
	for i := 8; i < qrSize-8; i++ {
		matrix.setFunction(6, i, i%2 == 0)
		matrix.setFunction(i, 6, i%2 == 0)
	}
	matrix.drawAlignment(26, 26)
	matrix.setFunction(25, 8, true)
	matrix.reserveFormat()
	return matrix
}

func (m *qrMatrix) drawFinder(row int, col int) {
	for dy := -1; dy <= 7; dy++ {
		for dx := -1; dx <= 7; dx++ {
			y := row + dy
			x := col + dx
			if y < 0 || y >= qrSize || x < 0 || x >= qrSize {
				continue
			}
			dark := dx >= 0 && dx <= 6 && dy >= 0 && dy <= 6 &&
				(dx == 0 || dx == 6 || dy == 0 || dy == 6 || (dx >= 2 && dx <= 4 && dy >= 2 && dy <= 4))
			m.setFunction(y, x, dark)
		}
	}
}

func (m *qrMatrix) drawAlignment(row int, col int) {
	for dy := -2; dy <= 2; dy++ {
		for dx := -2; dx <= 2; dx++ {
			dark := abs(dx) == 2 || abs(dy) == 2 || (dx == 0 && dy == 0)
			m.setFunction(row+dy, col+dx, dark)
		}
	}
}

func (m *qrMatrix) reserveFormat() {
	for i := 0; i < 9; i++ {
		if i != 6 {
			m.reserved[8][i] = true
			m.reserved[i][8] = true
		}
	}
	for i := 0; i < 8; i++ {
		m.reserved[qrSize-1-i][8] = true
		m.reserved[8][qrSize-1-i] = true
	}
}

func (m *qrMatrix) setFunction(row int, col int, dark bool) {
	m.dark[row][col] = dark
	m.reserved[row][col] = true
}

func (m *qrMatrix) drawCodewords(codewords []byte) {
	bitIndex := 0
	upward := true
	for right := qrSize - 1; right >= 1; right -= 2 {
		if right == 6 {
			right--
		}
		for vert := 0; vert < qrSize; vert++ {
			row := vert
			if upward {
				row = qrSize - 1 - vert
			}
			for j := 0; j < 2; j++ {
				col := right - j
				if m.reserved[row][col] {
					continue
				}
				if bitIndex < len(codewords)*8 {
					m.dark[row][col] = ((codewords[bitIndex>>3] >> uint(7-(bitIndex&7))) & 1) != 0
				}
				bitIndex++
			}
		}
		upward = !upward
	}
}

func (m *qrMatrix) applyMask0() {
	for row := 0; row < qrSize; row++ {
		for col := 0; col < qrSize; col++ {
			if !m.reserved[row][col] && (row+col)%2 == 0 {
				m.dark[row][col] = !m.dark[row][col]
			}
		}
	}
}

func (m *qrMatrix) drawFormatBits() {
	bits := formatBits()
	for i := 0; i <= 5; i++ {
		m.setFunction(8, i, bit(bits, i))
	}
	m.setFunction(8, 7, bit(bits, 6))
	m.setFunction(8, 8, bit(bits, 7))
	m.setFunction(7, 8, bit(bits, 8))
	for i := 9; i < 15; i++ {
		m.setFunction(14-i, 8, bit(bits, i))
	}
	for i := 0; i < 8; i++ {
		m.setFunction(qrSize-1-i, 8, bit(bits, i))
	}
	for i := 8; i < 15; i++ {
		m.setFunction(8, qrSize-15+i, bit(bits, i))
	}
	m.setFunction(qrSize-8, 8, true)
}

func formatBits() int {
	data := 0x08
	value := data << 10
	for i := 14; i >= 10; i-- {
		if ((value >> i) & 1) != 0 {
			value ^= 0x537 << uint(i-10)
		}
	}
	return ((data << 10) | value) ^ 0x5412
}

func bit(value int, index int) bool {
	return ((value >> uint(index)) & 1) != 0
}

func (m *qrMatrix) svg() string {
	const quiet = 4
	var b strings.Builder
	size := qrSize + quiet*2
	b.WriteString(fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 %d %d" shape-rendering="crispEdges">`, size, size))
	b.WriteString(`<rect width="100%" height="100%" fill="#fff"/>`)
	b.WriteString(`<path fill="#000" d="`)
	for row := 0; row < qrSize; row++ {
		for col := 0; col < qrSize; col++ {
			if m.dark[row][col] {
				b.WriteString(fmt.Sprintf("M%d %dh1v1h-1z", col+quiet, row+quiet))
			}
		}
	}
	b.WriteString(`"/></svg>`)
	return b.String()
}

func reedSolomonRemainder(data []byte, degree int) []byte {
	gen := reedSolomonGenerator(degree)
	result := make([]byte, degree)
	for _, value := range data {
		factor := value ^ result[0]
		copy(result, result[1:])
		result[degree-1] = 0
		for i := 0; i < degree; i++ {
			result[i] ^= gfMul(gen[i+1], factor)
		}
	}
	return result
}

func reedSolomonGenerator(degree int) []byte {
	gen := []byte{1}
	root := byte(1)
	for i := 0; i < degree; i++ {
		next := make([]byte, len(gen)+1)
		for j := range gen {
			next[j] ^= gfMul(gen[j], root)
			next[j+1] ^= gen[j]
		}
		gen = next
		root = gfMul(root, 0x02)
	}
	return gen
}

func gfMul(x byte, y byte) byte {
	var result int
	a := int(x)
	b := int(y)
	for b > 0 {
		if b&1 != 0 {
			result ^= a
		}
		a <<= 1
		if a&0x100 != 0 {
			a ^= 0x11d
		}
		b >>= 1
	}
	return byte(result)
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}
