package i18n

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

var verb = regexp.MustCompile(`%[a-z]`)

func TestTranslate(t *testing.T) {
	defer Set(English)
	Set(English)
	if T("Unlock") != "Unlock" || Current() != English {
		t.Fatal("English is not the identity")
	}
	Set(Turkish)
	if T("Unlock") != "Kilidi aç" || Current() != Turkish {
		t.Fatal("Turkish not applied")
	}
	if T("not in the catalogue") != "not in the catalogue" {
		t.Fatal("missing string did not fall back to English")
	}
	if got := Tf("Use at least %d characters.", 12); got != "En az 12 karakter kullanın." {
		t.Fatalf("Tf = %q", got)
	}
	Set("xx")
	if Current() != English {
		t.Fatal("unknown language did not fall back to English")
	}
}

func TestFromLocale(t *testing.T) {
	for locale, want := range map[string]Lang{
		"tr": Turkish, "tr-TR": Turkish, "TR_tr": Turkish,
		"en-US": English, "de": English, "": English,
	} {
		if got := FromLocale(locale); got != want {
			t.Errorf("FromLocale(%q) = %q", locale, got)
		}
	}
}

// A translation must keep the format verbs and line breaks of its source.
func TestCatalogueShapeMatches(t *testing.T) {
	for en, turkish := range tr {
		if turkish == "" {
			t.Errorf("empty translation for %q", en)
		}
		a, b := verb.FindAllString(en, -1), verb.FindAllString(turkish, -1)
		if strings.Join(a, "") != strings.Join(b, "") {
			t.Errorf("format verbs differ: %q -> %q", en, turkish)
		}
		if strings.Count(en, "\n") != strings.Count(turkish, "\n") {
			t.Errorf("line count differs: %q -> %q", en, turkish)
		}
	}
}

// The catalogue and the interface must agree: every string the interface
// translates has a Turkish entry, and no entry is left over.
func TestCatalogueCoversInterface(t *testing.T) {
	files, err := filepath.Glob(filepath.Join("..", "ui", "*.go"))
	if err != nil || len(files) == 0 {
		t.Fatalf("no ui sources found: %v", err)
	}
	used := map[string]bool{}
	for _, file := range files {
		if strings.HasSuffix(file, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(token.NewFileSet(), file, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		ast.Inspect(f, func(n ast.Node) bool {
			switch n := n.(type) {
			case *ast.CallExpr: // i18n.T("…") and i18n.Tf("…", …)
				sel, ok := n.Fun.(*ast.SelectorExpr)
				if !ok || len(n.Args) == 0 {
					return true
				}
				pkg, ok := sel.X.(*ast.Ident)
				if !ok || pkg.Name != "i18n" || (sel.Sel.Name != "T" && sel.Sel.Name != "Tf") {
					return true
				}
				if lit, ok := n.Args[0].(*ast.BasicLit); ok {
					s, _ := strconv.Unquote(lit.Value)
					used[s] = true
				}
			case *ast.CompositeLit: // {"After 5 minutes", 5} in the settings
				if len(n.Elts) != 2 {
					return true
				}
				label, ok1 := n.Elts[0].(*ast.BasicLit)
				value, ok2 := n.Elts[1].(*ast.BasicLit)
				if ok1 && ok2 && label.Kind == token.STRING && value.Kind == token.INT {
					s, _ := strconv.Unquote(label.Value)
					used[s] = true
				}
			}
			return true
		})
	}
	if len(used) < 40 {
		t.Fatalf("only %d interface strings found; the scan is broken", len(used))
	}
	for s := range used {
		if _, ok := tr[s]; !ok {
			t.Errorf("no Turkish translation for %q", s)
		}
	}
	for s := range tr {
		if !used[s] {
			t.Errorf("translation for %q is not used by the interface", s)
		}
	}
}
