package blocks

import (
	"bufio"
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"io"
	"io/ioutil"
	"regexp"
	"strconv"
	"strings"

	"github.com/sirupsen/logrus"
	"github.com/spf13/afero"
	"golang.org/x/tools/go/ast/astutil"
)

var (
	lineWithLeadingSpacesMatcher = regexp.MustCompile("^[[:space:]]*(.*\n)$")
)

type blockReadFunc func(*Reader, int, string) error

type BlockWriter interface {
	Write(index, startLine, endLine int, text string)
	Close() error
}

type Reader struct {
	FileName string

	Log *logrus.Logger

	//io
	Reader io.Reader
	Writer io.Writer

	//stats
	LineCount        int // total lines processed
	LinesBlock       int // total block lines processed
	BlockCount       int // total blocks found
	BlockCurrentLine int // current block line count

	CurrentNodeCursor          *astutil.Cursor
	CurrentNodeQuoteChar       string
	CurrentNodeLeadingPadding  string
	CurrentNodeTrailingPadding string

	ErrorBlocks int

	//options
	ReadOnly       bool
	FixFinishLines bool

	//callbacks
	LineRead  func(*Reader, int, string) error
	BlockRead blockReadFunc

	// Only used by the "blocks" command
	BlockWriter BlockWriter
}

func ReaderPassthrough(br *Reader, number int, line string) error {
	_, err := br.Writer.Write([]byte(line))
	return err
}

func ReaderIgnore(br *Reader, number int, line string) error {
	return nil
}

func IsStartLine(line string) bool {
	if strings.HasPrefix(line, "```hcl") { // documentation
		return true
	} else if strings.HasPrefix(line, "```terraform") { // documentation
		return true
	} else if strings.HasPrefix(line, "```tf") { // documentation
		return true
	}

	return false
}

func IsFinishLine(line string) bool {
	return strings.HasPrefix(line, "```") // documentation
}

type blockVisitor struct {
	br   *Reader
	fset *token.FileSet
	f    blockReadFunc
}

var leadingPaddingMatcher = regexp.MustCompile(`^\s*\n`)
var trailingPaddingMatcher = regexp.MustCompile(`\n\s*$`)

func (bv blockVisitor) Visit(cursor *astutil.Cursor) bool {
	if node, ok := cursor.Node().(*ast.BasicLit); ok && node.Kind == token.STRING {
		if unquoted, err := strconv.Unquote(node.Value); err == nil && looksLikeTerraform(unquoted) {
			value := strings.Trim(unquoted, " \t")
			value = strings.TrimPrefix(value, "\n")
			if strings.Contains(value, "\n") {
				bv.br.CurrentNodeCursor = cursor
				bv.br.CurrentNodeQuoteChar = node.Value[0:1]
				bv.br.CurrentNodeLeadingPadding = leadingPaddingMatcher.FindString(unquoted)
				bv.br.CurrentNodeTrailingPadding = trailingPaddingMatcher.FindString(unquoted)
				bv.br.BlockCount++
				bv.br.LineCount = bv.fset.Position(node.End()).Line
				// This is to deal with some outputs using just LineCount and some using LineCount-BlockCurrentLine
				bv.br.BlockCurrentLine = bv.fset.Position(node.End()).Line - bv.fset.Position(node.Pos()).Line
				err := bv.f(bv.br, 0, value)
				if err != nil {
					bv.br.ErrorBlocks++
					bv.br.Log.Errorf("block %d @ %s:%d failed to process with: %v", bv.br.BlockCount, bv.br.FileName, bv.fset.Position(node.Pos()).Line, err)
				}
				return false
			}
		}
	}
	return true
}

var terraformMatcher = regexp.MustCompile(`(((resource|data)\s+"[-a-z0-9_]+")|(variable|output))\s+"[-a-z0-9_]+"\s+\{`)

// A simple check to see if the content looks like a Terraform configuration.
// Looks for a line with either a resource, data source, variable, or output declaration
func looksLikeTerraform(s string) bool {
	return terraformMatcher.MatchString(s)
}

func (br *Reader) DoTheThing(fs afero.Fs, filename string, stdin io.Reader, stdout io.Writer) error {
	inStream := &bytes.Buffer{}
	if filename != "" {
		if !strings.HasSuffix(filename, ".go") {
			return br.doTheThingPatternMatch(fs, filename, stdin, stdout)
		}
	} else {
		tee := io.TeeReader(stdin, inStream)
		teee := bufio.NewReader(tee)
		if matched, err := regexp.MatchReader(`package [a-zA-Z0-9_]+\n`, teee); err != nil {
			return err
		} else if !matched {
			return br.doTheThingPatternMatch(fs, filename, inStream, stdout)
		}
	}

	buf := &strings.Builder{}

	if filename != "" {
		br.FileName = filename
		br.Log.Debugf("opening src file %s", filename)
		file, err := fs.Open(filename) // For read access.
		if err != nil {
			return err
		}
		defer file.Close()
		br.Reader = file

		// for now write to buffer
		if !br.ReadOnly {
			br.Writer = buf
		} else {
			br.Writer = ioutil.Discard
		}
	} else {
		br.FileName = "stdin"
		br.Reader = inStream
		br.Writer = stdout

		if br.ReadOnly {
			br.Writer = ioutil.Discard
		}
	}

	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, filename, br.Reader, parser.ParseComments)
	if err != nil {
		return err
	}
	visitor := blockVisitor{
		br:   br,
		fset: fset,
		f:    br.BlockRead,
	}
	result := astutil.Apply(f, func(cursor *astutil.Cursor) bool {
		return visitor.Visit(cursor)
	}, nil)

	br.LineCount = fset.Position(f.End()).Line // For summary line

	if err := format.Node(buf, fset, result); err != nil {
		return err
	}

	// If not read-only, need to write back to file.
	var destination io.Writer
	if !br.ReadOnly {
		if filename != "" {
			outfile, err := fs.Create(filename)
			if err != nil {
				return err
			}
			defer outfile.Close()
			destination = outfile
		} else {
			destination = stdout
		}
		br.Log.Debugf("copying..")
		_, err = io.WriteString(destination, buf.String())
		return err
	}

	// todo should this be at the end of a command?
	//fmt.Fprintf(os.Stderr, c.Sprintf("\nFinished processing <cyan>%d</> lines <yellow>%d</> blocks!\n", br.LineCount, br.BlockCount))
	return nil
}

func (br *Reader) doTheThingPatternMatch(fs afero.Fs, filename string, stdin io.Reader, stdout io.Writer) error {
	var buf *bytes.Buffer

	if filename != "" {
		br.FileName = filename
		br.Log.Debugf("opening src file %s", filename)
		file, err := fs.Open(filename) // For read access.
		if err != nil {
			return err
		}
		defer file.Close()
		br.Reader = file

		// for now write to buffer
		if !br.ReadOnly {
			buf = bytes.NewBuffer([]byte{})
			br.Writer = buf
		} else {
			br.Writer = ioutil.Discard
		}
	} else {
		br.FileName = "stdin"
		br.Reader = stdin
		br.Writer = stdout

		if br.ReadOnly {
			br.Writer = ioutil.Discard
		}
	}

	br.LineCount = 0
	br.BlockCount = 0
	s := bufio.NewScanner(br.Reader)
	for s.Scan() { // scan file
		br.LineCount += 1
		//br.CurrentLine = s.Text()+"\n"
		l := s.Text() + "\n"

		if err := br.LineRead(br, br.LineCount, l); err != nil {
			return fmt.Errorf("NB LineRead failed @ %s:%d for %s: %v", br.FileName, br.LineCount, l, err)
		}

		if IsStartLine(l) {
			block := ""
			br.BlockCurrentLine = 0
			br.BlockCount += 1

			for s.Scan() { // scan block
				br.LineCount += 1
				br.BlockCurrentLine += 1
				l2 := s.Text() + "\n"

				// make sure we don't run into another block
				if IsStartLine(l2) {
					// the end of current block must be malformed, so lets pass it through and log an error
					br.Log.Errorf("block %d @ %s:%d failed to find end of block", br.BlockCount, br.FileName, br.LineCount-br.BlockCurrentLine)
					if err := ReaderPassthrough(br, br.LineCount, block); err != nil { // is this ok or should we loop with LineRead?
						return err
					}

					if err := br.LineRead(br, br.LineCount, l2); err != nil {
						return fmt.Errorf("NB LineRead failed @ %s:%d for %s: %v", br.FileName, br.LineCount, l, err)
					}

					block = ""
					br.BlockCount += 1
					continue
				}

				if IsFinishLine(l2) {
					if br.FixFinishLines {
						l2 = lineWithLeadingSpacesMatcher.ReplaceAllString(l2, `$1`)
					}

					br.LinesBlock += br.BlockCurrentLine

					// todo configure this behaviour with switch's
					if err := br.BlockRead(br, br.LineCount, block); err != nil {
						//for now ignore block errors and output unformatted
						br.ErrorBlocks += 1
						br.Log.Errorf("block %d @ %s:%d failed to process with: %v", br.BlockCount, br.FileName, br.LineCount-br.BlockCurrentLine, err)
						if err := ReaderPassthrough(br, br.LineCount, block); err != nil {
							return err
						}
					}

					if err := br.LineRead(br, br.LineCount, l2); err != nil {
						return fmt.Errorf("NB LineRead failed @ %s:%d for %s: %v", br.FileName, br.LineCount, l2, err)
					}

					block = ""
					break
				} else {
					block += l2
				}
			}

			// ensure last block in the file was property handled
			if block != "" {
				//for each line { Lineread()?
				br.Log.Errorf("block %d @ %s:%d failed to find end of block", br.BlockCount, br.FileName, br.LineCount-br.BlockCurrentLine)
				if err := ReaderPassthrough(br, br.LineCount, block); err != nil { // is this ok or should we loop with LineRead?
					return err
				}
			}
		}
	}

	// If not read-only, need to write back to file.
	if !br.ReadOnly && filename != "" {
		destination, err := fs.Create(filename)
		if err != nil {
			return err
		}
		defer destination.Close()

		br.Log.Debugf("copying..")
		_, err = io.Copy(destination, buf)
		return err
	}

	// todo should this be at the end of a command?
	//fmt.Fprintf(os.Stderr, c.Sprintf("\nFinished processing <cyan>%d</> lines <yellow>%d</> blocks!\n", br.LineCount, br.BlockCount))
	return nil
}
