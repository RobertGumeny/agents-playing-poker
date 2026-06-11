package replay

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"text/template"
)

//go:embed template/replay.html.tmpl
var replayTemplate string

var tmpl = template.Must(template.New("replay").Parse(replayTemplate))

// Render writes a self-contained HTML replay of m to w. The model is marshaled to
// JSON and inlined into a <script type="application/json"> block; encoding/json's
// default HTML escaping (<, >, & -> \u00xx) makes that JSON safe to embed in a
// script element, so a plain text/template injection cannot break out.
func Render(m ReplayModel, w io.Writer) error {
	data, err := json.Marshal(m)
	if err != nil {
		return fmt.Errorf("render replay: marshal model: %w", err)
	}
	return tmpl.Execute(w, map[string]any{
		"Title":     m.Session.SessionID,
		"ModelJSON": string(data),
	})
}
