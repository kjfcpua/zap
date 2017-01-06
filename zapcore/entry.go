// Copyright (c) 2016 Uber Technologies, Inc.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

package zapcore

import (
	"fmt"
	"os"
	"strconv"
	"sync"
	"time"
)

var (
	_cePool = sync.Pool{}
	_exit   = os.Exit
)

// MakeEntryCaller makes an EntryCaller from the return signature of
// runtime.Caller().
func MakeEntryCaller(pc uintptr, file string, line int, ok bool) EntryCaller {
	if !ok {
		return EntryCaller{}
	}
	return EntryCaller{
		PC:      pc,
		File:    file,
		Line:    line,
		Defined: true,
	}
}

// EntryCaller represents a notable caller of a log entry.
type EntryCaller struct {
	Defined bool
	PC      uintptr
	File    string
	Line    int
}

// String returns a "file:line" string if the EntryCaller is Defined, and the
// empty string otherwise.
func (ec EntryCaller) String() string {
	if !ec.Defined {
		return ""
	}
	// TODO: pool these byte buffers.
	buf := make([]byte, 0, len(ec.File)+10) // line number is unlikely to take more than 9 bytes
	buf = append(buf, ec.File...)
	buf = append(buf, ':')
	return string(strconv.AppendInt(buf, int64(ec.Line), 10))
}

// An Entry represents a complete log message. The entry's structured context
// is already serialized, but the log level, time, and message are available
// for inspection and modification.
//
// Entries are pooled, so any functions that accept them must be careful not to
// retain references to them.
type Entry struct {
	Level   Level
	Time    time.Time
	Message string
	Caller  EntryCaller
	Stack   string
}

// CheckWriteAction indicates what action to take after (*CheckedEntry).Write
// is done. Actions are ordered in increasing severity.
type CheckWriteAction uint8

const (
	// WriteThenNoop is the default behavior to do nothing speccial after write.
	WriteThenNoop = CheckWriteAction(iota)
	// WriteThenFatal causes a fatal os.Exit() after Write.
	WriteThenFatal
	// WriteThenPanic causes a panic() after Write.
	WriteThenPanic
)

// CheckedEntry is an Entry together with an opaque Facility that has already
// agreed to log it (Facility.Enabled(Entry) == true). It is returned by
// Logger.Check to enable performance sensitive log sites to not allocate
// fields when disabled.
//
// CheckedEntry references should be created by calling AddFacility() or
// Should() on a nil *CheckedEntry. References are returned to a pool after
// Write, and MUST NOT be retained after calling their Write() method.
type CheckedEntry struct {
	Entry
	safeToWrite bool
	should      CheckWriteAction
	facs        []Facility
	// TODO: we could provide static spac for the first N facilities to avoid
	// allocations in common cases
}

// Write writes the entry to any Facility references stored, returning any
// errors, and returns the CheckedEntry reference to a pool for immediate
// re-use. Write finally does any panic or fatal exiting that should happen.
func (ce *CheckedEntry) Write(fields ...Field) error {
	if ce == nil {
		return nil
	}

	if !ce.safeToWrite {
		// this races with (*CheckedEntry).AddFacility; recover a log entry for
		// debugging, which may be an amalgamation of both the prior one
		// returned to pool, and any new one being set
		ent := ce.Entry
		return fmt.Errorf("race-prone write detected circa log entry %+v", ent)
	}
	ce.safeToWrite = false

	var errs multiError
	for i := range ce.facs {
		if err := ce.facs[i].Write(ce.Entry, fields); err != nil {
			errs = append(errs, err)
		}
	}

	should, msg := ce.should, ce.Message
	ce.should = WriteThenNoop
	ce.facs = ce.facs[:0]
	_cePool.Put(ce)

	switch should {
	case WriteThenFatal:
		_exit(1)
	case WriteThenPanic:
		panic(msg)
	}

	return errs.asError()
}

// AddFacility adds a facility that has agreed to log this entry. It's intended
// to be used by Facility.Check implementations. If ce is nil then a new
// CheckedEntry is created. Returns a non-nil CheckedEntry, maybe just created.
func (ce *CheckedEntry) AddFacility(ent Entry, fac Facility) *CheckedEntry {
	if ce != nil {
		ce.facs = append(ce.facs, fac)
		return ce
	}
	if x := _cePool.Get(); x != nil {
		ce = x.(*CheckedEntry)
		ce.Entry, ce.should, ce.safeToWrite = ent, WriteThenNoop, true
		ce.facs = append(ce.facs, fac)
		return ce
	}
	return &CheckedEntry{
		Entry:       ent,
		safeToWrite: true,
		facs:        []Facility{fac},
	}
}

// Should sets state so that a panic or fatal exit will happen after Write is
// called. Similarly to AddFacility, if ce is nil then a new CheckedEntry is
// built to record the intent to panic or fatal (this is why the caller must
// provide an Entry value, since ce may be nil).
func (ce *CheckedEntry) Should(ent Entry, should CheckWriteAction) *CheckedEntry {
	if ce != nil {
		ce.should = should
		return ce
	}
	if x := _cePool.Get(); x != nil {
		ce = x.(*CheckedEntry)
		ce.Entry, ce.should, ce.safeToWrite = ent, should, true
		return ce
	}
	return &CheckedEntry{
		Entry:       ent,
		safeToWrite: true,
		should:      should,
	}
}
