package parser

import (
	. "faustlsp/transport"
	"sync"

	tree_sitter_faust "github.com/khiner/tree-sitter-faust/bindings/go"
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
)

// TODO: Need mapping from LSP UTF-16 to TS UTF-8 and vice-versa
// TODO: Tidy up this file
// TODO: Improve DocumentSymbols function
// TODO: Handle Incremental Changes to Trees

type TSParser struct {
	language     *tree_sitter.Language
	parser       *tree_sitter.Parser
	treesToClose []*tree_sitter.Tree
	mu           sync.Mutex
}

var tsParser TSParser

func Init() {
	tsParser.language = tree_sitter.NewLanguage(tree_sitter_faust.Language())
	tsParser.parser = tree_sitter.NewParser()
	tsParser.parser.SetLanguage(tsParser.language)
}

type TSQueryResult struct {
	results map[string][]tree_sitter.Node
}

func ParseTree(code []byte) *tree_sitter.Tree {
	//	tsParser.parser = tree_sitter.NewParser()
	//	tsParser.parser.SetLanguage(tsParser.language)
	tsParser.mu.Lock()
	tree := tsParser.parser.Parse(code, nil)
	//	tsParser.parser.Close()
	tsParser.parser.Reset()
	tsParser.mu.Unlock()
	return tree
}

func DocumentSymbols(tree *tree_sitter.Tree, content []byte) []DocumentSymbol {
	cursor := tree.Walk()
	defer cursor.Close()

	program := DocumentSymbolsRecursive(tree.RootNode(), content)
	//	fmt.Println(program.Children)
	return program.Children
}

func DocumentSymbolsRecursiveNoEnvironment(node *tree_sitter.Node, content []byte) DocumentSymbol {
	name := node.GrammarName()
	var s DocumentSymbol
	if name == "definition" || name == "function_definition" {
		ident := node.Child(0)
		s.Name = ident.Utf8Text(content)
		//		istart := ident.StartPosition()
		//		iend := ident.EndPosition()
		start := node.StartPosition()
		end := node.EndPosition()
		if name == "function_definition" {
			s.Kind = Function
		} else if name == "definition" {
			s.Kind = Variable
		}
		s.SelectionRange = Range{
			Start: Position{Line: uint32(start.Row), Character: uint32(start.Column)},
			End:   Position{Line: uint32(end.Row), Character: uint32(end.Column)},
		}
		s.Range = Range{
			Start: Position{Line: uint32(start.Row), Character: uint32(start.Column)},
			End:   Position{Line: uint32(end.Row), Character: uint32(end.Column)},
		}
	}

	if name == "definition" || name == "function_definition" || name == "environment" || name == "program" {
		for i := uint(0); i < node.ChildCount(); i++ {
			n := node.Child(i)
			node := DocumentSymbolsRecursive(n, content)
			if node.Name != "" {
				s.Children = append(s.Children, node)
			}
		}
		return s
	} else {
		return DocumentSymbol{}
	}

}

func DocumentSymbolsRecursive(node *tree_sitter.Node, content []byte) DocumentSymbol {
	name := node.GrammarName()
	var s DocumentSymbol
	if name == "definition" || name == "function_definition" {
		ident := node.Child(0)
		s.Name = ident.Utf8Text(content)
		if name == "function_definition" {
			s.Kind = Function
		} else if name == "definition" {
			// Every definition is essentially a function in Faust than a variable
			s.Kind = Function
		}
		//		istart := ident.StartPosition()
		//		iend := ident.EndPosition()
		start := node.StartPosition()
		end := node.EndPosition()
		s.SelectionRange = Range{
			Start: Position{Line: uint32(start.Row), Character: uint32(start.Column)},
			End:   Position{Line: uint32(end.Row), Character: uint32(end.Column)},
		}
		s.Range = Range{
			Start: Position{Line: uint32(start.Row), Character: uint32(start.Column)},
			End:   Position{Line: uint32(end.Row), Character: uint32(end.Column)},
		}
	}

	if name == "definition" || name == "function_definition" || name == "program" {
		//		fmt.Printf("Got %s with %s\n",name,node.Utf8Text(content))
		for i := uint(0); i < node.ChildCount(); i++ {
			n := node.Child(i)
			node := DocumentSymbolsRecursive(n, content)
			if node.Name == "environment" {
				s.Children = append(s.Children, node.Children...)
			} else if node.Name != "" {
				s.Children = append(s.Children, node)
			}
		}
		//		fmt.Printf("children of %s is %v\n", node.GrammarName(), s.Children)
		return s
	} else if name == "with_environment" || name == "letrec_environment" {
		s.Name = "environment"
		//		fmt.Printf("Got %s with %s\n",name,node.Utf8Text(content))
		if node.ChildCount() >= 2 {
			node = node.Child(2)
		} else {
			return DocumentSymbol{}
		}
		//		fmt.Printf("Got %s with %s\n",node.GrammarName(),node.Utf8Text(content))
		for i := uint(0); i < node.ChildCount(); i++ {
			n := node.Child(i)
			node := DocumentSymbolsRecursive(n, content)
			if node.Name != "" {
				s.Children = append(s.Children, node)
			}
		}
		//		fmt.Printf("children of %s is %v\n", node.GrammarName(), s.Children)
		return s
	} else {
		return DocumentSymbol{}
	}

}

func GetQueryMatches(queryStr string, code []byte, tree *tree_sitter.Tree) TSQueryResult {
	tsParser.treesToClose = append(tsParser.treesToClose, tree)
	//	defer tree.Close()

	query, _ := tree_sitter.NewQuery(tsParser.language, queryStr)
	defer query.Close()

	cursor := tree_sitter.NewQueryCursor()
	defer cursor.Close()

	matches := cursor.Matches(query, tree.RootNode(), code)

	var result TSQueryResult
	result.results = make(map[string][]tree_sitter.Node)
	for match := matches.Next(); match != nil; match = matches.Next() {
		for _, capture := range match.Captures {
			//			fmt.Printf("Match %d, Capture %d (%s): %s\n", match.PatternIndex, capture.Index, query.CaptureNames()[capture.Index], capture.Node.Utf8Text(code))

			// Add to result
			captureName := query.CaptureNames()[capture.Index]
			captures, _ := result.results[captureName]
			node := capture.Node
			result.results[captureName] = append(captures, node)
		}
	}

	return result
}

func Close() {
	//	tsParser.parser.Close()
	for _, tree := range tsParser.treesToClose {
		tree.Close()
	}
}
