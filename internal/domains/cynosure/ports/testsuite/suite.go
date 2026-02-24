package testsuite

import (
	"cmp"
	"fmt"
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
	typ := reflect.TypeOf(suite)
	numMethods := getTestMethods(typ)

	if len(numMethods) == 0 {
		panic(fmt.Sprintf("No test methods found in %q", typ.String()))
	}

	return func(t *testing.T) {
		t.Helper()

		if s, ok := suite.(beforeSuite); ok {
			s.beforeSuite(t)
		}

		for _, i := range numMethods {
			method := typ.Method(i)

			if s, ok := suite.(beforeTest); ok {
				s.beforeTest(t)
			}

			t.Run(method.Name, func(t *testing.T) {
				method.Func.Call([]reflect.Value{reflect.ValueOf(suite), reflect.ValueOf(t)})
			})

			if s, ok := suite.(afterTest); ok {
				s.afterTest(t)
			}
		}

		if s, ok := suite.(afterSuite); ok {
			s.afterSuite(t)
		}
	}
}

func getTestMethods(typ reflect.Type) []int {
	res := make([]int, 0)

	for i := range typ.NumMethod() {
		method := typ.Method(i)
		if strings.HasPrefix(method.Name, "Test") && method.Type.NumIn() == 2 && method.Type.In(1) == reflect.TypeOf((*testing.T)(nil)) {
			res = append(res, i)
		}
	}

	slices.SortFunc(res, func(i, j int) int {
		return cmp.Compare(typ.Method(i).Name, typ.Method(j).Name)
	})

	return res
}
