package suite

import (
	"flag"
	"fmt"
	"os"
	"reflect"
	"regexp"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
)

var matchMethod = flag.String("m", "", "regular expression to select tests of the suite to run")

// Suite is a basic testing suite with methods for storing and
// retrieving the current *testing.T context.
type Suite struct {
	*assert.Assertions
	t *testing.T
}

// T retrieves the current *testing.T context.
func (suite *Suite) T() *testing.T {
	return suite.t
}

// SetT sets the current *testing.T context.
func (suite *Suite) SetT(t *testing.T) {
	suite.t = t
	suite.Assertions = assert.New(t)
}

func testName(suiteMethod string, testMethod string) string {
	methodRegexp := regexp.MustCompile(`([a-zA-Z/_]+\.)?Test([a-zA-Z_]*)`)
	parts := methodRegexp.FindStringSubmatch(suiteMethod)
	return fmt.Sprintf("%s.%s", parts[2], testMethod)
}

// Run takes a testing suite and runs all of the tests attached
// to it.
func Run(t *testing.T, suite TestingSuite) {
	suite.SetT(t)

	if setupAllSuite, ok := suite.(SetupAllSuite); ok {
		setupAllSuite.SetupSuite()
	}

	methodFinder := reflect.TypeOf(suite)
	tests := []testing.InternalTest{}
	for index := 0; index < methodFinder.NumMethod(); index++ {
		suitePc, _, _, ok := runtime.Caller(1)
		var suiteMethod string
		if ok {
			suiteMethod = runtime.FuncForPC(suitePc).Name()
		}
		method := methodFinder.Method(index)
		ok, err := methodFilter(method.Name)
		if err != nil {
			fmt.Fprintf(os.Stderr, "testify: invalid regexp for -m: %s\n", err)
			os.Exit(1)
		}
		if ok {
			test := testing.InternalTest{
				Name: testName(suiteMethod, method.Name),
				F: func(t *testing.T) {
					parentT := suite.T()
					suite.SetT(t)
					if setupTestSuite, ok := suite.(SetupTestSuite); ok {
						setupTestSuite.SetupTest()
					}
					defer func() {
						if tearDownTestSuite, ok := suite.(TearDownTestSuite); ok {
							tearDownTestSuite.TearDownTest()
						}
						suite.SetT(parentT)
					}()
					method.Func.Call([]reflect.Value{reflect.ValueOf(suite)})
				},
			}
			tests = append(tests, test)
		}
	}

	if !testing.RunTests(func(_, _ string) (bool, error) { return true, nil },
		tests) {
		t.Fail()
	}

	if tearDownAllSuite, ok := suite.(TearDownAllSuite); ok {
		tearDownAllSuite.TearDownSuite()
	}
}

// Filtering method according to set regular expression
// specified command-line argument -m
func methodFilter(name string) (bool, error) {
	if ok, _ := regexp.MatchString("^Test", name); !ok {
		return false, nil
	}
	return regexp.MatchString(*matchMethod, name)
}
