package cli

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/token"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/andreyvit/diff"
	c "github.com/gookit/color"
	"github.com/hashicorp/go-multierror"
	"github.com/katbyte/terrafmt/lib/blocks"
	"github.com/katbyte/terrafmt/lib/common"
	verbs "github.com/katbyte/terrafmt/lib/fmtverbs"
	"github.com/katbyte/terrafmt/lib/format"
	"github.com/katbyte/terrafmt/lib/upgrade012"
	"github.com/katbyte/terrafmt/lib/version"
	"github.com/sirupsen/logrus"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

const (
	ExitCodeNoError             = 0
	ExitCodeMiscError           = 1
	ExitCodeBlockParsingError   = 1 << 1
	ExitCodeFormattingDiffError = 1 << 2
)

func Make() *cobra.Command {
	root := &cobra.Command{
		Use:           "terrafmt [fmt|diff|blocks|upgrade012]",
		Short:         "terrafmt is a small utility to format terraform blocks found in files.",
		Long:          `A small utility that formats terraform blocks found in files. Primarily intended to help with terraform provider development.`,
		Args:          cobra.RangeArgs(0, 0),
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return fmt.Errorf("No command specified")
		},
	}

	//options : only count, blocks diff/found, total lines diff, etc
	fmtCmd := &cobra.Command{
		Use:   "fmt [path]",
		Short: "formats terraform blocks in a directory, file, or stdin",
		Args:  cobra.RangeArgs(0, 1),
		RunE: func(cmd *cobra.Command, args []string) error {
			log := common.CreateLogger(cmd.ErrOrStderr())

			path := ""
			if len(args) == 1 {
				path = args[0]
			}
			log.Debugf("terrafmt  %s", path)

			fs := afero.NewOsFs()

			pattern, _ := cmd.Flags().GetString("pattern")
			filenames, err := allFiles(fs, path, pattern)
			if err != nil {
				return err
			}
			fmtCompat := viper.GetBool("fmtcompat")
			fixFinishLines, _ := cmd.Flags().GetBool("fix-finish-lines")
			verbose := viper.GetBool("verbose")

			var errs *multierror.Error
			exitCode := ExitCodeNoError

			for _, filename := range filenames {
				br, err := formatFile(fs, log, filename, fmtCompat, fixFinishLines, verbose, cmd.InOrStdin(), cmd.OutOrStdout(), cmd.ErrOrStderr())

				if err != nil {
					errs = multierror.Append(errs, err)
				}

				if br.ErrorBlocks > 0 {
					exitCode |= ExitCodeBlockParsingError
				}
			}
			if errs != nil {
				return errs
			}
			if exitCode != ExitCodeNoError {
				os.Exit(exitCode)
			}

			return nil
		},
	}

	root.AddCommand(fmtCmd)
	fmtCmd.Flags().Bool("fix-finish-lines", false, "fix block finish lines by removing any leading spaces")
	fmtCmd.Flags().StringP("pattern", "p", "", "glob pattern to match with each file name (e.g. *.markdown)")

	//options : only count, blocks diff/found, total lines diff, etc
	root.AddCommand(&cobra.Command{
		Use:   "upgrade012 [file]",
		Short: "formats terraform blocks to 0.12 format in a single file or on stdin",
		Args:  cobra.RangeArgs(0, 1),
		RunE: func(cmd *cobra.Command, args []string) error {
			log := common.CreateLogger(cmd.ErrOrStderr())

			filename := ""
			if len(args) == 1 {
				filename = args[0]
			}
			log.Debugf("terrafmt upgrade012 %s", filename)

			fmtverbs := viper.GetBool("fmtcompat")
			verbose := viper.GetBool("verbose")

			fs := afero.NewOsFs()
			br, err := upgrade012File(fs, log, filename, fmtverbs, verbose, cmd.InOrStdin(), cmd.OutOrStdout(), cmd.ErrOrStderr())
			if err != nil {
				return err
			}
			if br.ErrorBlocks > 0 {
				os.Exit(ExitCodeBlockParsingError)
			}

			return nil
		},
	})

	//options : only count, blocks diff/found, total lines diff, etc
	diffCmd := &cobra.Command{
		Use:   "diff [path]",
		Short: "formats terraform blocks in a directory, file, or stdin and shows the difference",
		Args:  cobra.RangeArgs(0, 1),
		RunE: func(cmd *cobra.Command, args []string) error {
			log := common.CreateLogger(cmd.ErrOrStderr())

			path := ""
			if len(args) == 1 {
				path = args[0]
			}
			log.Debugf("terrafmt fmt %s", path)

			fs := afero.NewOsFs()

			pattern, _ := cmd.Flags().GetString("pattern")
			filenames, err := allFiles(fs, path, pattern)
			if err != nil {
				return err
			}

			var errs *multierror.Error
			check := viper.GetBool("check")
			exitCode := ExitCodeNoError

			for _, filename := range filenames {
				br, fileDiff, err := diffFile(fs, log, filename, viper.GetBool("fmtcompat"), viper.GetBool("verbose"), cmd.InOrStdin(), cmd.OutOrStdout(), cmd.ErrOrStderr())
				if err != nil {
					errs = multierror.Append(errs, err)
					continue
				}
				if br.ErrorBlocks > 0 {
					exitCode |= ExitCodeBlockParsingError
				}
				if check && fileDiff {
					exitCode |= ExitCodeFormattingDiffError
				}
			}
			if errs != nil {
				return errs
			}
			if exitCode != ExitCodeNoError {
				os.Exit(exitCode)
			}

			return nil
		},
	}

	root.AddCommand(diffCmd)
	diffCmd.Flags().StringP("pattern", "p", "", "glob pattern to match with each file name (e.g. *.markdown)")

	// options
	blocksCmd := &cobra.Command{
		Use:   "blocks [file]",
		Short: "extracts terraform blocks from a file ",
		//options: no header (######), format (json? xml? etc), only should block x?
		Args: cobra.RangeArgs(0, 1),
		RunE: func(cmd *cobra.Command, args []string) error {
			log := common.CreateLogger(cmd.ErrOrStderr())

			filename := ""
			if len(args) == 1 {
				filename = args[0]
			}
			log.Debugf("terrafmt blocks %s", filename)

			verbose := viper.GetBool("verbose")
			zeroTerminated, _ := cmd.Flags().GetBool("zero-terminated")
			jsonOutput, _ := cmd.Flags().GetBool("json")
			if zeroTerminated && jsonOutput {
				return fmt.Errorf("only one of zero-terminated or json can be specified")
			}
			fmtCompat := viper.GetBool("fmtcompat")

			fs := afero.NewOsFs()
			return findBlocksInFile(fs, log, filename, verbose, zeroTerminated, jsonOutput, fmtCompat, cmd.InOrStdin(), cmd.OutOrStdout(), cmd.ErrOrStderr())
		},
	}
	root.AddCommand(blocksCmd)
	blocksCmd.Flags().BoolP("zero-terminated", "z", false, "outputs blocks separated by null separator")
	blocksCmd.Flags().BoolP("json", "j", false, "outputs blocks in JSON format")

	root.AddCommand(&cobra.Command{
		Use:   "version",
		Short: "Print the version number of terrafmt",
		Args:  cobra.NoArgs,
		Run:   versionCmd,
	})

	pflags := root.PersistentFlags()
	pflags.BoolP("fmtcompat", "f", false, "enable format string (%s, %d etc) compatibility")
	pflags.BoolP("check", "c", false, "return an error during diff if formatting is required")
	pflags.BoolP("verbose", "v", false, "show files as they are processed& additional stats")
	pflags.BoolP("quiet", "q", false, "quiet mode, only shows block line numbers ")

	if err := viper.BindPFlag("fmtcompat", pflags.Lookup("fmtcompat")); err != nil {
		panic(err)
	}
	if err := viper.BindPFlag("check", pflags.Lookup("check")); err != nil {
		panic(err)
	}
	if err := viper.BindPFlag("quiet", pflags.Lookup("quiet")); err != nil {
		panic(err)
	}
	if err := viper.BindPFlag("verbose", pflags.Lookup("verbose")); err != nil {
		panic(err)
	}

	//todo bind to env?

	return root
}

func allFiles(fs afero.Fs, path string, pattern string) ([]string, error) {
	if path == "" {
		return []string{""}, nil
	}

	info, err := fs.Stat(path)

	if err != nil {
		return nil, fmt.Errorf("error reading path (%s): %s", path, err)
	}

	if !info.IsDir() {
		return []string{path}, nil
	}

	var filenames []string

	err = afero.Walk(fs, path,
		func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			if info.IsDir() {
				return nil
			}

			if pattern == "" {
				filenames = append(filenames, path)

				return nil
			}

			matched, err := filepath.Match(pattern, filepath.Base(path))

			if err != nil {
				return err
			}

			if matched {
				filenames = append(filenames, path)
			}

			return nil
		},
	)

	if err != nil {
		return nil, fmt.Errorf("error walking path (%s): %w", path, err)
	}

	return filenames, nil
}

func versionCmd(cmd *cobra.Command, args []string) {
	// nolint errcheck
	fmt.Println("terrafmt v" + version.Version + "-" + version.GitCommit)
}

type textBlockWriter struct {
	writer io.Writer
}

func (w textBlockWriter) Write(index, startLine, endLine int, text string) {
	fmt.Fprint(w.writer, c.Sprintf("\n<white>#######</> <cyan>B%d</><darkGray> @ #%d</>\n", index, endLine))
	fmt.Fprint(w.writer, text)
}

func (w textBlockWriter) Close() error { return nil }

type zeroTerminatedBlockWriter struct {
	writer io.Writer
}

func (w zeroTerminatedBlockWriter) Write(index, startLine, endLine int, text string) {
	fmt.Fprint(w.writer, text)
	fmt.Fprint(w.writer, "\x00")
}

func (w zeroTerminatedBlockWriter) Close() error { return nil }

type Block struct {
	StartLine int    `json:"start_line"`
	EndLine   int    `json:"end_line"`
	Text      string `json:"text"`
}

type Output struct {
	BlockCount int     `json:"block_count"`
	Blocks     []Block `json:"blocks"`
}

// MarshalJSON creates an empty slice if it is nil then marshals to JSON
func (o Output) MarshalJSON() ([]byte, error) {
	type Alias Output // Prevent an infinite loop
	a := struct{ Alias }{Alias: (Alias)(o)}
	if a.Blocks == nil {
		a.Blocks = make([]Block, 0)
	}

	return json.Marshal(a)
}

type jsonBlockWriter struct {
	writer io.Writer
	data   Output
}

func (w *jsonBlockWriter) Write(index, startLine, endLine int, text string) {
	w.data.BlockCount++
	w.data.Blocks = append(w.data.Blocks, Block{
		StartLine: startLine,
		EndLine:   endLine,
		Text:      text,
	})
}

func (w jsonBlockWriter) Close() error {
	encoder := json.NewEncoder(w.writer)
	return encoder.Encode(w.data)
}

func findBlocksInFile(fs afero.Fs, log *logrus.Logger, filename string, verbose, zeroTerminated, jsonOutput, fmtverbs bool, stdin io.Reader, stdout, stderr io.Writer) error {
	var blockWriter blocks.BlockWriter
	if zeroTerminated {
		blockWriter = zeroTerminatedBlockWriter{
			writer: stdout,
		}
	} else if jsonOutput {
		blockWriter = &jsonBlockWriter{
			writer: stdout,
		}
	} else {
		blockWriter = textBlockWriter{
			writer: stdout,
		}
	}
	br := blocks.Reader{
		Log:         log,
		ReadOnly:    true,
		LineRead:    blocks.ReaderIgnore,
		BlockWriter: blockWriter,
		BlockRead: func(br *blocks.Reader, i int, b string) error {
			if fmtverbs {
				b = verbs.Escape(b)
			}
			br.BlockWriter.Write(br.BlockCount, br.LineCount-br.BlockCurrentLine, br.LineCount, b)
			return nil
		},
	}

	err := br.DoTheThing(fs, filename, stdin, stdout)
	if err != nil {
		return err
	}

	if err = blockWriter.Close(); err != nil {
		return fmt.Errorf("error writing blocks output: %w", err)
	}

	if verbose {
		fmt.Fprint(stderr, c.Sprintf("\nFinished processing <cyan>%d</> lines <yellow>%d</> blocks!\n", br.LineCount, br.BlockCount))
	}

	return nil
}

func diffFile(fs afero.Fs, log *logrus.Logger, filename string, fmtverbs, verbose bool, stdin io.Reader, stdout, stderr io.Writer) (*blocks.Reader, bool, error) {
	blocksWithDiff := 0
	br := blocks.Reader{
		Log:      log,
		ReadOnly: true,
		LineRead: blocks.ReaderPassthrough,
		BlockRead: func(br *blocks.Reader, i int, b string) error {
			var fb string
			var err error
			if fmtverbs {
				fb, err = format.FmtVerbBlock(log, b, filename)
			} else {
				fb, err = format.Block(log, b, filename)
			}
			if err != nil {
				return err
			}

			if fb == b {
				return nil
			}
			blocksWithDiff++

			outW := stdout

			fmt.Fprint(outW, c.Sprintf("<lightMagenta>%s</><darkGray>:</><magenta>%d</>\n", br.FileName, br.LineCount-br.BlockCurrentLine))

			if !viper.GetBool("quiet") {
				d := diff.LineDiff(b, fb)
				scanner := bufio.NewScanner(strings.NewReader(d))
				for scanner.Scan() {
					l := scanner.Text()
					if strings.HasPrefix(l, "+") {
						fmt.Fprint(outW, c.Sprintf("<green>%s</>\n", l))
					} else if strings.HasPrefix(l, "-") {
						fmt.Fprint(outW, c.Sprintf("<red>%s</>\n", l))
					} else {
						fmt.Fprint(outW, l+"\n")
					}
				}
			}

			return nil
		},
	}

	err := br.DoTheThing(fs, filename, stdin, stdout)
	if err != nil {
		return nil, false, err
	}

	hasDiff := (blocksWithDiff > 0)

	fc := "magenta"
	if hasDiff {
		fc = "lightMagenta"
	}

	if verbose {
		fmt.Fprint(stderr, c.Sprintf("<%s>%s</>: <cyan>%d</> lines & <yellow>%d</>/<yellow>%d</> blocks need formatting.\n", fc, br.FileName, br.LineCount, blocksWithDiff, br.BlockCount))
	}

	return &br, hasDiff, nil
}

func formatFile(fs afero.Fs, log *logrus.Logger, filename string, fmtverbs, fixFinishLines, verbose bool, stdin io.Reader, stdout, stderr io.Writer) (*blocks.Reader, error) {
	blocksFormatted := 0

	br := blocks.Reader{
		Log:      log,
		LineRead: blocks.ReaderPassthrough,
		BlockRead: func(br *blocks.Reader, i int, b string) error {
			var fb string
			var err error
			if fmtverbs {
				fb, err = format.FmtVerbBlock(log, b, filename)
			} else {
				fb, err = format.Block(log, b, filename)
			}
			if err != nil {
				return err
			}

			hasChange := fb != b

			if br.CurrentNodeCursor != nil {
				fb = strings.TrimSuffix(fb, "\n")

				if br.FixFinishLines {
					trimmed := strings.TrimRight(br.CurrentNodeTrailingPadding, " \t")
					if trimmed != br.CurrentNodeTrailingPadding {
						br.CurrentNodeTrailingPadding = trimmed
						hasChange = true
					}
				}

				if hasChange {
					br.CurrentNodeCursor.Replace(&ast.BasicLit{
						Kind: token.STRING,
						Value: br.CurrentNodeQuoteChar +
							br.CurrentNodeLeadingPadding +
							fb +
							br.CurrentNodeTrailingPadding +
							br.CurrentNodeQuoteChar,
					})
					blocksFormatted++
				}
			} else {
				_, err = br.Writer.Write([]byte(fb))

				if err == nil && hasChange {
					blocksFormatted++
				}
			}

			return err
		},
		FixFinishLines: fixFinishLines,
	}
	err := br.DoTheThing(fs, filename, stdin, stdout)

	fc := "magenta"
	if blocksFormatted > 0 {
		fc = "lightMagenta"
	}

	if verbose {
		fmt.Fprint(stderr, c.Sprintf("<%s>%s</>: <cyan>%d</> lines & formatted <yellow>%d</>/<yellow>%d</> blocks!\n", fc, br.FileName, br.LineCount, blocksFormatted, br.BlockCount))
	}

	return &br, err
}

func upgrade012File(fs afero.Fs, log *logrus.Logger, filename string, fmtverbs, verbose bool, stdin io.Reader, stdout, stderr io.Writer) (*blocks.Reader, error) {
	ctx := context.Background()

	tfBin, err := upgrade012.InstallTerraform(ctx)
	if err != nil {
		return nil, err
	}

	blocksFormatted := 0
	br := blocks.Reader{
		Log:      log,
		LineRead: blocks.ReaderPassthrough,
		BlockRead: func(br *blocks.Reader, i int, b string) error {
			var fb string
			var err error
			if fmtverbs {
				fb, err = upgrade012.Upgrade12VerbBlock(ctx, tfBin, log, b)
			} else {
				fb, err = upgrade012.Block(ctx, tfBin, log, b)
			}
			if err != nil {
				return err
			}

			hasChange := fb != b

			if br.CurrentNodeCursor != nil {
				fb = strings.TrimSuffix(fb, "\n")

				if hasChange {
					br.CurrentNodeCursor.Replace(&ast.BasicLit{
						Kind: token.STRING,
						Value: br.CurrentNodeQuoteChar +
							br.CurrentNodeLeadingPadding +
							fb +
							br.CurrentNodeTrailingPadding +
							br.CurrentNodeQuoteChar,
					})
					blocksFormatted++
				}
			} else {
				_, err = br.Writer.Write([]byte(fb))

				if err == nil && hasChange {
					blocksFormatted++
				}
			}

			return nil
		},
	}
	err = br.DoTheThing(fs, filename, stdin, stdout)
	if err != nil {
		return &br, err
	}

	fc := "magenta"
	if blocksFormatted > 0 {
		fc = "lightMagenta"
	}

	if verbose {
		fmt.Fprint(stderr, c.Sprintf("<%s>%s</>: <cyan>%d</> lines & formatted <yellow>%d</>/<yellow>%d</> blocks!\n", fc, br.FileName, br.LineCount, blocksFormatted, br.BlockCount))
	}

	return &br, err
}
