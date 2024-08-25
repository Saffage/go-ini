package ini

import "strings"

var stop stopError

type stopError struct{}

func (stopError) Error() string { return "scanner.stop" }

type scanner struct {
	buf             []byte // Actual data.
	bufPos          int    // Current character index.
	lineNum         uint32 // Current line number.
	charNum         uint32 // Current character number.
	prevLineCharNum uint32 // Last character number in the previous line.
}

func (scan *scanner) init(buffer []byte) {
	*scan = scanner{buf: buffer, lineNum: 1, charNum: 1}
}

func (scan *scanner) peek() byte {
	return scan.lookAhead(0)
}

func (scan *scanner) lookAhead(offset int) byte {
	if scan.bufPos+offset < len(scan.buf) {
		return scan.buf[scan.bufPos+offset]
	}
	return '\000'
}

func (scan *scanner) advance() (previous byte) {
	previous = scan.peek()
	switch previous {
	case '\000':
		// Stay here

	case '\n', '\r':
		scan.handleNewline()

	default:
		scan.bufPos++
		scan.charNum++
	}
	return previous
}

func (base *scanner) consume(char byte) bool {
	if base.peek() == char {
		base.advance()
		return true
	}
	return false
}

func (base *scanner) take(f func() ([]byte, error)) (string, error) {
	result := strings.Builder{}
	for base.bufPos < len(base.buf) {
		b, err := f()
		result.Write(b)
		if err != nil {
			if err == stop {
				err = nil
			}
			return result.String(), err
		}
	}
	return result.String(), nil
}

func (base *scanner) takeWhile(f func(byte) bool) string {
	s, _ := base.take(func() ([]byte, error) {
		if f(base.peek()) {
			return []byte{base.advance()}, nil
		}
		return nil, stop
	})
	return s
}

func (base *scanner) takeUntil(f func(byte) bool) string {
	s, _ := base.take(func() ([]byte, error) {
		if !f(base.peek()) {
			return []byte{base.advance()}, nil
		}
		return nil, stop
	})
	return s
}

// Handles a new line character to update internal data.
// Must be called for every line in the buffer.
func (base *scanner) handleNewline() (wasNewline bool) {
	if base.peek() == '\r' {
		base.bufPos++
		wasNewline = true
	}
	if base.peek() == '\n' {
		base.bufPos++
		wasNewline = true
	}
	if wasNewline {
		base.prevLineCharNum = base.charNum
		base.charNum = 1
		base.lineNum++
	}
	return wasNewline
}
