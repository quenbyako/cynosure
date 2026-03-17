// Package testsuite provides tests for OAuth handler.
package testsuite

import (
	"cmp"
	"reflect"
	"slices"
	"strings"
	"testing"
)

type (
	beforeSuite interface{ beforeSuite(t *testing.T) }
	afterSuite  interface{ afterSuite(t *testing.T) }
	beforeTest  interface{ beforeTest(t *testing.T) }
	afterTest   interface{ afterTest(t *testing.T) }
)

func runSuite(suite any) func(t *testing.T) {
	typ := reflect.TypeOf(suite) //nolint:forbidigo // reflect is necessary for test suite
	methods := getTestMethods(typ)

	if len(methods) == 0 {
		return func(t *testing.T) {
			t.Helper()
			t.Fatalf("no test methods found in %T", suite)
		}
	}

	return func(t *testing.T) {
		t.Helper()

		runHooks(t, suite, true)
		defer runHooks(t, suite, false)

		for _, i := range methods {
			method := typ.Method(i)
			t.Run(method.Name, func(t *testing.T) {
				t.Helper()
				runTestWithHooks(t, suite, method)
			})
		}
	}
}

func runHooks(t *testing.T, suite any, isBefore bool) {
	t.Helper()

	if isBefore {
		if s, ok := suite.(beforeSuite); ok {
			s.beforeSuite(t)
		}

		return
	}

	if s, ok := suite.(afterSuite); ok {
		s.afterSuite(t)
	}
}

//nolint:forbidigo // reflect is necessary
func runTestWithHooks(t *testing.T, suite any, method reflect.Method) {
	t.Helper()

	if s, ok := suite.(beforeTest); ok {
		s.beforeTest(t)
	}

	defer func() {
		if s, ok := suite.(afterTest); ok {
			s.afterTest(t)
		}
	}()

	method.Func.Call([]reflect.Value{reflect.ValueOf(suite), reflect.ValueOf(t)})
}

//nolint:forbidigo // reflect is necessary
func getTestMethods(typ reflect.Type) []int {
	res := make([]int, 0)

	for i := range typ.NumMethod() {
		method := typ.Method(i)
		if strings.HasPrefix(method.Name, "Test") &&
			method.Type.NumIn() == 2 &&
			method.Type.In(1) == reflect.TypeOf((*testing.T)(nil)) {
			res = append(res, i)
		}
	}

	slices.SortFunc(res, func(i, j int) int {
		return cmp.Compare(typ.Method(i).Name, typ.Method(j).Name)
	})

	return res
}
