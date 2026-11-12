package lsp

type segment struct {
	name     string
	suffixes []byte
}

func (s segment) plain() bool {
	return len(s.suffixes) == 0
}

func skipGroup(src string, end int) (int, byte) {
	shut := src[end-1]
	open := byte('(')
	if shut == ']' {
		open = '['
	}
	depth := 0
	for i := end - 1; i >= 0; i-- {
		switch src[i] {
		case shut:
			depth++
		case open:
			depth--
			if depth == 0 {
				return i, open
			}
		}
	}
	return -1, open
}

func segmentsBefore(src string, end int) ([]segment, bool) {
	var segments []segment
	for {
		var suffixes []byte
		for end > 0 {
			switch src[end-1] {
			case ')', ']':
				start, open := skipGroup(src, end)
				if start < 0 {
					return nil, false
				}
				suffixes = append([]byte{open}, suffixes...)
				end = start
				continue
			case '?':
				suffixes = append([]byte{'?'}, suffixes...)
				end--
				continue
			}
			break
		}
		start := end
		for start > 0 && isWord(src[start-1]) {
			start--
		}
		if start == end {
			return nil, false
		}
		segments = append([]segment{{name: src[start:end], suffixes: suffixes}}, segments...)
		if start == 0 || src[start-1] != '.' {
			return segments, true
		}
		end = start - 1
	}
}

func calleeChain(segments []segment) []string {
	names := make([]string, len(segments))
	for i, s := range segments {
		last := i == len(segments)-1
		if !s.plain() && !(last && string(s.suffixes) == "[") {
			return nil
		}
		names[i] = s.name
	}
	return names
}
