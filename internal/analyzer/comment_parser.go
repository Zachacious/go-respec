package analyzer

import (
	"go/ast"
	"strings"
)

// ParsedComment holds the structured data extracted from a doc comment.
type ParsedComment struct {
	// Summary is a brief summary of the comment.
	Summary string
	// Description is a longer description of the comment.
	Description string
	// Tags are a list of tags extracted from the comment.
	Tags []string
	// SummaryExplicit reports whether Summary came from an @summary line rather
	// than from the first line of prose. An explicit summary is used verbatim;
	// a derived one is reworded for documentation.
	SummaryExplicit bool
}

// HumanizeSummary turns a Go doc comment's opening line into something that
// reads as an API summary.
//
// Go convention is for a comment to begin with the identifier it documents, so
// handlers yield "ListBooks returns the catalogue." Rendered next to a path in
// an API browser that is noise, and every operation ends up prefixed with a
// name the reader can already see. Dropping the leading identifier gives
// "Returns the catalogue."
func HumanizeSummary(summary, funcName string) string {
	if summary == "" || funcName == "" {
		return summary
	}

	rest, ok := strings.CutPrefix(summary, funcName+" ")
	if !ok {
		return summary
	}
	rest = strings.TrimSpace(rest)
	if rest == "" {
		return summary
	}

	// Only capitalise when the next word is ordinary prose. A word that is
	// already capitalised, or that names an identifier, is left alone.
	runes := []rune(rest)
	if runes[0] >= 'a' && runes[0] <= 'z' {
		runes[0] = runes[0] - 'a' + 'A'
	}
	return string(runes)
}

// parseDocComment inspects a comment group and extracts metadata.
// It recognizes simple tags like @summary and @tags.
func parseDocComment(doc *ast.CommentGroup) *ParsedComment {
	if doc == nil {
		return nil
	}

	var summary, description string
	var tags []string
	var descriptionLines []string
	var summaryExplicit bool

	for _, comment := range doc.List {
		// Remove leading "//" and trim whitespace from the comment line.
		line := strings.TrimSpace(strings.TrimPrefix(comment.Text, "//"))

		// Check if the line starts with a tag.
		if strings.HasPrefix(line, "@") {
			// Split the line into the tag name and value.
			parts := strings.SplitN(line, " ", 2)
			tagName := parts[0]
			var value string
			if len(parts) > 1 {
				value = strings.TrimSpace(parts[1])
			}

			switch tagName {
			case "@summary":
				// Set the summary from the @summary tag.
				summary = value
				summaryExplicit = true
			case "@tags":
				// Split the @tags value into individual tags and trim whitespace.
				tags = strings.Split(value, ",")
				for i, tag := range tags {
					tags[i] = strings.TrimSpace(tag)
				}
			}
		} else {
			// Add the line to the description.
			descriptionLines = append(descriptionLines, line)
		}
	}

	// If no explicit @summary is found, the first line acts as the summary and
	// the remainder becomes the description. Keeping the first line in both
	// fields made every operation repeat itself.
	if summary == "" && len(descriptionLines) > 0 {
		summary = descriptionLines[0]
		descriptionLines = descriptionLines[1:]
	}

	// Drop blank lines left at either end after the summary was taken out.
	for len(descriptionLines) > 0 && strings.TrimSpace(descriptionLines[0]) == "" {
		descriptionLines = descriptionLines[1:]
	}
	for len(descriptionLines) > 0 && strings.TrimSpace(descriptionLines[len(descriptionLines)-1]) == "" {
		descriptionLines = descriptionLines[:len(descriptionLines)-1]
	}

	description = strings.Join(descriptionLines, "\n")

	return &ParsedComment{
		Summary:         summary,
		Description:     description,
		Tags:            tags,
		SummaryExplicit: summaryExplicit,
	}
}
