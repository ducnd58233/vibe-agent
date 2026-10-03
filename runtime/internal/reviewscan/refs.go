package reviewscan

import "sort"

// index is every name the workspace uses, by whether a test file uses it.
type index struct {
	analyses []*analysis
	refs     map[string]int
	testRefs map[string]int
	defs     map[string]int
	mentions map[string]bool
}

func buildIndex(analyses []*analysis) *index {
	x := &index{
		analyses: analyses, refs: map[string]int{}, testRefs: map[string]int{},
		defs: map[string]int{}, mentions: map[string]bool{},
	}
	for _, a := range analyses {
		if a == nil {
			continue
		}
		bucket := x.refs
		if a.file.Test {
			bucket = x.testRefs
		}
		for name, n := range a.idents {
			bucket[name] += n
		}
		for name, n := range a.importRefs {
			bucket[name] += n
		}
		for name := range a.mentions {
			x.mentions[name] = true
		}
		for _, d := range a.defs {
			x.defs[d.name]++
		}
	}
	return x
}

// crossFileFindings sets each definition's reference count and reports the
// ones nothing uses. A name defined more than once is skipped: without types
// a scan cannot tell which definition a use reaches, and repeated names are
// mostly overrides and interface implementations.
func (x *index) crossFileFindings(diff *changes) {
	for _, a := range x.analyses {
		if a == nil || !a.code {
			continue
		}
		for _, d := range a.defs {
			prod, test := x.refs[d.name], x.testRefs[d.name]
			if a.file.Test {
				test -= d.selfRefs
			} else {
				prod -= d.selfRefs
			}
			total := max(prod+test, 0)
			if d.block >= 0 {
				a.file.Blocks[d.block].Refs = &total
			}
			if d.exempt || x.defs[d.name] > 1 || len(d.name) < 2 {
				continue
			}
			built := ""
			if total == 0 {
				built = x.builtPrefix(d.name)
			}
			switch {
			case total == 0 && x.mentions[d.name]:
				// Named in a string or a config file: possibly looked up by name.
			case built != "":
				a.add(d.line, ruleUnreferenced, SeverityInfo,
					"nothing references `%s` by name, but the string %q starts it; it may be called through a name built at runtime", d.name, built)
			case total == 0 && diff != nil && diff.removed[d.name]:
				a.add(d.line, ruleOrphaned, SeverityMedium,
					"`%s` lost its last reference in this change; delete it or restore the caller", d.name)
			case total == 0 && d.private:
				a.add(d.line, ruleUnreferenced, SeverityMedium,
					"`%s` is private and nothing in the repository references it; it is dead code", d.name)
			case total == 0:
				a.add(d.line, ruleUnreferenced, SeverityInfo,
					"nothing in this repository references `%s`; it is dead unless another package, a framework, or reflection calls it", d.name)
			case prod == 0 && !a.file.Test && d.private:
				a.add(d.line, ruleTestOnly, SeverityLow,
					"`%s` is referenced only from tests; production code never calls it", d.name)
			}
		}
	}
}

// builtPrefix returns a string-literal word that is a leading piece of name,
// cut where a name is usually spliced (after an underscore, before a capital):
// `getattr(self, "_handle_" + kind)` reaches `_handle_message`.
func (x *index) builtPrefix(name string) string {
	for i := 1; i < len(name); i++ {
		underscore := name[i-1] == '_' && i >= 3
		camel := i >= 4 && name[i] >= 'A' && name[i] <= 'Z' && name[i-1] >= 'a' && name[i-1] <= 'z'
		if (underscore || camel) && x.mentions[name[:i]] {
			return name[:i]
		}
	}
	return ""
}

// ReferencedIn lists the files that use name, at most limit of them.
func (r Report) ReferencedIn(name string, limit int) []string {
	if r.index == nil {
		return nil
	}
	var out []string
	for _, a := range r.index.analyses {
		if a != nil && (a.idents[name] > 0 || a.importRefs[name] > 0) {
			out = append(out, a.file.Path)
		}
	}
	sort.Strings(out)
	if len(out) > limit {
		out = out[:limit]
	}
	return out
}
