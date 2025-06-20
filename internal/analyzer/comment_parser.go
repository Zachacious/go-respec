package analyzer

import (
	"go/ast"
	"strings"
)

// ParsedComment holds the structured data extracted from a doc comment.
type ParsedComment struct {
	Summary     string
	Description string
	Tags        []string
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

	for _, comment := range doc.List {
		line := strings.TrimSpace(strings.TrimPrefix(comment.Text, "//"))

		if strings.HasPrefix(line, "@") {
			parts := strings.SplitN(line, " ", 2)
			tagName := parts[0]
			var value string
			if len(parts) > 1 {
				value = strings.TrimSpace(parts[1])
			}

			switch tagName {
			case "@summary":
				summary = value
			case "@tags":
				tags = strings.Split(value, ",")
				for i, tag := range tags {
					tags[i] = strings.TrimSpace(tag)
				}
			}
		} else {
			descriptionLines = append(descriptionLines, line)
		}
	}

	description = strings.Join(descriptionLines, "\n")

	// If no explicit @summary is found, use the first line of the description.
	if summary == "" && len(descriptionLines) > 0 {
		summary = descriptionLines[0]
	}

	return &ParsedComment{
		Summary:     summary,
		Description: description,
		Tags:        tags,
	}
}
