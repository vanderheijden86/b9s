package main

import (
 "encoding/json"
 "fmt"
 "os"
 "strings"
 "github.com/vanderheijden86/b9s/pkg/loader"
 "github.com/vanderheijden86/b9s/pkg/ui"
)

func main() {
 for _, spec := range []string{"docs/plans/example.md", "/other-vm/plan.md", "https://example.invalid/plan.md", "bd-spec1"} {
  input := fmt.Sprintf(`{"id":"probe-1","title":"Spec only","status":"open","priority":2,"issue_type":"task","spec_id":%q}`, spec)
  issues, err := loader.ParseIssues(strings.NewReader(input))
  if err != nil || len(issues) != 1 { panic(fmt.Sprintf("parse failed: %v, count=%d", err, len(issues))) }
  encoded, err := json.Marshal(issues[0])
  if err != nil { panic(err) }
  var fields map[string]any
  if err := json.Unmarshal(encoded, &fields); err != nil { panic(err) }
  _, retained := fields["spec_id"]
  m := ui.NewModel(issues, "")
  atts := m.AttachmentList()
  fmt.Printf("spec=%q issues=%d spec_id_retained=%t attachments=%d\n", spec, len(issues), retained, len(atts))
  if retained || len(atts) != 0 { os.Exit(1) }
 }
}
