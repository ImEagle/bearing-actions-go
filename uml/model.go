package uml

import "time"

type Model struct {
	GeneratedAt time.Time `json:"generated_at"`
	Root        string    `json:"root"`
	Module      *Module   `json:"module,omitempty"`
	Packages    []Package `json:"packages"`
}

type Module struct {
	Path string `json:"path"`
	Dir  string `json:"dir"`
}

type Package struct {
	Name       string     `json:"name"`
	ImportPath string     `json:"import_path,omitempty"`
	Dir        string     `json:"dir"`
	Files      []string   `json:"files,omitempty"`
	Types      []Type     `json:"types,omitempty"`
	Functions  []Function `json:"functions,omitempty"`
}

type TypeKind string

const (
	TypeKindStruct    TypeKind = "struct"
	TypeKindInterface TypeKind = "interface"
	TypeKindAlias     TypeKind = "alias"
	TypeKindOther     TypeKind = "other"
)

type Type struct {
	Name       string      `json:"name"`
	Kind       TypeKind    `json:"kind"`
	Exported   bool        `json:"exported"`
	Doc        string      `json:"doc,omitempty"`
	TypeParams []TypeParam `json:"type_params,omitempty"`

	Fields   []Field    `json:"fields,omitempty"`
	Embedded []string   `json:"embedded,omitempty"`
	Methods  []Function `json:"methods,omitempty"`
}

type Field struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Tag      string `json:"tag,omitempty"`
	Embedded bool   `json:"embedded,omitempty"`
	Exported bool   `json:"exported,omitempty"`
}

type Function struct {
	Name       string      `json:"name"`
	Exported   bool        `json:"exported"`
	Doc        string      `json:"doc,omitempty"`
	Receiver   string      `json:"receiver,omitempty"`
	TypeParams []TypeParam `json:"type_params,omitempty"`
	Params     []Param     `json:"params,omitempty"`
	Results    []Param     `json:"results,omitempty"`
	Variadic   bool        `json:"variadic,omitempty"`
}

type Param struct {
	Name string `json:"name,omitempty"`
	Type string `json:"type"`
}

type TypeParam struct {
	Name       string `json:"name"`
	Constraint string `json:"constraint,omitempty"`
}
