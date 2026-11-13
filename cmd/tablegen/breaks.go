package main

const (
	breaksShift       = 7
	breaksBlockMask   = 1<<breaksShift - 1
	breaksRecordSize  = u16Width
	lineClassMask     = 63
	wordClassMask     = 31
	lineClassShift    = 0
	wordClassShift    = 6
	eastAsianShift    = 11
	initialQuoteShift = 12
	finalQuoteShift   = 13
	pictoShift        = 14
	unassignedPicto   = 15
)

var lineClassNames = []string{
	"AL", "AK", "AP", "AS", "B2", "BA", "BB", "BK", "CB", "CL", "CM", "CP", "CR", "EB", "EM", "EX",
	"GL", "H2", "H3", "HH", "HL", "HY", "ID", "IN", "IS", "JL", "JT", "JV", "LF", "NL", "NS", "NU",
	"OP", "PO", "PR", "QU", "RI", "SP", "SY", "VF", "VI", "WJ", "ZW", "ZWJ",
}

var wordClassNames = []string{
	"Other", "CR", "LF", "Newline", "Extend", "ZWJ", "Regional_Indicator", "Format", "Katakana",
	"Hebrew_Letter", "ALetter", "Single_Quote", "Double_Quote", "MidNumLet", "MidLetter", "MidNum",
	"Numeric", "ExtendNumLet", "WSegSpace",
}

type breakClasses struct {
	line []uint8
	word []uint8
}

func newBreakClasses() breakClasses {
	return breakClasses{line: make([]uint8, codePoints), word: make([]uint8, codePoints)}
}

func (db *propertyDatabase) categoryOf(c rune) string {
	return categoryNames[db.props[c]>>categoryShift&categoryMask]
}

func (db *propertyDatabase) resolveLineBreak(c rune, name string) string {
	switch name {
	case "AI", "SG", "XX":
		return "AL"
	case "CJ":
		return "NS"
	case "SA":
		if category := db.categoryOf(c); category == "Mn" || category == "Mc" {
			return "CM"
		}
		return "AL"
	}
	return name
}

func (db *propertyDatabase) readLineBreak(first, last rune, fields []string) error {
	for c := first; c <= last; c++ {
		class, err := indexOf(lineClassNames, db.resolveLineBreak(c, fields[0]))
		if err != nil {
			return err
		}
		db.breaks.line[c] = uint8(class)
	}
	return nil
}

func (db *propertyDatabase) readWordBreak(first, last rune, fields []string) error {
	class, err := indexOf(wordClassNames, fields[0])
	if err != nil {
		return err
	}
	for c := first; c <= last; c++ {
		db.breaks.word[c] = uint8(class)
	}
	return nil
}

func (db *propertyDatabase) breakRecord(c rune) uint16 {
	p := db.props[c]
	line := db.breaks.line[c]
	record := uint16(line)<<lineClassShift | uint16(db.breaks.word[c])<<wordClassShift
	if width := widthNames[p>>widthShift&widthMask]; width == "F" || width == "W" || width == "H" {
		record |= 1 << eastAsianShift
	}
	if lineClassNames[line] == "QU" {
		switch db.categoryOf(c) {
		case "Pi":
			record |= 1 << initialQuoteShift
		case "Pf":
			record |= 1 << finalQuoteShift
		}
	}
	if p&pictographFlag != 0 {
		record |= 1 << pictoShift
		if db.categoryOf(c) == "Cn" {
			record |= 1 << unassignedPicto
		}
	}
	return record
}

func (db *propertyDatabase) breakTrie() (trie, error) {
	records := make([]uint16, codePoints)
	for c := range records {
		records[c] = db.breakRecord(rune(c))
	}
	return buildRecordTrie(records, breaksShift, func(v uint16) []byte {
		return appendU16(nil, int(v))
	})
}

func breakLayoutConstants() []layoutConstant {
	constants := []layoutConstant{
		lowConstant("breaks_block_shift", breaksShift, lowInt),
		lowConstant("breaks_block_mask", breaksBlockMask, lowWord),
		lowConstant("breaks_record_size", breaksRecordSize, lowInt),
		lowConstant("break_line_shift", lineClassShift, lowInt),
		lowConstant("break_line_mask", lineClassMask, lowWord),
		lowConstant("break_word_shift", wordClassShift, lowInt),
		lowConstant("break_word_mask", wordClassMask, lowWord),
		lowConstant("break_east_asian_shift", eastAsianShift, lowInt),
		lowConstant("break_initial_quote_shift", initialQuoteShift, lowInt),
		lowConstant("break_final_quote_shift", finalQuoteShift, lowInt),
		lowConstant("break_pictographic_shift", pictoShift, lowInt),
		lowConstant("break_unassigned_pictographic_shift", unassignedPicto, lowInt),
	}
	constants = append(constants, classConstants("line_", lineClassNames, lowInt)...)
	return append(constants, classConstants("word_", wordClassNames, lowInt)...)
}
