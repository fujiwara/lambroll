package lambroll

import (
	"context"
	"io"
	"os"
	"sync"

	"github.com/aereal/jsondiff"
)

var (
	CreateZipArchive        = createZipArchive
	ExpandExcludeFile       = expandExcludeFile
	LoadZipArchive          = loadZipArchive
	MergeTags               = mergeTags
	FillDefaultValues       = fillDefaultValues
	JSONStr                 = jsonStr
	MarshalJSON             = marshalJSON
	NewFunctionFrom         = newFunctionFrom
	SortFunctionForDiff     = sortFunctionForDiff
	NewCallerIdentity       = newCallerIdentity
	Unzip                   = unzip
	ExtractExitCodeAndError = extractExitCodeAndError
	IsAWSManagedTag         = isAWSManagedTag
	RunExternalDiff         = runExternalDiff
	RenderForExternalDiff   = renderForExternalDiff

	// masking core (mask.go)
	ResolveMaskSelector   = resolveMaskSelector
	BuildEffectiveMaskSet = buildEffectiveMaskSet
	Canonicalize          = canonicalize
	ApplyMask             = applyMask
	MaskInput             = maskInput
)

// MaskTokenRegistry re-exports the per-run token registry for white-box tests.
type MaskTokenRegistry = maskTokenRegistry

// NewMaskTokenRegistry re-exports the registry constructor for white-box tests.
func NewMaskTokenRegistry() *MaskTokenRegistry {
	return newMaskTokenRegistry()
}

// TokenFor re-exports the unexported tokenFor method for white-box tests.
func (r *MaskTokenRegistry) TokenFor(canonical string) string {
	return r.tokenFor(canonical)
}

// NextCounter exposes the registry's internal counter for white-box assertions.
func (r *MaskTokenRegistry) NextCounter() int {
	return r.nextCounter
}

// EmitDiff re-exports the unexported emitDiff method for white-box tests.
func (app *App) EmitDiff(ctx context.Context, opt *DiffOption, label string, from, to *jsondiff.Input, ignore string, maskSelectors []string, maskReg *MaskTokenRegistry) (bool, error) {
	return app.emitDiff(ctx, opt, label, from, to, ignore, maskSelectors, maskReg)
}

func (o *DiffOption) SetWriter(w io.Writer) {
	o.w = w
}

type VersionsOutput = versionsOutput
type VersionsOutputs = versionsOutputs

func (app *App) CallerIdentity() *CallerIdentity {
	return app.callerIdentity
}

func (app *App) LoadFunction(f string) (*Function, error) {
	return app.loadFunction(f)
}

func init() {
	Setenv = tSetenv
}

var envs = sync.Map{}

func tSetenv(key, value string) error {
	orig, ok := os.LookupEnv(key)
	os.Setenv(key, value)
	if ok {
		envs.Store(key, func() { os.Setenv(key, orig) })
	} else {
		envs.Store(key, func() { os.Unsetenv(key) })
	}
	return nil
}

func ResetEnv() {
	envs.Range(func(key, value any) bool {
		value.(func())()
		return true
	})
	envs = sync.Map{}
}
