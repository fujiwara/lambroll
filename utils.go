package lambroll

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/Songmu/prompter"
)

func (app *App) saveFile(path string, b []byte, mode os.FileMode) error {
	if _, err := os.Stat(path); err == nil {
		ok := prompter.YN(fmt.Sprintf("Overwrite existing file %s?", path), false)
		if !ok {
			return nil
		}
	}
	return os.WriteFile(path, b, mode)
}

func marshalJSONV2(s interface{}) ([]byte, error) {
	b, err := json.Marshal(s)
	if err != nil {
		return nil, err
	}
	x := make(map[string]interface{})
	if err := json.Unmarshal(b, &x); err != nil {
		return nil, err
	}
	if b, err := json.MarshalIndent(omitEmptyValues(x), "", "  "); err != nil {
		return nil, err
	} else {
		return append(b, '\n'), nil
	}
}

func unmarshalJSON(src []byte, v interface{}, path string) error {
	strict := json.NewDecoder(bytes.NewReader(src))
	strict.DisallowUnknownFields()
	if err := strict.Decode(&v); err != nil {
		if !strings.Contains(err.Error(), "unknown field") {
			return err
		}
		log.Printf("[warn] %s in %s", err, path)

		// unknown field -> try lax decoder
		lax := json.NewDecoder(bytes.NewReader(src))
		return lax.Decode(&v)
	}
	return nil
}

func FindFunctionFilename() string {
	for _, name := range FunctionFilenames {
		if _, err := os.Stat(name); err == nil {
			return name
		}
	}
	return FunctionFilenames[0]
}
