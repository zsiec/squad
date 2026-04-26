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
