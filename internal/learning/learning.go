// Package learning manages squad's learning artifacts under
// .squad/learnings/{actions,patterns,nots}/{proposed,approved,rejected}/.
// Each learning is a markdown file with a YAML frontmatter header; this
// package owns the parser, the kind taxonomy, and the move/promote helpers
// that drive squad learning approve/reject.
package learning

type Learning struct {
	ID           string   `yaml:"id"`
	Kind         Kind     `yaml:"kind"`
	Slug         string   `yaml:"slug"`
	Title        string   `yaml:"title"`
	Area         string   `yaml:"area"`
	Paths        []string `yaml:"paths"`
	Created      string   `yaml:"created"`
	CreatedBy    string   `yaml:"created_by"`
	Session      string   `yaml:"session"`
	State        State    `yaml:"state"`
	Evidence     []string `yaml:"evidence"`
	RelatedItems []string `yaml:"related_items"`

	Body string `yaml:"-"`
	Path string `yaml:"-"`
}
