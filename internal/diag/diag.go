package diag

import "fmt"

type Pos struct {
	File string
	Line int
	Col  int
}

func Start(file string) Pos {
	return Pos{File: file, Line: 1, Col: 1}
}

func (p Pos) String() string {
	if p.File == "" {
		return fmt.Sprintf("%d:%d", p.Line, p.Col)
	}
	return fmt.Sprintf("%s:%d:%d", p.File, p.Line, p.Col)
}

type Error struct {
	Pos Pos
	Msg string
}

func (e *Error) Error() string {
	return fmt.Sprintf("%s: %s", e.Pos, e.Msg)
}

func Errorf(pos Pos, format string, args ...any) *Error {
	return &Error{Pos: pos, Msg: fmt.Sprintf(format, args...)}
}

type bailout struct {
	err *Error
}

func Fail(pos Pos, format string, args ...any) {
	panic(bailout{Errorf(pos, format, args...)})
}

func Try(run func()) (failure *Error) {
	defer func() {
		r := recover()
		if r == nil {
			return
		}
		b, ok := r.(bailout)
		if !ok {
			panic(r)
		}
		failure = b.err
	}()
	run()
	return nil
}
