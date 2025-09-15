package suites

import (
	"cmp"
	"fmt"
	"reflect"
	"slices"
	"strings"
	"testing"
)

type (
	BeforeSuite interface{ BeforeSuite(t *testing.T) }
	AfterSuite  interface{ AfterSuite(t *testing.T) }
	BeforeTest  interface{ BeforeTest(t *testing.T) }
	AfterTest   interface{ AfterTest(t *testing.T) }
)

func Run(suite any) func(t *testing.T) {
	typ := reflect.TypeOf(suite)
	numMethods := getTestMethods(typ)

	if len(numMethods) == 0 {
		panic(fmt.Sprintf("No test methods found in %q", typ.String()))
	}

	return func(t *testing.T) {
		t.Helper()

		if s, ok := suite.(BeforeSuite); ok {
			s.BeforeSuite(t)
		}

		for _, i := range numMethods {
			method := typ.Method(i)

			if s, ok := suite.(BeforeTest); ok {
				s.BeforeTest(t)
			}

			t.Run(method.Name, func(t *testing.T) {
				method.Func.Call([]reflect.Value{reflect.ValueOf(suite), reflect.ValueOf(t)})
			})

			if s, ok := suite.(AfterTest); ok {
				s.AfterTest(t)
			}
		}

		if s, ok := suite.(AfterSuite); ok {
			s.AfterSuite(t)
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
