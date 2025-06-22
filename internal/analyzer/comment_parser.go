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

	// Join the description lines into a single string.
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