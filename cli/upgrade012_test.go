package cli

import (
	"fmt"
	"strings"
	"testing"

	c "github.com/gookit/color"
	"github.com/katbyte/terrafmt/lib/common"
	"github.com/kylelemons/godebug/diff"
	"github.com/spf13/afero"
)

var upgradeTestcases = []struct {
	name              string
	sourcefile        string
	resultfile        string
	noDiff            bool
	errMsg            []string
	fmtcompat         bool
	lineCount         int
	errorBlockCount   int
	updatedBlockCount int
	totalBlockCount   int
}{
	{
		name:            "Go no change",
		sourcefile:      "testdata/no_diffs.go",
		noDiff:          true,
		lineCount:       29,
		totalBlockCount: 3,
	},
	{
		name:              "Go formatting",
		sourcefile:        "testdata/has_diffs.go",
		resultfile:        "testdata/has_diffs_upgrade012.go", // This has stricter formatting than `fmt`
		lineCount:         86,
		updatedBlockCount: 4,
		totalBlockCount:   6,
	},
	{
		name:       "Go fmt verbs",
		sourcefile: "testdata/fmt_compat.go",
		noDiff:     true,
		fmtcompat:  false,
		errMsg: []string{
			"block 1 @ %s:8 failed to process with: terraform init failed:",
			"block 3 @ %s:30 failed to process with: terraform init failed:",
		},
		lineCount:       41,
		totalBlockCount: 3,
	},
	{
		name:              "Go fmt verbs --fmtcompat",
		sourcefile:        "testdata/fmt_compat.go",
		resultfile:        "testdata/fmt_compat_upgrade012.go",
		fmtcompat:         true,
		lineCount:         41,
		updatedBlockCount: 1,
		totalBlockCount:   3,
	},
	{
		name:       "Go bad terraform",
		sourcefile: "testdata/bad_terraform.go",
		resultfile: "testdata/bad_terraform_upgrade012.go",
		errMsg: []string{
			"block 2 @ %s:16 failed to process with: terraform init failed:",
		},
		errorBlockCount:   1,
		lineCount:         20,
		updatedBlockCount: 1,
		totalBlockCount:   2,
	},
	{
		name:       "Go unsupported format verbs",
		sourcefile: "testdata/unsupported_fmt.go",
		noDiff:     true,
		errMsg: []string{
			"block 1 @ %s:8 failed to process with: terraform init failed:",
		},
		errorBlockCount: 1,
		lineCount:       21,
		totalBlockCount: 1,
	},
	{
		name:       "Go unsupported format verbs --fmtcompat",
		sourcefile: "testdata/unsupported_fmt.go",
		noDiff:     true,
		fmtcompat:  true,
		errMsg: []string{
			"block 1 @ %s:8 failed to process with: terraform 0.12upgrade failed:",
		},
		errorBlockCount: 1,
		lineCount:       21,
		totalBlockCount: 1,
	},
	{
		name:            "Markdown no change",
		sourcefile:      "testdata/no_diffs.md",
		noDiff:          true,
		lineCount:       25,
		totalBlockCount: 3,
	},
	{
		name:              "Markdown formatting",
		sourcefile:        "testdata/has_diffs.md",
		resultfile:        "testdata/has_diffs_upgrade012.md", // This has stricter formatting than `fmt`
		lineCount:         33,
		updatedBlockCount: 4,
		totalBlockCount:   5,
	},
}

func TestCmdUpgrade012StdinDefault(t *testing.T) {
	t.Parallel()

	for _, testcase := range upgradeTestcases {
		testcase := testcase
		t.Run(testcase.name, func(t *testing.T) {
			t.Parallel()

			fs := afero.NewReadOnlyFs(afero.NewOsFs())

			inR, err := fs.Open(testcase.sourcefile)
			if err != nil {
				t.Fatalf("Error opening test input file %q: %s", testcase.sourcefile, err)
			}

			resultfile := testcase.resultfile
			if testcase.noDiff {
				resultfile = testcase.sourcefile
			}
			data, err := afero.ReadFile(fs, resultfile)
			if err != nil {
				t.Fatalf("Error reading test result file %q: %s", resultfile, err)
			}
			expected := c.String(string(data))

			var outB strings.Builder
			var errB strings.Builder
			log := common.CreateLogger(&errB)
			_, err = upgrade012File(fs, log, "", testcase.fmtcompat, false, inR, &outB, &errB)
			actualStdOut := outB.String()
			actualStdErr := errB.String()

			if err != nil {
				t.Fatalf("Got an error when none was expected: %v", err)
			}

			if actualStdOut != expected {
				t.Errorf("Output does not match expected: ('-' actual, '+' expected)\n%s", diff.Diff(actualStdOut, expected))
			}

			errMsg := []string{}
			for _, msg := range testcase.errMsg {
				errMsg = append(errMsg, fmt.Sprintf(msg, "stdin"))
			}
			checkExpectedErrors(t, actualStdErr, errMsg)
		})
	}
}

func TestCmdUpgrade012StdinVerbose(t *testing.T) {
	t.Parallel()

	for _, testcase := range upgradeTestcases {
		testcase := testcase
		t.Run(testcase.name, func(t *testing.T) {
			t.Parallel()

			fs := afero.NewReadOnlyFs(afero.NewOsFs())

			inR, err := fs.Open(testcase.sourcefile)
			if err != nil {
				t.Fatalf("Error opening test input file %q: %s", testcase.sourcefile, err)
			}

			var outB strings.Builder
			var errB strings.Builder
			log := common.CreateLogger(&errB)
			_, err = upgrade012File(fs, log, "", testcase.fmtcompat, true, inR, &outB, &errB)
			actualStdErr := errB.String()

			if err != nil {
				t.Fatalf("Got an error when none was expected: %v", err)
			}

			filenameColor := "lightMagenta"
			if testcase.noDiff {
				filenameColor = "magenta"
			}
			expectedSummaryLine := c.String(fmt.Sprintf(
				"<%s>%s</>: <cyan>%d</> lines & formatted <yellow>%d</>/<yellow>%d</> blocks!",
				filenameColor,
				"stdin",
				testcase.lineCount,
				testcase.updatedBlockCount,
				testcase.totalBlockCount,
			))

			trimmedStdErr := strings.TrimSpace(actualStdErr)
			lines := strings.Split(trimmedStdErr, "\n")
			summaryLine := lines[len(lines)-1]
			if summaryLine != expectedSummaryLine {
				t.Errorf("Unexpected summary:\nexpected %s\ngot      %s", expectedSummaryLine, summaryLine)
			}
		})
	}
}

func TestCmdUpgrade012FileDefault(t *testing.T) {
	t.Parallel()

	for _, testcase := range upgradeTestcases {
		testcase := testcase
		t.Run(testcase.name, func(t *testing.T) {
			t.Parallel()

			fs := afero.NewCopyOnWriteFs(
				afero.NewReadOnlyFs(afero.NewOsFs()),
				afero.NewMemMapFs(),
			)

			resultfile := testcase.resultfile
			if testcase.noDiff {
				resultfile = testcase.sourcefile
			}
			data, err := afero.ReadFile(fs, resultfile)
			if err != nil {
				t.Fatalf("Error reading test result file %q: %s", resultfile, err)
			}
			expected := c.String(string(data))

			var outB strings.Builder
			var errB strings.Builder
			log := common.CreateLogger(&errB)
			_, err = upgrade012File(fs, log, testcase.sourcefile, testcase.fmtcompat, false, nil, &outB, &errB)
			actualStdOut := outB.String()
			actualStdErr := errB.String()

			if err != nil {
				t.Fatalf("Got an error when none was expected: %v", err)
			}

			if actualStdOut != "" {
				t.Errorf("Stdout should not have output, got:\n%s", actualStdOut)
			}

			data, err = afero.ReadFile(fs, testcase.sourcefile)
			if err != nil {
				t.Fatalf("Error reading results from file %q: %s", resultfile, err)
			}
			actualContent := c.String(string(data))
			if actualContent != expected {
				t.Errorf("File does not match expected: ('-' actual, '+' expected)\n%s", diff.Diff(actualContent, expected))
			}

			errMsg := []string{}
			for _, msg := range testcase.errMsg {
				errMsg = append(errMsg, fmt.Sprintf(msg, testcase.sourcefile))
			}
			checkExpectedErrors(t, actualStdErr, errMsg)
		})
	}
}

func TestCmdUpgrade012FileVerbose(t *testing.T) {
	t.Parallel()

	for _, testcase := range upgradeTestcases {
		testcase := testcase
		t.Run(testcase.name, func(t *testing.T) {
			t.Parallel()

			fs := afero.NewCopyOnWriteFs(
				afero.NewReadOnlyFs(afero.NewOsFs()),
				afero.NewMemMapFs(),
			)

			var outB strings.Builder
			var errB strings.Builder
			log := common.CreateLogger(&errB)
			_, err := upgrade012File(fs, log, testcase.sourcefile, testcase.fmtcompat, true, nil, &outB, &errB)
			actualStdErr := errB.String()

			if err != nil {
				t.Fatalf("Got an error when none was expected: %v", err)
			}

			filenameColor := "lightMagenta"
			if testcase.noDiff {
				filenameColor = "magenta"
			}
			expectedSummaryLine := c.String(fmt.Sprintf(
				"<%s>%s</>: <cyan>%d</> lines & formatted <yellow>%d</>/<yellow>%d</> blocks!",
				filenameColor,
				testcase.sourcefile,
				testcase.lineCount,
				testcase.updatedBlockCount,
				testcase.totalBlockCount,
			))

			trimmedStdErr := strings.TrimSpace(actualStdErr)
			lines := strings.Split(trimmedStdErr, "\n")
			summaryLine := lines[len(lines)-1]
			if summaryLine != expectedSummaryLine {
				t.Errorf("Unexpected summary:\nexpected %s\ngot      %s", expectedSummaryLine, summaryLine)
			}
		})
	}
}
