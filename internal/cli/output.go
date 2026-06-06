package cli

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/spf13/cobra"
)

// output wraps the output stream plus the chosen format. Use
// newOutput(cmd) to get one wired to the current command.
type output struct {
	writer io.Writer
	json   bool
}

func newOutput(cmd *cobra.Command) *output {
	return &output{writer: cmd.OutOrStdout(), json: flagsFromCmd(cmd).JSON}
}

// JSON writes v as indented JSON followed by a newline.
func (o *output) JSON(v any) error {
	if !o.json {
		return nil
	}
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	_, err = o.writer.Write(data)
	return err
}

// Printf writes a formatted line to the output. Always goes to the
// writer regardless of --json; commands gate data dumps on JSON()
// and use Printf for human-readable banners.
func (o *output) Printf(format string, args ...any) error {
	_, err := fmt.Fprintf(o.writer, format, args...)
	return err
}

// Println writes a single line.
func (o *output) Println(args ...any) error {
	_, err := fmt.Fprintln(o.writer, args...)
	return err
}

// IsJSON reports whether the user requested JSON output. Commands
// can use it to decide whether to also write a text summary.
func (o *output) IsJSON() bool { return o.json }


