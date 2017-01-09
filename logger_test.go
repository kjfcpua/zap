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

package zap

import (
	"os"
	"sync"
	"testing"

	"go.uber.org/zap/zapcore"

	"github.com/stretchr/testify/assert"
)

func opts(opts ...Option) []Option {
	return opts
}

type stubbedExit struct {
	Status *int
}

func (se *stubbedExit) Unstub() {
	_exit = os.Exit
}

func (se *stubbedExit) AssertNoExit(t testing.TB) {
	assert.Nil(t, se.Status, "Unexpected exit.")
}

func (se *stubbedExit) AssertStatus(t testing.TB, expected int) {
	if assert.NotNil(t, se.Status, "Expected to exit.") {
		assert.Equal(t, expected, *se.Status, "Unexpected exit code.")
	}
}

func stubExit() *stubbedExit {
	stub := &stubbedExit{}
	_exit = func(s int) { stub.Status = &s }
	return stub
}

func withLogger(t testing.TB, e zapcore.LevelEnabler, opts []Option, f func(Logger, func() []zapcore.ObservedLog)) {
	fac, getLogs := zapcore.NewObserver(e)
	getUntimedLogs := func() []zapcore.ObservedLog {
		timed := getLogs()
		for i := range timed {
			timed[i] = timed[i].Untimed()
		}
		return timed
	}
	log := New(fac, opts...)
	f(log, getUntimedLogs)
}

func TestLoggerDynamicLevel(t *testing.T) {
	// Test that the DynamicLevel applys to all ancestors and descendants.
	dl := DynamicLevel()

	// generations are zero-indexed
	gen1 := Int("generation", 1)
	gen2 := Int("generation", 2)
	loggerF := func(i int) zapcore.Field { return Int("logger", i) }
	roundF := func(i int) zapcore.Field { return Int("round", i) }

	withLogger(t, dl, nil, func(grandparent Logger, getLogs func() []zapcore.ObservedLog) {
		parent := grandparent.With(gen1)
		child := parent.With(gen2)

		all := []Logger{grandparent, parent, child}
		for round, lvl := range []zapcore.Level{InfoLevel, DebugLevel} {
			dl.SetLevel(lvl)
			for i, log := range all {
				log.Debug("", roundF(round), loggerF(i))
			}
			for i, log := range all {
				log.Info("", roundF(round), loggerF(i))
			}
		}

		assert.Equal(t, []zapcore.ObservedLog{
			// round 0 at InfoLevel
			{
				Entry:   zapcore.Entry{Level: InfoLevel},
				Context: []zapcore.Field{roundF(0), loggerF(0)},
			},
			{
				Entry:   zapcore.Entry{Level: InfoLevel},
				Context: []zapcore.Field{gen1, roundF(0), loggerF(0)},
			},
			{
				Entry:   zapcore.Entry{Level: InfoLevel},
				Context: []zapcore.Field{gen1, gen2, roundF(0), loggerF(0)},
			},

			// round 0 at DebugLevel logs nothing, since debug logs aren't enabled.
			// round 1 at InfoLevel
			{
				Entry:   zapcore.Entry{Level: InfoLevel},
				Context: []zapcore.Field{roundF(1), loggerF(0)},
			},
			{
				Entry:   zapcore.Entry{Level: InfoLevel},
				Context: []zapcore.Field{gen1, roundF(1), loggerF(0)},
			},
			{
				Entry:   zapcore.Entry{Level: InfoLevel},
				Context: []zapcore.Field{gen1, gen2, roundF(1), loggerF(0)},
			},

			// round 1 at DebugLevel
			{
				Entry:   zapcore.Entry{Level: DebugLevel},
				Context: []zapcore.Field{roundF(1), loggerF(0)},
			},
			{
				Entry:   zapcore.Entry{Level: DebugLevel},
				Context: []zapcore.Field{gen1, roundF(1), loggerF(0)},
			},
			{
				Entry:   zapcore.Entry{Level: DebugLevel},
				Context: []zapcore.Field{gen1, gen2, roundF(1), loggerF(0)},
			},
		}, getLogs())
	})
}

func TestLoggerInitialFields(t *testing.T) {
	fieldOpts := opts(Fields(Int("foo", 42), String("bar", "baz")))
	withLogger(t, DebugLevel, fieldOpts, func(logger Logger, getLogs func() []zapcore.ObservedLog) {
		logger.Info("")
		assert.Equal(
			t,
			zapcore.ObservedLog{Context: []zapcore.Field{Int("foo", 42), String("bar", "baz")}},
			getLogs()[0],
			"Unexpected output with initial fields set.",
		)
	})
}

func TestLoggerWith(t *testing.T) {
	fieldOpts := opts(Fields(Int("foo", 42)))
	withLogger(t, DebugLevel, fieldOpts, func(logger Logger, getLogs func() []zapcore.ObservedLog) {
		// Child loggers should have copy-on-write semantics, so two children
		// shouldn't stomp on each other's fields or affect the parent's fields.
		logger.With(String("one", "two")).Info("")
		logger.With(String("three", "four")).Info("")
		logger.Info("")

		assert.Equal(t, []zapcore.ObservedLog{
			{Context: []zapcore.Field{Int("foo", 42), String("one", "two")}},
			{Context: []zapcore.Field{Int("foo", 42), String("three", "four")}},
			{Context: []zapcore.Field{Int("foo", 42)}},
		}, getLogs(), "Unexpected cross-talk between child loggers.")
	})
}

func TestLoggerLogPanic(t *testing.T) {
	for _, tt := range []struct {
		do       func(Logger)
		should   bool
		expected string
	}{
		{func(logger Logger) { logger.Check(PanicLevel, "bar").Write() }, true, "bar"},
		{func(logger Logger) { logger.Panic("baz") }, true, "baz"},
	} {
		withLogger(t, DebugLevel, nil, func(logger Logger, getLogs func() []zapcore.ObservedLog) {
			if tt.should {
				assert.Panics(t, func() { tt.do(logger) }, "Expected panic")
			} else {
				assert.NotPanics(t, func() { tt.do(logger) }, "Expected no panic")
			}

			output := getLogs()
			assert.Equal(t, 1, len(output), "Unexpected number of logs.")
			assert.Equal(t, 1, len(output[0].Context), "Unexpected context on first log.")
			assert.Equal(t,
				t,
				zapcore.Entry{Message: tt.expected, Level: PanicLevel},
				output[0].Entry,
				"Unexpected output from panic-level Log.",
			)
		})
	}
}

func TestLoggerLogFatal(t *testing.T) {
	for _, tt := range []struct {
		do       func(Logger)
		should   bool
		status   int
		expected string
	}{
		{func(logger Logger) { logger.Check(FatalLevel, "bar").Write() }, true, 1, "bar"},
		{func(logger Logger) { logger.Fatal("baz") }, true, 1, "baz"},
	} {
		withLogger(t, DebugLevel, nil, func(logger Logger, getLogs func() []zapcore.ObservedLog) {
			stub := stubExit()
			defer stub.Unstub()
			tt.do(logger)
			if tt.should {
				stub.AssertStatus(t, tt.status)
			} else {
				stub.AssertNoExit(t)
			}
			output := getLogs()
			assert.Equal(t, 1, len(output), "Unexpected number of logs.")
			assert.Equal(t, 1, len(output[0].Context), "Unexpected context on first log.")
			assert.Equal(
				t,
				zapcore.Entry{Message: tt.expected, Level: FatalLevel},
				output[0].Entry,
				"Unexpected output from fatal-level Log.",
			)
		})
	}
}

func TestLoggerLeveledMethods(t *testing.T) {
	withLogger(t, DebugLevel, nil, func(logger Logger, getLogs func() []zapcore.ObservedLog) {
		tests := []struct {
			method        func(string, ...zapcore.Field)
			expectedLevel zapcore.Level
		}{
			{logger.Debug, DebugLevel},
			{logger.Info, InfoLevel},
			{logger.Warn, WarnLevel},
			{logger.Error, ErrorLevel},
		}
		for _, tt := range tests {
			tt.method("")
			output := getLogs()
			assert.Equal(t, 1, len(output), "Unexpected number of logs.")
			assert.Equal(t, 1, len(output[0].Context), "Unexpected context on first log.")
			assert.Equal(
				t,
				zapcore.Entry{Level: tt.expectedLevel},
				output[0].Entry,
				"Unexpected output from %s-level logger method.", tt.expectedLevel)
		}
	})
}

func TestLoggerAlwaysPanics(t *testing.T) {
	withLogger(t, FatalLevel, nil, func(logger Logger, getLogs func() []zapcore.ObservedLog) {
		msg := "Even if output is disabled, logger.Panic should always panic."
		assert.Panics(t, func() { logger.Panic("foo") }, msg)
		assert.Panics(t, func() {
			if ce := logger.Check(PanicLevel, "foo"); ce != nil {
				ce.Write()
			}
		}, msg)
		assert.Equal(t, 0, len(getLogs()), "Panics shouldn't be written out if PanicLevel is disabled.")
	})
}

func TestJSONLoggerAlwaysFatals(t *testing.T) {
	fat := func(logger Logger) {
		logger.Fatal("")
	}
	checkFat := func(logger Logger) {
		if ce := logger.Check(FatalLevel, ""); ce != nil {
			ce.Write()
		}
	}
	for _, f := range []func(Logger){fat, checkFat} {
		stub := stubExit()
		defer stub.Unstub()
		withLogger(t, FatalLevel+1, nil, func(logger Logger, getLogs func() []zapcore.ObservedLog) {
			f(logger)
			stub.AssertStatus(t, 1)
			assert.Equal(t, 0, len(getLogs()), "Fatals shouldn't be written out if FatalLevel is disabled.")
		})
	}
}

/*
func TestJSONLoggerDPanic(t *testing.T) {
	withJSONLogger(t, DebugLevel, nil, func(logger Logger, buf *testutils.Buffer) {
		assert.NotPanics(t, func() { logger.DPanic("foo") })
		assert.Equal(t, `{"level":"dpanic","msg":"foo"}`, buf.Stripped(), "Unexpected output from DPanic in production mode.")
	})
	withJSONLogger(t, DebugLevel, opts(Development()), func(logger Logger, buf *testutils.Buffer) {
		assert.Panics(t, func() { logger.DPanic("foo") })
		assert.Equal(t, `{"level":"dpanic","msg":"foo"}`, buf.Stripped(), "Unexpected output from Logger.Fatal in development mode.")
	})
}

func TestJSONLoggerNoOpsDisabledLevels(t *testing.T) {
	withJSONLogger(t, WarnLevel, nil, func(logger Logger, buf *testutils.Buffer) {
		logger.Info("silence!")
		assert.Equal(t, []string{}, buf.Lines(), "Expected logging at a disabled level to produce no output.")
	})
}

func TestJSONLoggerWriteEntryFailure(t *testing.T) {
	errBuf := &testutils.Buffer{}
	errSink := &spywrite.WriteSyncer{Writer: errBuf}
	logger := New(
		WriterFacility(newJSONEncoder(), AddSync(spywrite.FailWriter{}), DebugLevel),
		ErrorOutput(errSink))

	logger.Info("foo")
	// Should log the error.
	assert.Regexp(t, `write error: failed`, errBuf.Stripped(), "Expected to log the error to the error output.")
	assert.True(t, errSink.Called(), "Expected logging an internal error to call Sync the error sink.")
}

func TestJSONLoggerSyncsOutput(t *testing.T) {
	sink := &spywrite.WriteSyncer{Writer: ioutil.Discard}
	logger := New(WriterFacility(newJSONEncoder(), sink, DebugLevel))

	logger.Error("foo")
	assert.False(t, sink.Called(), "Didn't expect logging at error level to Sync underlying WriteCloser.")

	assert.Panics(t, func() { logger.Panic("foo") }, "Expected panic when logging at Panic level.")
	assert.True(t, sink.Called(), "Expected logging at panic level to Sync underlying WriteSyncer.")
}

func TestLoggerAddCaller(t *testing.T) {
	withJSONLogger(t, DebugLevel, opts(AddCaller()), func(logger Logger, buf *testutils.Buffer) {
		logger.Info("Callers.")
		assert.Regexp(t,
			`"caller":"[^"]+/logger_test.go:[\d]+","msg":"Callers\."`,
			buf.Stripped(), "Expected to find package name and file name in output.")
	})
}

func TestLoggerAddCallerFail(t *testing.T) {
	errBuf := &testutils.Buffer{}
	withJSONLogger(t, DebugLevel, opts(
		AddCaller(),
		ErrorOutput(errBuf),
	), func(log Logger, buf *testutils.Buffer) {
		logImpl := log.(*logger)
		logImpl.callerSkip = 1e3

		log.Info("Failure.")
		assert.Regexp(t,
			`addCaller error: failed to get caller`,
			errBuf.String(), "Didn't find expected failure message.")
		assert.Contains(t,
			buf.String(), `"msg":"Failure."`,
			"Expected original message to survive failures in runtime.Caller.")
	})
}

func TestLoggerAddStacks(t *testing.T) {
	withJSONLogger(t, DebugLevel, opts(AddStacks(InfoLevel)), func(logger Logger, buf *testutils.Buffer) {
		logger.Info("Stacks.")
		output := buf.String()
		require.Contains(t, output, "zap.TestLoggerAddStacks", "Expected to find test function in stacktrace.")
		assert.Contains(t, output, `"stacktrace":`, "Stacktrace added under an unexpected key.")

		buf.Reset()
		logger.Warn("Stacks.")
		assert.Contains(t, buf.String(), `"stacktrace":`, "Expected to include stacktrace at Warn level.")

		buf.Reset()
		logger.Debug("No stacks.")
		assert.NotContains(t, buf.String(), "Unexpected stacktrace at Debug level.")
	})
}

func TestLoggerConcurrent(t *testing.T) {
	withJSONLogger(t, DebugLevel, nil, func(logger Logger, buf *testutils.Buffer) {
		child := logger.With(String("foo", "bar"))

		wg := &sync.WaitGroup{}
		runConcurrently(5, 10, wg, func() {
			logger.Info("info", String("foo", "bar"))
		})
		runConcurrently(5, 10, wg, func() {
			child.Info("info")
		})

		wg.Wait()

		// Make sure the output doesn't contain interspersed entries.
		expected := `{"level":"info","msg":"info","foo":"bar"}` + "\n"
		assert.Equal(t, strings.Repeat(expected, 100), buf.String())
	})
}
*/

func runConcurrently(goroutines, iterations int, wg *sync.WaitGroup, f func()) {
	wg.Add(goroutines)
	for g := 0; g < goroutines; g++ {
		go func() {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				f()
			}
		}()
	}
}
