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

package compile

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/uber/thriftrw-go/ast"
	"github.com/uber/thriftrw-go/idl"
)

func parseService(s string) *ast.Service {
	prog, err := idl.Parse([]byte(s))
	if err != nil {
		panic(fmt.Sprintf("failure to parse: %v: %s", err, s))
	}

	if len(prog.Definitions) != 1 {
		panic("parseService may be used to parse single services only")
	}

	return prog.Definitions[0].(*ast.Service)
}

func TestCompileService(t *testing.T) {
	keyDoesNotExistSpec := &StructSpec{
		Name:   "KeyDoesNotExist",
		Type:   ast.ExceptionType,
		Fields: make(FieldGroup),
	}

	internalErrorSpec := &StructSpec{
		Name:   "InternalServiceError",
		Type:   ast.ExceptionType,
		Fields: make(FieldGroup),
	}

	keyValueSpec := &ServiceSpec{
		Name: "KeyValue",
		Functions: map[string]*FunctionSpec{
			"setValue": {
				Name: "setValue",
				ArgsSpec: map[string]*FieldSpec{
					"key": {
						ID:   1,
						Name: "key",
						Type: StringSpec,
					},
					"value": {
						ID:   2,
						Name: "value",
						Type: BinarySpec,
					},
				},
			},
			"getValue": {
				Name: "getValue",
				ArgsSpec: map[string]*FieldSpec{
					"key": {
						ID:   1,
						Name: "key",
						Type: StringSpec,
					},
				},
				ResultSpec: &ResultSpec{
					ReturnType: BinarySpec,
					Exceptions: map[string]*FieldSpec{
						"doesNotExist": {
							ID:   1,
							Name: "doesNotExist",
							Type: keyDoesNotExistSpec,
						},
						"internalError": {
							ID:   2,
							Name: "internalError",
							Type: internalErrorSpec,
						},
					},
				},
			},
		},
	}

	tests := []struct {
		desc  string
		src   string
		scope Scope
		spec  *ServiceSpec
	}{
		{
			"empty service",
			"service Foo {}",
			nil,
			&ServiceSpec{
				Name:      "Foo",
				Functions: make(map[string]*FunctionSpec),
			},
		},
		{
			"simple service",
			`
				service KeyValue {
					void setValue(1: string key, 2: binary value)
					binary getValue(1: string key)
						throws (
							1: KeyDoesNotExist doesNotExist,
							2: InternalServiceError internalError
						)
				}
			`,
			scope(
				"KeyDoesNotExist", keyDoesNotExistSpec,
				"InternalServiceError", internalErrorSpec,
			),
			keyValueSpec,
		},
		{
			"service inheritance",
			`
				service BulkKeyValue extends KeyValue {
					void setValues(1: map<string, binary> items)
				}
			`,
			scope("KeyValue", keyValueSpec),
			&ServiceSpec{
				Name:   "BulkKeyValue",
				Parent: keyValueSpec,
				Functions: map[string]*FunctionSpec{
					"setValues": {
						Name: "setValues",
						ArgsSpec: map[string]*FieldSpec{
							"items": {
								ID:   1,
								Name: "items",
								Type: &MapSpec{
									KeySpec:   StringSpec,
									ValueSpec: BinarySpec,
								},
							},
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		require.NoError(
			t, tt.spec.Link(scope()),
			"invalid test: service must with an empty scope",
		)
		scope := scopeOrDefault(tt.scope)

		src := parseService(tt.src)
		if spec, err := compileService(src); assert.NoError(t, err, tt.desc) {
			if assert.NoError(t, spec.Link(scope), tt.desc) {
				assert.Equal(t, tt.spec, spec, tt.desc)
			}
		}
	}
}

func TestCompileServiceFailure(t *testing.T) {
	tests := []struct {
		desc     string
		src      string
		messages []string
	}{
		{
			"duplicate function name",
			`
				service Foo {
					void foo()
					void bar()
					i32 foo()
				}
			`,
			[]string{
				`the name "foo" has already been used on line 3`,
			},
		},
		{
			"duplicate in arg list",
			`
				service Foo {
					void bar(
						1: string foo,
						2: binary bar,
						3: i32 foo
					)
				}
			`,
			[]string{
				`cannot compile "bar"`,
				`the name "foo" has already been used`,
			},
		},
		{
			"duplicate in exception list",
			`
				service Foo {
					i32 bar(1: string foo) throws (
						1: KeyDoesNotExist error,
						2: InternalServiceError error,
					)
				}
			`,
			[]string{
				`cannot compile "bar"`,
				`the name "error" has already been used`,
			},
		},
		{
			"exceptions cannot have default values",
			`
				service Foo {
					void bar() throws (
						1: KeyDoesNotExist doesNotExist,
						2: InternalServiceError internalError = {
							'message': 'An internal error occurred'
						}
					)
				}
			`,
			[]string{`field "internalError"`, "cannot have a default value"},
		},
	}

	for _, tt := range tests {
		src := parseService(tt.src)
		_, err := compileService(src)
		if assert.Error(t, err, tt.desc) {
			for _, msg := range tt.messages {
				assert.Contains(t, err.Error(), msg, tt.desc)
			}
		}
	}
}

// TODO(abg) test link failures