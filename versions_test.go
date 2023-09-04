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
	{Version: "1", LastModified: TestFixedTime, PackageType: "Zip", Runtime: "provided.al2"},
	{Version: "2", LastModified: TestFixedTime, PackageType: "Image", Runtime: "", Aliases: []string{"current", "latest"}},
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
	t.Setenv("TZ", "UTC+9")
	expectedTSV := "1\t2023-08-30T12:34:56+09:00\t\tZip\tprovided.al2\n" +
		"2\t2023-08-30T12:34:56+09:00\tcurrent,latest\tImage\t\n"

	if d := cmp.Diff(TestVersionsOutputs.TSV(), expectedTSV); d != "" {
		t.Errorf("TSV mismatch: diff:%s", d)
	}
}

func TestVersionsTable(t *testing.T) {
	t.Setenv("TZ", "UTC+9")
	tableOutput := TestVersionsOutputs.Table()
	expectedOutput := `
+---------+---------------------------+----------------+--------------+--------------+
| VERSION |       LAST MODIFIED       |    ALIASES     | PACKAGE TYPE |   RUNTIME    |
+---------+---------------------------+----------------+--------------+--------------+
|       1 | 2023-08-30T12:34:56+09:00 |                | Zip          | provided.al2 |
|       2 | 2023-08-30T12:34:56+09:00 | current,latest | Image        |              |
+---------+---------------------------+----------------+--------------+--------------+
`
	expectedOutput = expectedOutput[1:] // remove first newline

	if d := cmp.Diff(tableOutput, expectedOutput); d != "" {
		t.Errorf("Table mismatch: diff:%s", d)
	}
}
