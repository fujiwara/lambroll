package lambroll

import (
	"io"
	"os"
	"sync"
)

var (
	CreateZipArchive           = createZipArchive
	ExpandExcludeFile          = expandExcludeFile
	LoadZipArchive             = loadZipArchive
	MergeTags                  = mergeTags
	FillDefaultValues          = fillDefaultValues
	JSONStr                    = jsonStr
	MarshalJSON                = marshalJSON
	NewFunctionFrom            = newFunctionFrom
	SortFunctionForDiff        = sortFunctionForDiff
	NewCallerIdentity          = newCallerIdentity
	Unzip                      = unzip
	ExtractExitCodeAndError    = extractExitCodeAndError
	IsAWSManagedTag            = isAWSManagedTag
	RunExternalDiff            = runExternalDiff
	RenderForExternalDiff      = renderForExternalDiff
	NewUpdateFunctionCodeInput = newUpdateFunctionCodeInput
	NormalizeCodeForDiff       = normalizeCodeForDiff
)

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
