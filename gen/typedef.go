// Copyright (c) 2015 Uber Technologies, Inc.
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

package gen

import "github.com/thriftrw/thriftrw-go/compile"

// typedefGenerator generates code to serialize and deserialize typedefs.
type typedefGenerator struct {
	hasReaders
}

func (t *typedefGenerator) Reader(g Generator, spec *compile.TypedefSpec) (string, error) {
	name := "_" + goCase(spec.ThriftName()) + "_Read"
	if t.HasReader(name) {
		return name, nil
	}

	err := g.DeclareFromTemplate(
		`
		<$wire := import "github.com/thriftrw/thriftrw-go/wire">

		<$x := newVar "x">
		<$w := newVar "w">
		func <.Name>(<$w> <$wire>.Value) (<typeReference .Spec>, error) {
			var <$x> <typeName .Spec>
			err := <$x>.FromWire(<$w>)
			<if isStructType .Spec.Target>
				return &<$x>, err
			<else>
				return <$x>, err
			<end>
		}
		`,
		struct {
			Name string
			Spec *compile.TypedefSpec
		}{Name: name, Spec: spec},
	)

	return name, wrapGenerateError(spec.ThriftName(), err)
}

// typedef generates code for the given typedef.
func typedef(g Generator, spec *compile.TypedefSpec) error {
	err := g.DeclareFromTemplate(
		`
		<$wire := import "github.com/thriftrw/thriftrw-go/wire">
		<$typedefType := typeReference .>

		type <typeName .> <typeName .Target>

		<$v := newVar "v">
		<$x := newVar "x">
		func (<$v> <$typedefType>) ToWire() <$wire>.Value {
			<$x> := (<typeReference .Target>)(<$v>)
			return <toWire .Target $x>
		}

		<$w := newVar "w">
		func (<$v> *<typeName .>) FromWire(<$w> <$wire>.Value) error {
			<if isStructType .>
				return (<typeReference .Target>)(<$v>).FromWire(<$w>)
			<else>
				<$x>, err := <fromWire .Target $w>
				*<$v> = (<$typedefType>)(<$x>)
				return err
			<end>
		}
		`,
		spec,
	)
	return wrapGenerateError(spec.Name, err)
}
