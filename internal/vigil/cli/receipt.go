package cli

import (
	"fmt"
	"strconv"

	"github.com/ceasarb/trovery-tools/internal/vigil/receipt"
	"github.com/spf13/cobra"
)

var receiptCmd = &cobra.Command{
	Use:   "receipt [run-id]",
	Short: "Print the receipt for a recorded Lumi run",
	Long: `Print a person-readable receipt for a run a Lumi harness recorded.

The receipt is read from Lumi's own store (read-only) and states exactly what
the harness recorded: what was asked, how it ended, and what it cost. With no
run-id, the most recent run is used.`,
	Args: cobra.MaximumNArgs(1),
	RunE: runReceipt,
}

var (
	receiptStore  string
	receiptFormat string
)

func init() {
	receiptCmd.Flags().StringVar(&receiptStore, "store", "",
		"Path to the Lumi store (default: ~/.lumi/lumi.db)")
	receiptCmd.Flags().StringVar(&receiptFormat, "format", "text",
		"Output format: text, markdown")
	rootCmd.AddCommand(receiptCmd)
}

func runReceipt(cmd *cobra.Command, args []string) error {
	path := receiptStore
	if path == "" {
		path = receipt.DefaultLumiStore()
	}

	store, err := receipt.OpenLumi(path)
	if err != nil {
		return err
	}
	defer store.Close()

	var r receipt.Receipt
	if len(args) == 1 {
		id, err := strconv.ParseInt(args[0], 10, 64)
		if err != nil {
			return fmt.Errorf("run-id must be a number, got %q", args[0])
		}
		r, err = store.Run(id)
		if err != nil {
			return err
		}
	} else {
		r, err = store.LatestRun()
		if err != nil {
			return err
		}
	}

	switch receiptFormat {
	case "text":
		fmt.Print(receipt.RenderText(r))
	case "markdown", "md":
		fmt.Print(receipt.RenderMarkdown(r))
	default:
		return fmt.Errorf("unknown format %q (use text or markdown)", receiptFormat)
	}
	return nil
}
