package lambroll

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"

	"github.com/Songmu/prompter"
)

func (app *App) saveFile(path string, b []byte, mode os.FileMode) error {
	if _, err := os.Stat(path); err == nil {
		ok := prompter.YN(fmt.Sprintf("Overwrite existing file %s?", path), false)
		if !ok {
			return nil
		}
	}
	return ioutil.WriteFile(path, b, mode)
}

func marshalJSON(s interface{}) ([]byte, error) {
	var buf bytes.Buffer
	b, err := json.Marshal(s)
	if err != nil {
		return nil, err
	}
	var v map[string]interface{}
	json.Unmarshal(b, &v)
	for key, value := range v {
		if value == nil {
			delete(v, key)
		}
	}
	b, _ = json.Marshal(v)
	json.Indent(&buf, b, "", "  ")
	buf.WriteString("\n")
	return buf.Bytes(), nil
}
