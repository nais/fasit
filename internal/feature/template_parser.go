package feature

import (
	"fmt"
	"slices"
	"strings"
	"text/template"
	"text/template/parse"

	"github.com/nais/fasit/internal/graph/model"
)

type FeatureTemplateDetails struct {
	Management []string
	Env        []string
	Envs       []string
	All        []string
	Functions  []string
}

func (e *FeatureTemplateDetails) addIdent(s *scope, ident []string) {
	if len(ident) == 0 {
		return
	}

	if strings.HasPrefix(ident[0], "$") {
		n := s.Get(ident[0])
		if len(n) > 0 {
			if len(n) == 1 && n[0] == "" {
				ident = ident[1:]
			} else {
				ident = append(n, ident[1:]...)
			}
		}
	}

	e.All = append(e.All, strings.Join(ident, "."))

	if len(ident) > 1 {
		switch ident[0] {
		case "Management":
			e.Management = append(e.Management, strings.Join(ident[1:], "."))
		case "Env":
			e.Env = append(e.Env, strings.Join(ident[1:], "."))
		case "Envs":
			e.Envs = append(e.Envs, strings.Join(ident[1:], "."))
		}
	}
}

func (e *FeatureTemplateDetails) addFunction(name string) {
	e.Functions = append(e.Functions, name)
}

func (e *FeatureTemplateDetails) clean() *FeatureTemplateDetails {
	su := func(v []string) []string {
		m := map[string]struct{}{}
		for _, s := range v {
			m[s] = struct{}{}
		}
		v = []string{}
		for k := range m {
			v = append(v, k)
		}

		slices.Sort(v)
		return v
	}

	return &FeatureTemplateDetails{
		Management: su(e.Management),
		Env:        su(e.Env),
		Envs:       su(e.Envs),
		All:        su(e.All),
		Functions:  su(e.Functions),
	}
}

func ParseTemplateDetails(content model.Values) (*FeatureTemplateDetails, error) {
	eviu := &FeatureTemplateDetails{}

	for _, cv := range content {
		if cv.Computed != nil {
			if err := parseComputedDetails(eviu, cv.Computed); err != nil {
				return nil, fmt.Errorf("parse computed: %w", err)
			}
		}
	}

	return eviu.clean(), nil
}

func parseComputedDetails(eviu *FeatureTemplateDetails, cv *model.Computed) error {
	t, err := template.New("tpl").Funcs(templateFuncs).Parse(cv.Template)
	if err != nil {
		return fmt.Errorf("parse: %w", err)
	}

	unknownNodes := []parse.Node{}

	var walk func(s *scope, node parse.Node)
	walk = func(s *scope, node parse.Node) {
		switch n := node.(type) {
		case *parse.ListNode:
			if n == nil {
				return
			}

			for _, c := range n.Nodes {
				walk(s, c)
			}
		case *parse.ActionNode:
			walk(s, n.Pipe)
		case *parse.PipeNode:
			for _, c := range n.Decl {
				walk(s, c)
			}
			for _, c := range n.Cmds {
				walk(s, c)
			}
		case *parse.CommandNode:
			for _, c := range n.Args {
				walk(s, c)
			}
		case *parse.FieldNode:
			eviu.addIdent(s, n.Ident)
		case *parse.IfNode:
			walk(s, n.Pipe)
			walk(s.New(), n.List)
			walk(s.New(), n.ElseList)
		case *parse.RangeNode:
			walk(s, &n.BranchNode)
		case *parse.WithNode:
			scope := s.New()
			scope.AddPipe(n.Pipe)
			walk(scope, n.List)
		case *parse.IdentifierNode:
			eviu.addFunction(n.Ident)
		case *parse.VariableNode:
			eviu.addIdent(s, n.Ident)
		case *parse.BranchNode:
			scope := s.New()
			scope.AddPipe(n.Pipe)
			walk(scope, n.List)
			walk(scope, n.ElseList)
		case *parse.TemplateNode:
			// ignore
		case *parse.TextNode:
			// ignore
		case *parse.StringNode:
			// ignore
		case *parse.BoolNode:
			// ignore
		case *parse.NumberNode:
			// ignore
		case *parse.DotNode:
			// ignore
		case *parse.ChainNode:
			// ignore
		case *parse.NilNode:
		// ignore
		case *parse.ContinueNode:
			// ignore
		case *parse.BreakNode:
			// ignore
		default:
			unknownNodes = append(unknownNodes, n)
		}
	}

	walk(&scope{}, t.Root)

	if len(unknownNodes) > 0 {
		return fmt.Errorf("unknown nodes: %#v", unknownNodes)
	}

	return nil
}

type scope struct {
	root       *scope
	references map[string][]string
}

func (s *scope) String() string {
	sb := &strings.Builder{}
	s.writeTo(sb)
	return sb.String()
}

func (s *scope) writeTo(sb *strings.Builder) {
	if s == nil {
		return
	}
	if s.root != nil {
		s.root.writeTo(sb)
	}
	for name, target := range s.references {
		if name != "" {
			sb.WriteString(name)
			sb.WriteString(" -> .")
			sb.WriteString(strings.Join(target, ", "))
			sb.WriteString("\n")
		} else {
			sb.WriteString(".")
			sb.WriteString(" -> .")
			sb.WriteString(strings.Join(target, "."))
			sb.WriteString("\n")
		}
	}
}

func (s *scope) Add(name string, target ...string) {
	if s.references == nil {
		s.references = map[string][]string{}
	}

	s.references[name] = target
}

func (s *scope) AddPipe(pipe *parse.PipeNode) {
	if pipe == nil {
		return
	}

	if len(pipe.Decl) > 1 {
		return
		// TODO: Add support for this
		// panic("more than one pipe decl not supported")
	}

	if len(pipe.Cmds) > 1 {
		panic("more than one pipe cmd not supported")
	}
	if len(pipe.Cmds) == 0 {
		return
	}
	cmd := pipe.Cmds[0]

	if len(pipe.Decl) == 0 {
		s.Add("", toSlice(cmd.Args[0])...)
		return
	}

	s.Add(pipe.Decl[0].Ident[0], toSlice(cmd.Args[0])...)
}

func (s *scope) Get(name string) []string {
	if s == nil {
		return nil
	}

	if s.references == nil {
		return nil
	}

	if len(s.references[name]) > 0 {
		return s.references[name]
	}

	return s.root.Get(name)
}

func (s *scope) New() *scope {
	return &scope{root: s}
}

func toSlice(s fmt.Stringer) []string {
	return strings.Split(strings.Trim(s.String(), "."), ".")
}
