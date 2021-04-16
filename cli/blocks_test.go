package cli

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"

	c "github.com/gookit/color"
	"github.com/katbyte/terrafmt/lib/common"
	"github.com/katbyte/terrafmt/lib/fmtverbs"
	"github.com/kylelemons/godebug/diff"
	"github.com/spf13/afero"
)

type block struct {
	startLine int
	endLine   int
	text      string
}

var blocksTestcases = []struct {
	name           string
	sourcefile     string
	lineCount      int
	expectedBlocks []block
}{
	{
		name:       "Go no change",
		sourcefile: "testdata/no_diffs.go",
		lineCount:  29,
		expectedBlocks: []block{
			{
				startLine: 8,
				endLine:   12,
				text: `resource "aws_s3_bucket" "simple" {
  bucket = "tf-test-bucket-simple"
}
`,
			},
			{
				startLine: 16,
				endLine:   20,
				text: `resource "aws_s3_bucket" "with-parameters" {
  bucket = "tf-test-bucket-with-parameters-%d"
}
`,
			},
			{
				startLine: 24,
				endLine:   28,
				text: `resource "aws_s3_bucket" "with-parameters-and-append" {
  bucket = "tf-test-bucket-parameters-and-append-%d"
}
`,
			},
		},
	},
	{
		name:       "Go formatting",
		sourcefile: "testdata/has_diffs.go",
		lineCount:  86,
		expectedBlocks: []block{
			{
				startLine: 8,
				endLine:   13,
				text: `resource "aws_s3_bucket" "extra-lines" {
  
  bucket = "tf-test-bucket-extra-lines"
}
`,
			},
			{
				startLine: 18,
				endLine:   22,
				text: `resource "aws_s3_bucket" "no-errors" {
  bucket = "tf-test-bucket-no-errors-%d"
}
`,
			},
			{
				startLine: 26,
				endLine:   30,
				text: `resource "aws_s3_bucket" "extra-space" {
  bucket    = "tf-test-bucket-extra-space-%d"
}
`,
			},
			{
				startLine: 34,
				endLine:   38,
				text: `resource "aws_s3_bucket" "end-line" {
  bucket = "tf-test-bucket-end-line-%d"
}
`,
			},
			{
				startLine: 42,
				endLine:   77,
				text: `resource "aws_alb_target_group" "test" {
  name = "%s"
  port = 443
  protocol = "HTTPS"
  vpc_id = "${aws_vpc.test.id}"

  deregistration_delay = 200

  stickiness {
    type = "lb_cookie"
    cookie_duration = 10000
  }

  health_check {
    path = "/health"
    interval = 60
    port = 8081
    protocol = "HTTP"
    timeout = 3
    healthy_threshold = 3
    unhealthy_threshold = 3
    matcher = "200-299"
  }

  tags = {
    TestName = "TestAccAWSALBTargetGroup_basic"
  }
}

resource "aws_vpc" "test" {
  cidr_block = "10.0.0.0/16"

  tags = {
    Name = "terraform-testacc-alb-target-group-basic"
  }
}`,
			},
			{
				startLine: 81,
				endLine:   85,
				text: `    resource "aws_s3_bucket" "leading-space" {
  bucket = "tf-test-bucket-leading-space-%d"
}
`,
			},
		},
	},
	{
		name:       "Go fmt verbs",
		sourcefile: "testdata/fmt_compat.go",
		lineCount:  41,
		expectedBlocks: []block{
			{
				startLine: 8,
				endLine:   18,
				text: `resource "aws_s3_bucket" "no-errors" {
  bucket = "tf-test-bucket-no-errors-%d"

  %s

  tags = {
    %[1]q = %[2]q
  }
}
`,
			},
			{
				startLine: 22,
				endLine:   26,
				text: `resource "aws_s3_bucket" "absolutely-nothing" {
  bucket = "tf-test-bucket-absolutely-nothing"
}
`,
			},
			{
				startLine: 30,
				endLine:   40,
				text: `resource "aws_s3_bucket" "extra-space" {
  bucket    = "tf-test-bucket-extra-space-%d"

  %s

  tags = {
    %[1]q    = %[2]q
  }
}
`,
			},
		},
	},
	{
		name:       "Go bad terraform",
		sourcefile: "testdata/bad_terraform.go",
		lineCount:  20,
		expectedBlocks: []block{
			{
				startLine: 8,
				endLine:   12,
				text: `rrrrrresource "aws_s3_bucket" "rrrrrrr" {
  bucket =    "tf-test-bucket"
}
`,
			},
			{
				startLine: 16,
				endLine:   19,
				text: `resource "aws_s3_bucket" "unclosed" {
  bucket =    "tf-test-bucket"
`,
			},
		},
	},
	{
		name:       "Go unsupported format verbs",
		sourcefile: "testdata/unsupported_fmt.go",
		lineCount:  21,
		expectedBlocks: []block{
			{
				startLine: 8,
				endLine:   20,
				text: `resource "aws_s3_bucket" "multi-verb" {
  bucket =    "tf-test-bucket"

  tags = {
    %[1]q =    %[2]q
    Test  =  "${%[5]s.name}"
    Name  =       "${%s.name}"
    byte       = "${aws_acm_certificate.test.*.arn[%[2]d]}"
    Data  =    "${data.%s.name}"
  }
}
`,
			},
		},
	},
	{
		name:       "Markdown no change",
		sourcefile: "testdata/no_diffs.md",
		lineCount:  25,
		expectedBlocks: []block{
			{
				startLine: 3,
				endLine:   7,
				text: `resource "aws_s3_bucket" "one" {
  bucket = "tf-test-bucket-one"
}
`,
			},
			{
				startLine: 9,
				endLine:   13,
				text: `resource "aws_s3_bucket" "two" {
  bucket = "tf-test-bucket-two"
}
`,
			},
			{
				startLine: 15,
				endLine:   19,
				text: `resource "aws_s3_bucket" "three" {
  bucket = "tf-test-bucket-three"
}
`,
			},
		},
	},
	{
		name:       "Markdown formatting",
		sourcefile: "testdata/has_diffs.md",
		lineCount:  33,
		expectedBlocks: []block{
			{
				startLine: 3,
				endLine:   8,
				text: `resource "aws_s3_bucket" "extra-lines" {
  
  bucket = "tf-test-bucket-extra-lines"
}
`,
			},
			{
				startLine: 10,
				endLine:   14,
				text: `resource "aws_s3_bucket" "no-errors" {
  bucket = "tf-test-bucket-no-errors"
}
`,
			},
			{
				startLine: 16,
				endLine:   20,
				text: `resource "aws_s3_bucket" "extra-space" {
  bucket    = "tf-test-bucket-extra-space"
}
`,
			},
			{
				startLine: 22,
				endLine:   27,
				text: `resource "aws_s3_bucket" "end-line" {
  bucket = "tf-test-bucket-end-line"
}
  
`,
			},
			{
				startLine: 29,
				endLine:   33,
				text: `     resource "aws_s3_bucket" "leading-space" {
  bucket = "tf-test-bucket-leading-space"
}
`,
			},
		},
	},
	{
		name:           "Markdown no blocks",
		sourcefile:     "testdata/no_blocks.md",
		lineCount:      3,
		expectedBlocks: []block{},
	},
}

func TestCmdBlocksDefault(t *testing.T) {
	t.Parallel()

	for _, testcase := range blocksTestcases {
		testcase := testcase
		t.Run(testcase.name, func(t *testing.T) {
			t.Parallel()

			fs := afero.NewReadOnlyFs(afero.NewOsFs())

			expectedBuilder := strings.Builder{}
			for i, block := range testcase.expectedBlocks {
				fmt.Fprint(&expectedBuilder, c.Sprintf("\n<white>#######</> <cyan>B%d</><darkGray> @ #%d</>\n", i+1, block.endLine))
				fmt.Fprint(&expectedBuilder, block.text)
			}
			expected := expectedBuilder.String()

			var outB strings.Builder
			var errB strings.Builder
			log := common.CreateLogger(&errB)
			err := findBlocksInFile(fs, log, testcase.sourcefile, false, false, false, false, nil, &outB, &errB)
			actualStdOut := outB.String()
			actualStdErr := errB.String()

			if err != nil {
				t.Fatalf("Got an error when none was expected: %v", err)
			}

			if actualStdOut != expected {
				t.Errorf("Output does not match expected: ('-' actual, '+' expected)\n%s", diff.Diff(actualStdOut, expected))
			}

			if actualStdErr != "" {
				t.Errorf("Got error output:\n%s", actualStdErr)
			}
		})
	}
}

func TestCmdBlocksVerbose(t *testing.T) {
	t.Parallel()

	for _, testcase := range blocksTestcases {
		testcase := testcase
		t.Run(testcase.name, func(t *testing.T) {
			t.Parallel()

			fs := afero.NewReadOnlyFs(afero.NewOsFs())

			var outB strings.Builder
			var errB strings.Builder
			log := common.CreateLogger(&errB)
			err := findBlocksInFile(fs, log, testcase.sourcefile, true, false, false, false, nil, &outB, &errB)
			actualStdErr := errB.String()
			if err != nil {
				t.Fatalf("Got an error when none was expected: %v", err)
			}

			expectedSummaryLine := c.String(fmt.Sprintf("Finished processing <cyan>%d</> lines <yellow>%d</> blocks!", testcase.lineCount, len(testcase.expectedBlocks)))

			summaryLine := strings.TrimSpace(actualStdErr)
			if summaryLine != expectedSummaryLine {
				t.Errorf("Unexpected summary:\nexpected %s\ngot      %s", expectedSummaryLine, summaryLine)
			}
		})
	}
}

func TestCmdBlocksZeroTerminated(t *testing.T) {
	t.Parallel()

	for _, testcase := range blocksTestcases {
		testcase := testcase
		t.Run(testcase.name, func(t *testing.T) {
			t.Parallel()

			fs := afero.NewReadOnlyFs(afero.NewOsFs())

			expectedBuilder := strings.Builder{}
			for _, block := range testcase.expectedBlocks {
				fmt.Fprint(&expectedBuilder, block.text, "\x00")
			}
			expected := expectedBuilder.String()

			var outB strings.Builder
			var errB strings.Builder
			log := common.CreateLogger(&errB)
			err := findBlocksInFile(fs, log, testcase.sourcefile, false, true, false, false, nil, &outB, &errB)
			actualStdOut := outB.String()
			actualStdErr := errB.String()

			if err != nil {
				t.Fatalf("Got an error when none was expected: %v", err)
			}

			if actualStdOut != expected {
				t.Errorf("Output does not match expected: ('-' actual, '+' expected)\n%s", diff.Diff(actualStdOut, expected))
			}

			if actualStdErr != "" {
				t.Errorf("Got error output:\n%s", actualStdErr)
			}
		})
	}
}

func TestCmdBlocksJson(t *testing.T) {
	t.Parallel()

	for _, testcase := range blocksTestcases {
		testcase := testcase
		t.Run(testcase.name, func(t *testing.T) {
			t.Parallel()

			fs := afero.NewReadOnlyFs(afero.NewOsFs())

			data := Output{}
			for _, block := range testcase.expectedBlocks {
				data.BlockCount++
				blockData := Block{
					StartLine: block.startLine,
					EndLine:   block.endLine,
					Text:      block.text,
				}
				data.Blocks = append(data.Blocks, blockData)
			}
			expected, err := json.Marshal(data)
			if err != nil {
				t.Fatalf("Error generating expected JSON output: %v", err)
			}

			var outB strings.Builder
			var errB strings.Builder
			log := common.CreateLogger(&errB)
			err = findBlocksInFile(fs, log, testcase.sourcefile, false, false, true, false, nil, &outB, &errB)
			actualStdOut := outB.String()
			actualStdErr := errB.String()

			if err != nil {
				t.Fatalf("Got an error when none was expected: %v", err)
			}

			if !equivalentJSON([]byte(actualStdOut), expected) {
				t.Errorf("Output does not match expected: ('-' actual, '+' expected)\n%s", diff.Diff(actualStdOut, string(expected)))
			}

			if actualStdErr != "" {
				t.Errorf("Got error output:\n%s", actualStdErr)
			}
		})
	}
}

func TestCmdBlocksFmtVerbsJson(t *testing.T) {
	t.Parallel()

	for _, testcase := range blocksTestcases {
		testcase := testcase
		t.Run(testcase.name, func(t *testing.T) {
			t.Parallel()

			fs := afero.NewReadOnlyFs(afero.NewOsFs())

			data := Output{}
			for _, block := range testcase.expectedBlocks {
				data.BlockCount++
				blockData := Block{
					StartLine: block.startLine,
					EndLine:   block.endLine,
					Text:      fmtverbs.Escape(block.text),
				}
				data.Blocks = append(data.Blocks, blockData)
			}
			expected, err := json.Marshal(data)
			if err != nil {
				t.Fatalf("Error generating expected JSON output: %v", err)
			}

			var outB strings.Builder
			var errB strings.Builder
			log := common.CreateLogger(&errB)
			err = findBlocksInFile(fs, log, testcase.sourcefile, false, false, true, true, nil, &outB, &errB)
			actualStdOut := outB.String()
			actualStdErr := errB.String()

			if err != nil {
				t.Fatalf("Got an error when none was expected: %v", err)
			}

			if !equivalentJSON([]byte(actualStdOut), expected) {
				t.Errorf("Output does not match expected: ('-' actual, '+' expected)\n%s", diff.Diff(actualStdOut, string(expected)))
			}

			if actualStdErr != "" {
				t.Errorf("Got error output:\n%s", actualStdErr)
			}
		})
	}
}

func equivalentJSON(b1, b2 []byte) bool {
	var o1 interface{}
	if err := json.Unmarshal(b1, &o1); err != nil {
		return false
	}

	var o2 interface{}
	if err := json.Unmarshal(b2, &o2); err != nil {
		return false
	}

	return reflect.DeepEqual(o1, o2)
}

func TestBlocksOutputJsonSerializesEmptyArray(t *testing.T) {
	expected := `{"block_count":0,"blocks":[]}`

	actual, err := json.Marshal(&Output{})
	if err != nil {
		t.Fatalf("Error marshalling JSON output: %v", err)
	}

	if string(actual) != expected {
		t.Errorf("Unexpected JSON output:\nexpected %s\ngot      %s", expected, string(actual))
	}
}
