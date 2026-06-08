package lambroll

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/aereal/jsondiff"
	"github.com/itchyny/gojq"
)

const (
	maskTokenPrefix = "***MASKED#"
	maskTokenSuffix = "***"
)

const (
	envVarSelectorPrefix = `.Environment.Variables["`
	envVarSelectorSuffix = `"]`
)

// maskTokenRegistry maps each distinct canonical value to a stable token for a
// single diff run. Equal values share a token; distinct values get distinct
// tokens from a global counter in encounter order.
type maskTokenRegistry struct {
	valueToToken map[string]string
	nextCounter  int
}

func newMaskTokenRegistry() *maskTokenRegistry {
	return &maskTokenRegistry{
		valueToToken: map[string]string{},
		nextCounter:  1,
	}
}

func (r *maskTokenRegistry) tokenFor(canonical string) string {
	if tok, ok := r.valueToToken[canonical]; ok {
		return tok
	}
	tok := fmt.Sprintf("%s%d%s", maskTokenPrefix, r.nextCounter, maskTokenSuffix)
	r.valueToToken[canonical] = tok
	r.nextCounter++
	return tok
}

// resolveMaskSelector classifies a mask argument. An argument starting with "."
// is used verbatim as a jq selector; otherwise it is an environment variable
// name expanded to .Environment.Variables["<name>"]. A dotless argument that
// looks like a selector (contains "." "[" or "|") returns warn=true.
func resolveMaskSelector(arg string) (selector string, warn bool) {
	if strings.HasPrefix(arg, ".") {
		return arg, false
	}
	warn = strings.ContainsAny(arg, ".[|")
	return envVarSelectorPrefix + arg + envVarSelectorSuffix, warn
}

// buildEffectiveMaskSet resolves and de-duplicates all mask arguments, keeping
// first-occurrence order with option-file entries before CLI entries. A mistyped
// dotless argument is warned about but still used as a name.
func buildEffectiveMaskSet(cliMasks, optionMasks []string) []string {
	seen := map[string]struct{}{}
	result := make([]string, 0, len(cliMasks)+len(optionMasks))

	add := func(args []string) {
		for _, arg := range args {
			selector, warn := resolveMaskSelector(arg)
			if warn {
				slog.Warn("mask argument looks like a selector but does not start with '.'; treating it as an environment variable name", "argument", arg, "resolved", selector)
			}
			if _, ok := seen[selector]; ok {
				continue
			}
			seen[selector] = struct{}{}
			result = append(result, selector)
		}
	}

	add(optionMasks)
	add(cliMasks)
	return result
}

// canonicalize reduces a matched value to a stable string used as the registry
// key, so structurally-equal values collapse to the same token. json.Marshal
// sorts object keys, so equal values always produce the same key.
func canonicalize(value any) (string, error) {
	b, err := json.Marshal(value)
	if err != nil {
		return "", fmt.Errorf("failed to canonicalize masked value: %w", err)
	}
	return string(b), nil
}

// applyMask replaces every value matched by selectors with a token, in place. A
// selector that matches nothing warns and is a no-op; one that fails to evaluate
// returns an error (so a value is never silently left unmasked); a root selector
// (".", "..") collapses the whole value to one token. root must be a deep copy.
func applyMask(root any, selectors []string, reg *maskTokenRegistry) (any, error) {
	for _, selector := range selectors {
		query, err := gojq.Parse("path(" + selector + ")")
		if err != nil {
			return nil, fmt.Errorf("failed to parse mask selector %q: %w", selector, err)
		}
		paths, err := enumeratePaths(query, root)
		if err != nil {
			return nil, fmt.Errorf("failed to evaluate mask selector %q: %w", selector, err)
		}
		matched := 0
		for _, path := range paths {
			if len(path) == 0 {
				// the selector targets the whole document root; replace the
				// entire value with one token (it cannot be set in place).
				canonical, err := canonicalize(root)
				if err != nil {
					return nil, err
				}
				return reg.tokenFor(canonical), nil
			}
			rawValue, exists, err := getValueAtPath(root, path)
			if err != nil {
				return nil, fmt.Errorf("failed to apply mask selector %q: %w", selector, err)
			}
			if !exists {
				continue
			}
			matched++
			canonical, err := canonicalize(rawValue)
			if err != nil {
				return nil, err
			}
			if err := setValueAtPath(root, path, reg.tokenFor(canonical)); err != nil {
				return nil, fmt.Errorf("failed to apply mask selector %q: %w", selector, err)
			}
		}
		if matched == 0 {
			slog.Warn("mask selector matched nothing", "selector", selector)
		}
	}
	return root, nil
}

// enumeratePaths returns the paths matched by a path(...) query. Iterating an
// absent/null node (e.g. ".Environment.Variables[]" on a function with no
// Environment) counts as no match; any other evaluation error is returned so an
// invalid selector fails loudly instead of leaving a value unmasked.
func enumeratePaths(query *gojq.Query, root any) ([][]any, error) {
	var paths [][]any
	iter := query.Run(root)
	for {
		got, ok := iter.Next()
		if !ok {
			break
		}
		if err, isErr := got.(error); isErr {
			if isNullIterationError(err) {
				return paths, nil
			}
			return nil, err
		}
		path, ok := got.([]any)
		if !ok {
			return nil, fmt.Errorf("expected a path but got %T", got)
		}
		paths = append(paths, path)
	}
	return paths, nil
}

// isNullIterationError reports whether err is gojq's "cannot iterate over: null"
// error, raised when a "[]" selector iterates an absent/null node. The message
// is matched literally because gojq's iteratorError type is unexported; gojq is
// version-pinned in go.mod and TestApplyMaskAbsentIterable guards this.
func isNullIterationError(err error) bool {
	return err != nil && err.Error() == "cannot iterate over: null"
}

// getValueAtPath returns the value at a gojq path and whether it exists. path()
// yields paths for missing keys too, so the exists flag distinguishes a real
// match from a path that points at nothing. An unsupported path element (e.g. an
// array slice) is returned as an error rather than silently treated as absent.
func getValueAtPath(root any, path []any) (any, bool, error) {
	cur := root
	for _, p := range path {
		switch key := p.(type) {
		case string:
			m, ok := cur.(map[string]any)
			if !ok {
				return nil, false, nil
			}
			v, ok := m[key]
			if !ok {
				return nil, false, nil
			}
			cur = v
		case int:
			arr, ok := cur.([]any)
			if !ok {
				return nil, false, nil
			}
			idx := arrayIndex(key, len(arr))
			if idx < 0 || idx >= len(arr) {
				return nil, false, nil
			}
			cur = arr[idx]
		default:
			return nil, false, fmt.Errorf("unsupported mask selector path element %v (%T)", p, p)
		}
	}
	return cur, true, nil
}

// arrayIndex resolves a gojq path index, which may be negative (counted from the
// end), to an absolute index.
func arrayIndex(key, length int) int {
	if key < 0 {
		return key + length
	}
	return key
}

// setValueAtPath writes token at the given gojq path within root.
func setValueAtPath(root any, path []any, token string) error {
	if len(path) == 0 {
		return fmt.Errorf("cannot set value at empty path")
	}
	cur := root
	for i, p := range path[:len(path)-1] {
		switch key := p.(type) {
		case string:
			m, ok := cur.(map[string]any)
			if !ok {
				return fmt.Errorf("mask path element %d (%q) is not an object", i, key)
			}
			cur = m[key]
		case int:
			arr, ok := cur.([]any)
			if !ok {
				return fmt.Errorf("mask path element %d (%d) is not an array", i, key)
			}
			idx := arrayIndex(key, len(arr))
			if idx < 0 || idx >= len(arr) {
				return fmt.Errorf("mask path index %d out of range", key)
			}
			cur = arr[idx]
		default:
			return fmt.Errorf("unsupported mask path element type %T", p)
		}
	}
	switch key := path[len(path)-1].(type) {
	case string:
		m, ok := cur.(map[string]any)
		if !ok {
			return fmt.Errorf("mask path leaf (%q) is not in an object", key)
		}
		m[key] = token
	case int:
		arr, ok := cur.([]any)
		if !ok {
			return fmt.Errorf("mask path leaf (%d) is not in an array", key)
		}
		idx := arrayIndex(key, len(arr))
		if idx < 0 || idx >= len(arr) {
			return fmt.Errorf("mask path leaf index %d out of range", key)
		}
		arr[idx] = token
	default:
		return fmt.Errorf("unsupported mask path leaf type %T", path[len(path)-1])
	}
	return nil
}

// deepCopyJSONValue returns a deep copy of a JSON-decoded value via a round-trip,
// so masking never mutates the original local definition or remote configuration.
func deepCopyJSONValue(x any) (any, error) {
	b, err := json.Marshal(x)
	if err != nil {
		return nil, fmt.Errorf("failed to deep-copy value for masking: %w", err)
	}
	var out any
	if err := json.Unmarshal(b, &out); err != nil {
		return nil, fmt.Errorf("failed to deep-copy value for masking: %w", err)
	}
	return out, nil
}

// maskInput returns x with ignore then mask applied to a deep copy. With no
// selectors it returns x unchanged. The returned value already has ignore
// applied, so callers must not apply it again.
func maskInput(x any, ignore string, selectors []string, reg *maskTokenRegistry) (any, error) {
	if len(selectors) == 0 {
		return x, nil
	}
	copied, err := deepCopyJSONValue(x)
	if err != nil {
		return nil, err
	}
	if ignore != "" {
		q, err := gojq.Parse("del(" + ignore + ")")
		if err != nil {
			return nil, fmt.Errorf("failed to parse ignore query: %s %w", ignore, err)
		}
		modified, err := jsondiff.ModifyValue(q, copied)
		if err != nil {
			return nil, fmt.Errorf("failed to apply ignore query: %w", err)
		}
		copied = modified
	}
	return applyMask(copied, selectors, reg)
}
