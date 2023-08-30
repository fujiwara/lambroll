package lambroll_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/fujiwara/lambroll"
	"github.com/google/go-cmp/cmp"
)

// TestFixedTime is a fixed time for testing (JST)
var TestFixedTime = time.Date(2023, 8, 30, 12, 34, 56, 0, time.FixedZone("Asia/Tokyo", 9*60*60))

var TestVersionsOutputs = lambroll.VersionsOutputs{
	{Version: "1", LastModified: TestFixedTime, Runtime: "go1.x"},
	{Version: "2", LastModified: TestFixedTime, Runtime: "python3.8", Aliases: []string{"current", "latest"}},
}

func TestVersionsJSON(t *testing.T) {
	jsonOutput := TestVersionsOutputs.JSON()

	var parsedOutputs lambroll.VersionsOutputs
	err := json.Unmarshal([]byte(jsonOutput), &parsedOutputs)

	if err != nil {
		t.Errorf("Failed to unmarshal JSON: %s", err)
	}

	if d := cmp.Diff(parsedOutputs, TestVersionsOutputs); d != "" {
		t.Errorf("JSON mismatch: diff:%s", d)
	}
}

func TestVersionsTSV(t *testing.T) {
	expectedTSV := "1\t2023-08-30T12:34:56+09:00\t\tgo1.x\n" +
		"2\t2023-08-30T12:34:56+09:00\tcurrent,latest\tpython3.8\n"

	if d := cmp.Diff(TestVersionsOutputs.TSV(), expectedTSV); d != "" {
		t.Errorf("TSV mismatch: diff:%s", d)
	}
}

func TestVersionsTable(t *testing.T) {
	tableOutput := TestVersionsOutputs.Table()
	expectedOutput := `
+---------+---------------------------+----------------+-----------+
| VERSION |       LAST MODIFIED       |    ALIASES     |  RUNTIME  |
+---------+---------------------------+----------------+-----------+
|       1 | 2023-08-30T12:34:56+09:00 |                | go1.x     |
|       2 | 2023-08-30T12:34:56+09:00 | current,latest | python3.8 |
+---------+---------------------------+----------------+-----------+
`
	expectedOutput = expectedOutput[1:] // remove first newline

	if d := cmp.Diff(tableOutput, expectedOutput); d != "" {
		t.Errorf("Table mismatch: diff:%s", d)
	}
}
