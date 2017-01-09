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
	"sync"
	"time"
)

// Facility is a destination for log entries. It can have pervasive fields
// added with With().
type Facility interface {
	LevelEnabler

	With([]Field) Facility
	Check(Entry, *CheckedEntry) *CheckedEntry
	Write(Entry, []Field) error
}

// WriterFacility creates a facility that writes logs to a WriteSyncer. By
// default, if w is nil, os.Stdout is used.
func WriterFacility(enc Encoder, ws WriteSyncer, enab LevelEnabler) Facility {
	return ioFacility{
		LevelEnabler: enab,
		enc:          enc,
		out:          Lock(ws),
	}
}

type ioFacility struct {
	LevelEnabler
	enc Encoder
	out WriteSyncer
}

func (iof ioFacility) With(fields []Field) Facility {
	iof.enc = iof.enc.Clone()
	addFields(iof.enc, fields)
	return iof
}

func (iof ioFacility) Check(ent Entry, ce *CheckedEntry) *CheckedEntry {
	if iof.Enabled(ent.Level) {
		ce = ce.AddFacility(ent, iof)
	}
	return ce
}

func (iof ioFacility) Write(ent Entry, fields []Field) error {
	if err := iof.enc.WriteEntry(iof.out, ent, fields); err != nil {
		return err
	}
	if ent.Level > ErrorLevel {
		// Sync on Panic and Fatal, since they may crash the program.
		return iof.out.Sync()
	}
	return nil
}

// An ObservedLog is an encoding-agnostic representation of a log message.
type ObservedLog struct {
	Entry
	Context []Field
}

// Untimed returns a copy of the observed log with the entry time set to the
// zero value.
func (o ObservedLog) Untimed() ObservedLog {
	o.Entry.Time = time.Time{}
	return o
}

type observerSink struct {
	mu sync.Mutex
	// TODO: When we make observers a first-class concern, sinks should probably
	// have bounded space.
	logs []ObservedLog
}

func (s *observerSink) write(ent Entry, fields []Field) {
	log := ObservedLog{
		Entry:   ent,
		Context: fields,
	}
	s.mu.Lock()
	s.logs = append(s.logs, log)
	s.mu.Unlock()
}

func (s *observerSink) getLogs() []ObservedLog {
	s.mu.Lock()
	logs := make([]ObservedLog, 0, len(s.logs))
	logs = append(logs, s.logs...)
	s.mu.Unlock()
	return logs
}

type observerFacility struct {
	LevelEnabler
	sink    *observerSink
	context []Field
}

// NewObserver creates a new facility that buffers logs in memory (without any
// encoding). It's particularly useful in tests, though it can serve a variety
// of other purposes as well. This constructor returns the facility itself and
// a function to retrieve the observed logs.
func NewObserver(enab LevelEnabler) (Facility, func() []ObservedLog) {
	fac := &observerFacility{
		LevelEnabler: enab,
		sink:         &observerSink{},
	}
	return fac, fac.sink.getLogs
}

func (o *observerFacility) With(fields []Field) Facility {
	return &observerFacility{
		LevelEnabler: o.LevelEnabler,
		sink:         o.sink,
		context:      append(o.context[:len(o.context):len(o.context)], fields...),
	}
}

func (o *observerFacility) Write(ent Entry, fields []Field) error {
	all := make([]Field, 0, len(fields)+len(o.context))
	all = append(all, o.context...)
	all = append(all, fields...)
	o.sink.write(ent, all)
	return nil
}

func (o *observerFacility) Check(ent Entry, ce *CheckedEntry) *CheckedEntry {
	if o.Enabled(ent.Level) {
		ce = ce.AddFacility(ent, o)
	}
	return ce
}
