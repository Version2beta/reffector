package output

import (
	"encoding/json"
	"fmt"
	"io"

	"reffector/internal/analyzer"
)

func Write(w io.Writer, records []analyzer.Record, asJSON bool) error {
	if asJSON {
		if records == nil {
			records = []analyzer.Record{}
		}
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(records)
	}
	for _, r := range records {
		for _, effect := range r.Effects {
			if _, err := fmt.Fprintf(w, "%s.%s %s\n", r.Package, r.Function, effect.Identifier); err != nil {
				return err
			}
		}
	}
	return nil
}
