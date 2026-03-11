package engine

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/expr-lang/expr"
	"github.com/ohler55/ojg/jp"
	"github.com/ohler55/ojg/oj"
)

// Extract applies a mapping definition to raw JSON, returning extracted values.
// String values starting with "$" are treated as JSONPath expressions.
// Map values with an "expr" key are evaluated as arithmetic expressions.
// Keys ending with "[]" iterate over an array with sub-mappings.
func Extract(rawJSON []byte, mapping map[string]any) (map[string]any, error) {
	var parsed any
	if err := oj.Unmarshal(rawJSON, &parsed); err != nil {
		return nil, fmt.Errorf("parsing JSON: %w", err)
	}

	return extractFromParsed(parsed, mapping)
}

func extractFromParsed(data any, mapping map[string]any) (map[string]any, error) {
	result := make(map[string]any, len(mapping))

	for key, spec := range mapping {
		// Array iteration: keys ending with "[]"
		if strings.HasSuffix(key, "[]") {
			baseKey := strings.TrimSuffix(key, "[]")
			subMapping, ok := spec.(map[string]any)
			if !ok {
				continue
			}

			// Get the array from already-extracted result or from data
			var arr []any
			if items, ok := result[baseKey]; ok {
				arr, _ = toSlice(items)
			}
			if arr == nil {
				// Try to get it from the data root
				arr, _ = toSlice(data)
			}

			var extracted []map[string]any
			for _, item := range arr {
				row, err := extractFromParsed(item, subMapping)
				if err != nil {
					continue
				}
				extracted = append(extracted, row)
			}
			result[baseKey] = extracted
			continue
		}

		val, err := resolveValue(data, spec, result)
		if err != nil {
			result[key] = nil
			continue
		}
		result[key] = val
	}

	return result, nil
}

func resolveValue(data any, spec any, resolved map[string]any) (any, error) {
	switch v := spec.(type) {
	case string:
		if strings.HasPrefix(v, "$") {
			return evalJSONPath(data, v)
		}
		return v, nil

	case map[string]any:
		if exprStr, ok := v["expr"]; ok {
			s, _ := exprStr.(string)
			return evalExpression(data, s, resolved)
		}
		// Nested mapping
		return extractFromParsed(data, v)

	default:
		return v, nil
	}
}

func evalJSONPath(data any, path string) (any, error) {
	parsed, err := jp.ParseString(path)
	if err != nil {
		return nil, fmt.Errorf("parsing JSONPath %q: %w", path, err)
	}

	results := parsed.Get(data)
	if len(results) == 0 {
		return nil, fmt.Errorf("no results for JSONPath %q", path)
	}
	if len(results) == 1 {
		return results[0], nil
	}
	return results, nil
}

func evalExpression(data any, exprStr string, resolved map[string]any) (any, error) {
	// Build environment from resolved values and special functions
	env := make(map[string]any)
	for k, v := range resolved {
		env[k] = v
	}

	// Handle JSONPath references in expressions: resolve $... patterns
	processed := resolveJSONPathInExpr(data, exprStr)

	program, err := expr.Compile(processed, expr.Env(env), expr.AllowUndefinedVariables())
	if err != nil {
		return nil, fmt.Errorf("compiling expression %q: %w", exprStr, err)
	}

	result, err := expr.Run(program, env)
	if err != nil {
		return nil, fmt.Errorf("evaluating expression %q: %w", exprStr, err)
	}

	return result, nil
}

// resolveJSONPathInExpr finds JSONPath references like $.field in an expression
// string and replaces them with their resolved values. This handles expressions
// like "$.size > 0 ? (1 - $.sizeleft / $.size) * 100 : 0".
func resolveJSONPathInExpr(data any, exprStr string) string {
	// Handle special functions first: length($...) and sum($...)
	exprStr = resolveFunction(data, exprStr, "length")
	exprStr = resolveFunction(data, exprStr, "sum")

	// Then handle remaining $... references
	result := &strings.Builder{}
	i := 0
	for i < len(exprStr) {
		if exprStr[i] == '$' && (i == 0 || !isIdentChar(exprStr[i-1])) {
			// Extract the JSONPath
			j := i + 1
			for j < len(exprStr) && isJSONPathChar(exprStr[j]) {
				j++
			}
			path := exprStr[i:j]
			if val, err := evalJSONPath(data, path); err == nil {
				result.WriteString(toExprLiteral(val))
			} else {
				result.WriteString("0")
			}
			i = j
		} else {
			result.WriteByte(exprStr[i])
			i++
		}
	}
	return result.String()
}

func resolveFunction(data any, exprStr string, funcName string) string {
	for {
		prefix := funcName + "("
		idx := strings.Index(exprStr, prefix)
		if idx == -1 {
			return exprStr
		}

		// Find matching closing paren
		start := idx + len(prefix)
		depth := 1
		end := start
		for end < len(exprStr) && depth > 0 {
			if exprStr[end] == '(' {
				depth++
			} else if exprStr[end] == ')' {
				depth--
			}
			end++
		}
		if depth != 0 {
			return exprStr
		}

		inner := exprStr[start : end-1]

		// Evaluate the inner JSONPath
		var replacement string
		if strings.HasPrefix(inner, "$") {
			val, err := evalJSONPath(data, inner)
			if err != nil {
				replacement = "0"
			} else {
				switch funcName {
				case "length":
					replacement = fmt.Sprintf("%d", sliceLen(val))
				case "sum":
					replacement = toExprLiteral(sumValues(val))
				default:
					replacement = "0"
				}
			}
		} else {
			replacement = "0"
		}

		exprStr = exprStr[:idx] + replacement + exprStr[end:]
	}
}

func isJSONPathChar(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') ||
		(c >= '0' && c <= '9') || c == '.' || c == '[' || c == ']' ||
		c == '*' || c == '@' || c == '?' || c == '=' || c == '!' ||
		c == '<' || c == '>' || c == '\'' || c == '"' || c == ' ' || c == '_'
}

func isIdentChar(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_'
}

func toExprLiteral(val any) string {
	switch v := val.(type) {
	case int, int64, float64:
		return fmt.Sprintf("%v", v)
	case bool:
		return fmt.Sprintf("%v", v)
	case string:
		return fmt.Sprintf("%q", v)
	case nil:
		return "0"
	default:
		b, _ := json.Marshal(v)
		return string(b)
	}
}

func sliceLen(val any) int {
	if arr, ok := toSlice(val); ok {
		return len(arr)
	}
	return 0
}

func sumValues(val any) float64 {
	arr, ok := toSlice(val)
	if !ok {
		return 0
	}
	var total float64
	for _, item := range arr {
		total += toFloat64(item)
	}
	return total
}

func toSlice(val any) ([]any, bool) {
	switch v := val.(type) {
	case []any:
		return v, true
	case []map[string]any:
		result := make([]any, len(v))
		for i, item := range v {
			result[i] = item
		}
		return result, true
	default:
		return nil, false
	}
}

func toFloat64(val any) float64 {
	switch v := val.(type) {
	case float64:
		return v
	case int:
		return float64(v)
	case int64:
		return float64(v)
	case json.Number:
		f, _ := v.Float64()
		return f
	default:
		return 0
	}
}
