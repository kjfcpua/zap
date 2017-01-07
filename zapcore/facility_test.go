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
	"testing"

	"github.com/stretchr/testify/assert"
)

func makeIntField(key string, val int) Field {
	return Field{Type: IntType, Integer: int64(val), Key: key}
}

func TestObserverWith(t *testing.T) {
	sf1, getLogs := NewObserver(InfoLevel)

	// need to pad out enough initial fields so that the underlying slice cap()
	// gets ahead of its len() so that the sf3/4 With append's could choose
	// not to copy (if the implementation doesn't force them)
	sf1 = sf1.With([]Field{makeIntField("a", 1), makeIntField("b", 2)})

	sf2 := sf1.With([]Field{makeIntField("c", 3)})
	sf3 := sf2.With([]Field{makeIntField("d", 4)})
	sf4 := sf2.With([]Field{makeIntField("e", 5)})
	ent := Entry{Level: InfoLevel, Message: "hello"}

	for i, f := range []Facility{sf2, sf3, sf4} {
		if ce := f.Check(ent, nil); ce != nil {
			ce.Write(makeIntField("i", i))
		}
	}

	assert.Equal(t, []ObservedLog{
		{
			Entry: ent,
			Context: []Field{
				makeIntField("a", 1),
				makeIntField("b", 2),
				makeIntField("c", 3),
				makeIntField("i", 0),
			},
		},
		{
			Entry: ent,
			Context: []Field{
				makeIntField("a", 1),
				makeIntField("b", 2),
				makeIntField("c", 3),
				makeIntField("d", 4),
				makeIntField("i", 1),
			},
		},
		{
			Entry: ent,
			Context: []Field{
				makeIntField("a", 1),
				makeIntField("b", 2),
				makeIntField("c", 3),
				makeIntField("e", 5),
				makeIntField("i", 2),
			},
		},
	}, getLogs(), "expected no field sharing between With siblings")
}
