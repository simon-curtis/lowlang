package link

import (
	"encoding/binary"
	"fmt"
)

type structWriter struct {
	data []byte
	err  error
}

func (w *structWriter) add(value any) {
	if w.err != nil {
		return
	}
	appended, err := binary.Append(w.data, binary.LittleEndian, value)
	if err != nil {
		w.err = fmt.Errorf("link: encode %T: %w", value, err)
		return
	}
	w.data = appended
}

func (w *structWriter) bytes() ([]byte, error) {
	return w.data, w.err
}

func structSize(value any) uint64 {
	return uint64(binary.Size(value))
}
