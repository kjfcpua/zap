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
	"sync"
	"testing"

	"go.uber.org/zap/testutils"
	"go.uber.org/zap/zapcore"

	"github.com/stretchr/testify/assert"
)

func TestLevelEnablerFunc(t *testing.T) {
	enab := LevelEnablerFunc(func(l zapcore.Level) bool { return l == zapcore.DebugLevel })
	opts := []Option{Fields(Int("foo", 42))}
	withJSONLogger(t, enab, opts, func(log Logger, buf *testutils.Buffer) {
		log.Debug("@debug", Int("logger", 0))
		log.Info("@info", Int("logger", 0))
		assert.Equal(t, []string{
			`{"level":"debug","msg":"@debug","foo":42,"logger":0}`,
		}, buf.Lines())
	})
}

func TestDynamicLevel(t *testing.T) {
	lvl := DynamicLevel()
	assert.Equal(t, InfoLevel, lvl.Level(), "Unexpected initial level.")
	lvl.SetLevel(ErrorLevel)
	assert.Equal(t, ErrorLevel, lvl.Level(), "Unexpected level after SetLevel.")
}

func TestDynamicLevelMutation(t *testing.T) {
	lvl := DynamicLevel()
	lvl.SetLevel(WarnLevel)
	// Trigger races for non-atomic level mutations.
	proceed := make(chan struct{})
	wg := &sync.WaitGroup{}
	runConcurrently(10, 100, wg, func() {
		<-proceed
		assert.Equal(t, WarnLevel, lvl.Level())
	})
	runConcurrently(10, 100, wg, func() {
		<-proceed
		lvl.SetLevel(WarnLevel)
	})
	close(proceed)
	wg.Wait()
}
