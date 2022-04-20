package utils_test

import (
	"testing"

	"github.com/njasm/clerk/internal/utils"
	"github.com/stretchr/testify/assert"
)

var stringArray = []string{
	"foo", "bar", "baz",
}

type TestScenario[T any] struct {
	name     string
	haystack []T
	needle   T
	expected bool
	assert   func(assert.TestingT, bool, ...interface{}) bool
}

func TestAny(t *testing.T) {
	stringTestCases := []TestScenario[string]{
		{
			name: "Testing Strings - true", haystack: stringArray,
			needle: "bar", expected: true, assert: assert.True,
		},
		{
			name: "Testing Strings - false", haystack: stringArray,
			needle: "falsefalsefalse", expected: false, assert: assert.True,
		},
	}

	for _, scenario := range stringTestCases {
		t.Run(scenario.name, func(t *testing.T) {
			rv := utils.Any(scenario.haystack, scenario.needle)
			assert.Equal(t, scenario.expected, rv)
		})
	}

	intTestCases := []TestScenario[int]{
		{
			name: "Testing Integer - true", haystack: []int{1, 2, 3},
			needle: 3, expected: true, assert: assert.True,
		},
		{
			name: "Testing Integer - false", haystack: []int{1, 2, 3},
			needle: 0, expected: false, assert: assert.True,
		},
	}

	for _, scenario := range intTestCases {
		t.Run(scenario.name, func(t *testing.T) {
			rv := utils.Any(scenario.haystack, scenario.needle)
			assert.Equal(t, scenario.expected, rv)
		})
	}

	float32TestCases := []TestScenario[float32]{
		{
			name: "Testing float32 - true", haystack: []float32{.1, 2.532, 24.3},
			needle: 2.532, expected: true, assert: assert.True,
		},
		{
			name: "Testing float32 - false", haystack: []float32{.1, 2.532, 24.3},
			needle: 1.0, expected: false, assert: assert.True,
		},
	}

	for _, scenario := range float32TestCases {
		t.Run(scenario.name, func(t *testing.T) {
			rv := utils.Any(scenario.haystack, scenario.needle)
			assert.Equal(t, scenario.expected, rv)
		})
	}

	float64TestCases := []TestScenario[float64]{
		{
			name: "Testing float64 - true", haystack: []float64{.1, 2.532, 24.3},
			needle: 2.532, expected: true, assert: assert.True,
		},
		{
			name: "Testing float64 - false", haystack: []float64{.1, 2.532, 24.3},
			needle: 1.0, expected: false, assert: assert.True,
		},
	}

	for _, scenario := range float64TestCases {
		t.Run(scenario.name, func(t *testing.T) {
			rv := utils.Any(scenario.haystack, scenario.needle)
			assert.Equal(t, scenario.expected, rv)
		})
	}
}
