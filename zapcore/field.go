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
	"math"
)

type fieldType uint8

const (
	unknownType fieldType = iota
	boolType
	floatType
	intType
	int64Type
	uintType
	uint64Type
	uintptrType
	stringType
	marshalerType
	reflectType
	stringerType
	errorType
	skipType
)

// A Field is a marshaling operation used to add a key-value pair to a logger's
// context. Most fields are lazily marshaled, so it's inexpensive to add fields
// to disabled debug-level log statements.
type Field struct {
	key       string
	fieldType fieldType
	ival      int64
	str       string
	obj       interface{}
}

// AddTo exports a field through the ObjectEncoder interface. It's primarily
// useful to library authors, and shouldn't be necessary in most applications.
func (f Field) AddTo(enc ObjectEncoder) {
	var err error

	switch f.fieldType {
	case boolType:
		enc.AddBool(f.key, f.ival == 1)
	case floatType:
		enc.AddFloat64(f.key, math.Float64frombits(uint64(f.ival)))
	case intType:
		enc.AddInt(f.key, int(f.ival))
	case int64Type:
		enc.AddInt64(f.key, f.ival)
	case uintType:
		enc.AddUint(f.key, uint(f.ival))
	case uint64Type:
		enc.AddUint64(f.key, uint64(f.ival))
	case uintptrType:
		enc.AddUintptr(f.key, uintptr(f.ival))
	case stringType:
		enc.AddString(f.key, f.str)
	case stringerType:
		enc.AddString(f.key, f.obj.(fmt.Stringer).String())
	case marshalerType:
		err = enc.AddObject(f.key, f.obj.(LogObjectMarshaler))
	case reflectType:
		err = enc.AddReflected(f.key, f.obj)
	case errorType:
		enc.AddString(f.key, f.obj.(error).Error())
	case skipType:
		break
	default:
		panic(fmt.Sprintf("unknown field type: %v", f))
	}

	if err != nil {
		enc.AddString(fmt.Sprintf("%sError", f.key), err.Error())
	}
}

func addFields(enc ObjectEncoder, fields []Field) {
	for i := range fields {
		fields[i].AddTo(enc)
	}
}
