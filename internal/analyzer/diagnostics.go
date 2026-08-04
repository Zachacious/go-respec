package analyzer

import (
	"fmt"
	"go/token"
	"sort"

	"github.com/Zachacious/go-respec/internal/model"
)

// Severity classifies how much a diagnostic affects the generated spec.
type Severity int

const (
	// SeverityWarning marks something respec could not fully resolve. The spec
	// is still produced, but part of it is missing or less precise.
	SeverityWarning Severity = iota
	// SeverityInfo reports a resolution respec made that the user may want to
	// verify, such as following a handler through a factory function.
	SeverityInfo
)

func (s Severity) String() string {
	if s == SeverityInfo {
		return "info"
	}
	return "warning"
}

// Diagnostic is a single message about the analysis, anchored to source.
//
// Every branch where the analyzer gives up on a route emits one of these.
// Before they existed, an unresolvable handler produced an empty spec with no
// explanation, which is what made github.com/Zachacious/go-respec issue #1 hard
// to act on: the reporter could see the routes were missing but not why.
type Diagnostic struct {
	Severity Severity
	Pos      token.Pos
	Message  string
	// Location is the resolved "file:line:col", filled in by Render.
	Location string
}

// reportf records a warning anchored at pos.
func (s *State) reportf(pos token.Pos, format string, args ...any) {
	s.Diagnostics = append(s.Diagnostics, Diagnostic{
		Severity: SeverityWarning,
		Pos:      pos,
		Message:  fmt.Sprintf(format, args...),
	})
}

// infof records an informational diagnostic anchored at pos.
func (s *State) infof(pos token.Pos, format string, args ...any) {
	s.Diagnostics = append(s.Diagnostics, Diagnostic{
		Severity: SeverityInfo,
		Pos:      pos,
		Message:  fmt.Sprintf(format, args...),
	})
}

// toModelDiagnostics converts analyzer diagnostics into the transport form the
// rest of the pipeline and the CLI consume.
func toModelDiagnostics(in []Diagnostic) []model.Diagnostic {
	if len(in) == 0 {
		return nil
	}
	out := make([]model.Diagnostic, 0, len(in))
	for _, d := range in {
		out = append(out, model.Diagnostic{
			Severity: d.Severity.String(),
			Location: d.Location,
			Message:  d.Message,
		})
	}
	return out
}

// Render resolves positions to file locations and returns the diagnostics in a
// stable order, so output does not churn between runs.
func (s *State) Render() []Diagnostic {
	out := make([]Diagnostic, len(s.Diagnostics))
	copy(out, s.Diagnostics)

	for i := range out {
		if out[i].Pos.IsValid() && s.Fset != nil {
			out[i].Location = s.Fset.Position(out[i].Pos).String()
		}
	}

	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Severity != out[j].Severity {
			return out[i].Severity < out[j].Severity
		}
		if out[i].Location != out[j].Location {
			return out[i].Location < out[j].Location
		}
		return out[i].Message < out[j].Message
	})
	return out
}
