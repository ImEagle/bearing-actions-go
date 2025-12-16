package uml

type Options struct {
	IncludeTests     bool
	IncludeGenerated bool
	ExcludeDirNames  []string

	Indent string
}

func (o Options) withDefaults() Options {
	if len(o.ExcludeDirNames) == 0 {
		o.ExcludeDirNames = []string{
			".git",
			".idea",
			".vscode",
			"node_modules",
			"testdata",
			"vendor",
		}
	}
	if o.Indent == "" {
		o.Indent = "  "
	}
	return o
}
