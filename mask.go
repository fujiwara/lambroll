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

// envVarSelectorBase is the path to the Lambda environment variable map; an
// environment-variable-name mask argument is expanded to a subscript of it.
const envVarSelectorBase = ".Environment.Variables"

// maskFuncName is the name of the custom gojq function registered while masking.
// It is distinctive to avoid colliding with anything in a user selector.
const maskFuncName = "_lambroll_mask"

// maskTokenRegistry maps each distinct canonical value to a stable token within
// a single diff render. Equal values share a token (so an unchanged secret
// produces no diff line); distinct values get distinct tokens (so a changed
// secret still shows as a diff) without revealing the value.
type maskTokenRegistry struct {
	valueToToken map[string]string
	next         int
}

func newMaskTokenRegistry() *maskTokenRegistry {
	return &maskTokenRegistry{valueToToken: map[string]string{}, next: 1}
}

func (r *maskTokenRegistry) tokenFor(canonical string) string {
	if tok, ok := r.valueToToken[canonical]; ok {
		return tok
	}
	tok := fmt.Sprintf("%s%d%s", maskTokenPrefix, r.next, maskTokenSuffix)
	r.valueToToken[canonical] = tok
	r.next++
	return tok
}

// resolveMaskSelector classifies a mask argument. An argument starting with "."
// is used verbatim as a jq selector; otherwise it is a Lambda environment
// variable name expanded to .Environment.Variables["<name>"]. A dotless argument
// that looks like a selector (contains ".", "[" or "|") sets warn so a likely
// mistyped selector is reported.
func resolveMaskSelector(arg string) (selector string, warn bool) {
	if strings.HasPrefix(arg, ".") {
		return arg, false
	}
	warn = strings.ContainsAny(arg, ".[|")
	// Encode the name as a JSON string literal (which is valid jq) so a name
	// containing " or \ cannot break out of the subscript. json.Marshal of a
	// string never fails.
	key, _ := json.Marshal(arg)
	return envVarSelectorBase + "[" + string(key) + "]", warn
}

// resolveMaskSelectors resolves every mask argument to a jq selector, dropping
// duplicates while preserving first-occurrence order.
func resolveMaskSelectors(args []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(args))
	for _, arg := range args {
		selector, warn := resolveMaskSelector(arg)
		if warn {
			slog.Warn("mask argument looks like a selector but does not start with '.'; treating it as an environment variable name", "argument", arg, "resolved", selector)
		}
		if _, ok := seen[selector]; ok {
			continue
		}
		seen[selector] = struct{}{}
		out = append(out, selector)
	}
	return out
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

// applyMask replaces every value matched by the selectors with a token. Each
// selector is run as an update assignment of the form
//
//	(<selector> | select(. != null)) |= _lambroll_mask
//
// so only existing (non-null) values are replaced. A selector pointing at an
// absent path therefore neither creates that path (which would otherwise appear
// as a spurious diff and fabricate a field on the side that lacks it) nor
// errors. A selector that matches nothing warns and is a no-op; a selector that
// iterates an absent node (e.g. ".Environment.Variables[]" on a function with
// no Environment) is also a no-op; any other evaluation error is returned so an
// invalid selector fails loudly instead of leaving a value unmasked. gojq does
// not mutate its input; the updated document is returned.
func applyMask(root any, selectors []string, reg *maskTokenRegistry) (any, error) {
	for _, selector := range selectors {
		query, err := gojq.Parse("(" + selector + " | select(. != null)) |= " + maskFuncName)
		if err != nil {
			return nil, fmt.Errorf("failed to parse mask selector %q: %w", selector, err)
		}
		matched := 0
		var maskErr error
		code, err := gojq.Compile(query, gojq.WithFunction(maskFuncName, 0, 0, func(in any, _ []any) any {
			canonical, err := canonicalize(in)
			if err != nil {
				if maskErr == nil {
					maskErr = err
				}
				return in
			}
			matched++
			return reg.tokenFor(canonical)
		}))
		if err != nil {
			return nil, fmt.Errorf("failed to compile mask selector %q: %w", selector, err)
		}
		v, ok := code.Run(root).Next()
		if !ok {
			return nil, fmt.Errorf("mask selector %q produced no result", selector)
		}
		if err, isErr := v.(error); isErr {
			if isNullIterationError(err) {
				slog.Warn("mask selector matched nothing", "selector", selector)
				continue
			}
			return nil, fmt.Errorf("failed to apply mask selector %q: %w", selector, err)
		}
		if maskErr != nil {
			return nil, fmt.Errorf("failed to apply mask selector %q: %w", selector, maskErr)
		}
		root = v
		if matched == 0 {
			slog.Warn("mask selector matched nothing", "selector", selector)
		}
	}
	return root, nil
}

// isNullIterationError reports whether err is gojq's "cannot iterate over: null"
// error, raised when a "[]" selector iterates an absent/null node. The message
// is matched literally because gojq's iteratorError type is unexported; gojq is
// version-pinned in go.mod and TestApplyMaskAbsentIterable guards this.
func isNullIterationError(err error) bool {
	return err != nil && err.Error() == "cannot iterate over: null"
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

// maskValue applies the mask selectors to a deep copy of v using a fresh token
// registry. It is the single-document counterpart of the diff masking (used by
// render), where there is no second side to keep token-consistent. With no
// selectors v is returned unchanged.
func maskValue(v any, selectors []string) (any, error) {
	return maskInput(v, "", selectors, newMaskTokenRegistry())
}

// maskInput returns x with the ignore query and then the mask selectors applied
// to a deep copy, so the original value is never mutated. With no selectors it
// returns x unchanged. The returned value already has ignore applied, so callers
// must not apply ignore again.
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
