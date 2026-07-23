package eval

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
)

// AssertionResult holds the outcome of a single assertion check.
type AssertionResult struct {
	Type     string `json:"type"`
	Field    string `json:"field"`
	Passed   bool   `json:"passed"`
	Message  string `json:"message"`
	Expected string `json:"expected,omitempty"`
	Actual   string `json:"actual,omitempty"`
}

// RunAssertion evaluates a single assertion against tool call result data.
// The data argument is the parsed JSON result from the tool call.
func RunAssertion(a Assertion, data map[string]any) AssertionResult {
	result := AssertionResult{
		Type:  a.Type,
		Field: a.Field,
	}

	fieldVal, err := resolveField(data, a.Field)
	if err != nil {
		result.Passed = false
		result.Message = fmt.Sprintf("field resolution failed: %v", err)
		return result
	}

	switch a.Type {
	case "schema":
		checkSchema(&result, fieldVal, a.Expected)
	case "range":
		checkRange(&result, fieldVal, a.Expected)
	case "length":
		checkLength(&result, fieldVal, a.Expected)
	case "contains":
		checkContains(&result, fieldVal, a.Expected)
	case "golden_file":
		checkGoldenFile(&result, fieldVal, a.Expected)
	default:
		result.Passed = false
		result.Message = fmt.Sprintf("unknown assertion type: %s", a.Type)
	}

	return result
}

// resolveField navigates a map using a dot-separated path.
// Supports simple paths like "result.count" or root-level "." for the whole object.
func resolveField(data map[string]any, field string) (any, error) {
	if field == "" || field == "." {
		return data, nil
	}

	parts := strings.Split(field, ".")
	var current any = data

	for _, part := range parts {
		m, ok := current.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("cannot traverse into non-object at %q", part)
		}
		val, exists := m[part]
		if !exists {
			return nil, fmt.Errorf("field %q not found", part)
		}
		current = val
	}

	return current, nil
}

// checkSchema validates that the value matches the expected JSON type.
func checkSchema(r *AssertionResult, val any, expected any) {
	expectedType, ok := expected.(string)
	if !ok {
		r.Passed = false
		r.Message = "schema assertion expected value must be a string type name"
		return
	}

	actualType := jsonTypeName(val)
	r.Expected = expectedType
	r.Actual = actualType

	if actualType == expectedType {
		r.Passed = true
		r.Message = fmt.Sprintf("type is %s", actualType)
	} else {
		r.Passed = false
		r.Message = fmt.Sprintf("expected type %s, got %s", expectedType, actualType)
	}
}

// jsonTypeName returns the JSON type name for a Go value.
func jsonTypeName(val any) string {
	if val == nil {
		return "null"
	}

	switch v := val.(type) {
	case bool:
		return "boolean"
	case string:
		return "string"
	case float64, float32, int, int64:
		return "number"
	case []any:
		return "array"
	case map[string]any:
		return "object"
	default:
		// Handle numeric types that may come from YAML parsing
		rv := reflect.ValueOf(v)
		switch rv.Kind() {
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			return "number"
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			return "number"
		case reflect.Float32, reflect.Float64:
			return "number"
		case reflect.Slice:
			return "array"
		case reflect.Map:
			return "object"
		default:
			return fmt.Sprintf("unknown(%T)", val)
		}
	}
}

// checkRange validates that a numeric value falls within min/max bounds.
func checkRange(r *AssertionResult, val any, expected any) {
	num, ok := toFloat64(val)
	if !ok {
		r.Passed = false
		r.Message = fmt.Sprintf("value is not numeric: %v", val)
		return
	}

	bounds, ok := expected.(map[string]any)
	if !ok {
		r.Passed = false
		r.Message = "range assertion expected value must be {min, max}"
		return
	}

	minVal, hasMin := toFloat64(bounds["min"])
	maxVal, hasMax := toFloat64(bounds["max"])

	r.Actual = fmt.Sprintf("%v", num)

	if hasMin && hasMax {
		r.Expected = fmt.Sprintf("[%v, %v]", bounds["min"], bounds["max"])
		if num >= minVal && num <= maxVal {
			r.Passed = true
			r.Message = fmt.Sprintf("%v is within [%v, %v]", num, minVal, maxVal)
		} else {
			r.Passed = false
			r.Message = fmt.Sprintf("%v is outside [%v, %v]", num, minVal, maxVal)
		}
	} else if hasMin {
		r.Expected = fmt.Sprintf(">= %v", bounds["min"])
		r.Passed = num >= minVal
		r.Message = fmt.Sprintf("%v >= %v: %v", num, minVal, r.Passed)
	} else if hasMax {
		r.Expected = fmt.Sprintf("<= %v", bounds["max"])
		r.Passed = num <= maxVal
		r.Message = fmt.Sprintf("%v <= %v: %v", num, maxVal, r.Passed)
	} else {
		r.Passed = false
		r.Message = "range assertion requires at least min or max"
	}
}

// checkLength validates the length of a string or array.
func checkLength(r *AssertionResult, val any, expected any) {
	var length int

	switch v := val.(type) {
	case string:
		length = len(v)
	case []any:
		length = len(v)
	default:
		r.Passed = false
		r.Message = fmt.Sprintf("length assertion requires string or array, got %T", val)
		return
	}

	bounds, ok := expected.(map[string]any)
	if !ok {
		r.Passed = false
		r.Message = "length assertion expected value must be {min, max}"
		return
	}

	minVal, hasMin := toFloat64(bounds["min"])
	maxVal, hasMax := toFloat64(bounds["max"])

	r.Actual = fmt.Sprintf("%d", length)

	if hasMin && hasMax {
		r.Expected = fmt.Sprintf("[%v, %v]", int(minVal), int(maxVal))
		if float64(length) >= minVal && float64(length) <= maxVal {
			r.Passed = true
			r.Message = fmt.Sprintf("length %d is within [%v, %v]", length, int(minVal), int(maxVal))
		} else {
			r.Passed = false
			r.Message = fmt.Sprintf("length %d is outside [%v, %v]", length, int(minVal), int(maxVal))
		}
	} else if hasMin {
		r.Expected = fmt.Sprintf(">= %v", int(minVal))
		r.Passed = float64(length) >= minVal
		r.Message = fmt.Sprintf("length %d >= %v: %v", length, int(minVal), r.Passed)
	} else if hasMax {
		r.Expected = fmt.Sprintf("<= %v", int(maxVal))
		r.Passed = float64(length) <= maxVal
		r.Message = fmt.Sprintf("length %d <= %v: %v", length, int(maxVal), r.Passed)
	} else {
		r.Passed = false
		r.Message = "length assertion requires at least min or max"
	}
}

// checkContains validates that a string contains a substring, or an array
// contains a specific element.
func checkContains(r *AssertionResult, val any, expected any) {
	expectedStr := fmt.Sprintf("%v", expected)
	r.Expected = expectedStr

	switch v := val.(type) {
	case string:
		r.Actual = v
		if strings.Contains(v, expectedStr) {
			r.Passed = true
			r.Message = fmt.Sprintf("string contains %q", expectedStr)
		} else {
			r.Passed = false
			r.Message = fmt.Sprintf("string does not contain %q", expectedStr)
		}
	case []any:
		r.Actual = fmt.Sprintf("%v", v)
		found := false
		for _, elem := range v {
			if fmt.Sprintf("%v", elem) == expectedStr {
				found = true
				break
			}
		}
		if found {
			r.Passed = true
			r.Message = fmt.Sprintf("array contains %v", expected)
		} else {
			r.Passed = false
			r.Message = fmt.Sprintf("array does not contain %v", expected)
		}
	default:
		r.Passed = false
		r.Message = fmt.Sprintf("contains assertion requires string or array, got %T", val)
	}
}

// checkGoldenFile compares the value against stored golden file content.
func checkGoldenFile(r *AssertionResult, val any, expected any) {
	goldenPath, ok := expected.(string)
	if !ok {
		r.Passed = false
		r.Message = "golden_file assertion expected value must be a file path string"
		return
	}

	r.Expected = fmt.Sprintf("matches %s", goldenPath)

	actualJSON, err := json.Marshal(val)
	if err != nil {
		r.Passed = false
		r.Message = fmt.Sprintf("failed to marshal actual value: %v", err)
		return
	}

	goldenData, err := readGoldenFile(goldenPath)
	if err != nil {
		r.Passed = false
		r.Message = fmt.Sprintf("golden file error: %v", err)
		return
	}

	if jsonEqual(actualJSON, goldenData) {
		r.Passed = true
		r.Message = fmt.Sprintf("matches golden file %s", goldenPath)
	} else {
		r.Passed = false
		r.Actual = string(actualJSON)
		r.Message = fmt.Sprintf("does not match golden file %s", goldenPath)
	}
}

// toFloat64 converts a value to float64. Returns (value, true) on success.
func toFloat64(val any) (float64, bool) {
	if val == nil {
		return 0, false
	}
	switch v := val.(type) {
	case float64:
		return v, true
	case float32:
		return float64(v), true
	case int:
		return float64(v), true
	case int64:
		return float64(v), true
	default:
		rv := reflect.ValueOf(v)
		switch rv.Kind() {
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			return float64(rv.Int()), true
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			return float64(rv.Uint()), true
		case reflect.Float32, reflect.Float64:
			return rv.Float(), true
		default:
			return 0, false
		}
	}
}
