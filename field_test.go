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
	"errors"
	"net"
	"sync"
	"testing"
	"time"

	"go.uber.org/zap/zapcore"

	"math"

	"github.com/stretchr/testify/assert"
)

var (
	// Compiler complains about constants overflowing, so store this in a variable.
	maxUint64 = uint64(math.MaxUint64)
)

func assertCanBeReused(t testing.TB, field zapcore.Field) {
	var wg sync.WaitGroup

	for i := 0; i < 100; i++ {
		enc := make(zapcore.MapObjectEncoder)

		// Ensure using the field in multiple encoders in separate goroutines
		// does not cause any races or panics.
		wg.Add(1)
		go func() {
			defer wg.Done()
			assert.NotPanics(t, func() {
				field.AddTo(enc)
			}, "Reusing a field should not cause issues")
		}()
	}

	wg.Wait()
}

func TestFieldConstructors(t *testing.T) {
	tests := []struct {
		name   string
		field  zapcore.Field
		expect zapcore.Field
	}{
		{"Skip", zapcore.Field{Type: zapcore.SkipType}, Skip()},
		{"Bool", zapcore.Field{Key: "k", Type: zapcore.BoolType, Integer: 1}, Bool("k", true)},
		{"Bool", zapcore.Field{Key: "k", Type: zapcore.BoolType, Integer: 1}, Bool("k", true)},
		{"Int", zapcore.Field{Key: "k", Type: zapcore.Int64Type, Integer: 1}, Int("k", 1)},
		{"Int64", zapcore.Field{Key: "k", Type: zapcore.Int64Type, Integer: 1}, Int64("k", 1)},
		{"Uint", zapcore.Field{Key: "k", Type: zapcore.Uint64Type, Integer: 1}, Uint("k", 1)},
		{"Uint64", zapcore.Field{Key: "k", Type: zapcore.Uint64Type, Integer: 1}, Uint64("k", 1)},
		{"Uint64", zapcore.Field{Key: "k", Type: zapcore.Uint64Type, Integer: int64(maxUint64)}, Uint64("k", maxUint64)},
		{"Uintptr", zapcore.Field{Key: "k", Type: zapcore.Int64Type, Integer: 10}, Uintptr("k", 0xa)},
		{"String", zapcore.Field{Key: "k", Type: zapcore.StringType, String: "foo"}, String("k", "foo")},
		{"Time", zapcore.Field{Key: "k", Type: zapcore.Int64Type, Integer: 0}, Time("k", time.Unix(0, 0))},
		{"Time", zapcore.Field{Key: "k", Type: zapcore.Int64Type, Integer: 1000}, Time("k", time.Unix(1, 0))},
		{"Error", zapcore.Field{Key: "error", Type: zapcore.StringType, String: "fail"}, Error(errors.New("fail"))},
		{"Error", Skip(), Error(nil)},
		{"Error", zapcore.Field{Key: "k", Type: zapcore.Int64Type, Integer: 1}, Duration("k", time.Nanosecond)},
		{"Stringer", zapcore.Field{Key: "k", Type: zapcore.StringType, String: "1.2.3.4"}, Stringer("k", net.ParseIP("1.2.3.4"))},
		{"Base64", zapcore.Field{Key: "k", Type: zapcore.StringType, String: "YWIxMg=="}, Base64("k", []byte("ab12"))},
	}

	for _, tt := range tests {
		assert.Equal(t, tt.expect, tt.field, "Unexpected output from convenience field constructor %s.", tt.name)
		assertCanBeReused(t, tt.field)
	}
}

func TestObjectField(t *testing.T) {
	name := zapcore.ObjectMarshalerFunc(func(enc zapcore.ObjectEncoder) error {
		enc.AddString("name", "jane")
		return nil
	})
	nameF := Object("k", name)
	assert.Equal(
		t,
		zapcore.Field{Key: "k", Type: zapcore.ObjectMarshalerType, Interface: name},
		nameF,
		"Unexpected output from Object field constructor.",
	)
	assertCanBeReused(t, nameF)
}

func TestReflectField(t *testing.T) {
	ints := []int{5, 6}
	intsF := Reflect("k", ints)
	assert.Equal(
		t,
		zapcore.Field{Key: "k", Type: zapcore.ReflectType, Interface: ints},
		Reflect("k", ints),
		"Unexpected output from Reflect field constructor.",
	)
}

func TestNestField(t *testing.T) {
	fs := zapcore.Fields{String("name", "phil"), Int("age", 42)}
	nest := Nest("k", fs...)
	assert.Equal(
		t,
		zapcore.Field{Key: "k", Type: zapcore.ObjectMarshalerType, Interface: fs},
		nest,
		"Unexpected output from Nest field constructor.",
	)
	assertCanBeReused(t, nest)
}

func TestStackField(t *testing.T) {
	f := Stack()
	assert.Equal(t, "stacktrace", f.Key, "Unexpected field key.")
	assert.Equal(t, zapcore.StringType, f.Type, "Unexpected field type.")
	assert.Contains(t, f.String, "zap.TestStackField", "Expected stacktrace to contain caller.")
	assertCanBeReused(t, f)
}
