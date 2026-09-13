package graph

import (
	"golang.org/x/text/unicode/norm"
	"math"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"
)

func Normalize(s string) string {
	return strings.Join(strings.Fields(strings.NewReplacer("ё", "е", "—", "-", "–", "-", "‑", "-").Replace(strings.ToLower(norm.NFKC.String(s)))), " ")
}

type ExtractOptions struct {
	Language, CurrentDomain, CurrentQuestion string
	MaxConcepts                              int
}
type segment struct {
	text   string
	weight float64
	offset int
}

func segments(s string) []segment {
	out := []segment{}
	start := 0
	weight := 1.
	delimiter := ""
	for i := 0; i < len(s); {
		marker := ""
		if strings.HasPrefix(s[i:], "```") {
			marker = "```"
		} else if s[i] == '`' {
			marker = "`"
		}
		if marker != "" && (delimiter == "" || delimiter == marker) {
			if i > start {
				out = append(out, segment{s[start:i], weight, start})
			}
			if delimiter == "" {
				delimiter = marker
				if marker == "```" {
					weight = .55
				} else {
					weight = .75
				}
			} else {
				delimiter = ""
				weight = 1
			}
			i += len(marker)
			start = i
		} else {
			i++
		}
	}
	if start < len(s) {
		out = append(out, segment{s[start:], weight, start})
	}
	return out
}
func word(r rune) bool { return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' }
func boundary(s string, start, end int) bool {
	if start > 0 {
		r, _ := utf8.DecodeLastRuneInString(s[:start])
		if word(r) {
			return false
		}
	}
	if end < len(s) {
		r, _ := utf8.DecodeRuneInString(s[end:])
		if word(r) {
			return false
		}
	}
	return true
}
func containsTerm(s, term string) bool {
	term = Normalize(term)
	if term == "" {
		return false
	}
	at := 0
	for at < len(s) {
		i := strings.Index(s[at:], term)
		if i < 0 {
			return false
		}
		i += at
		if boundary(s, i, i+len(term)) {
			return true
		}
		at = i + len(term)
	}
	return false
}
func allowed(a Alias, text string, o ExtractOptions) bool {
	lang := strings.ToLower(strings.Split(o.Language, "-")[0])
	aliasLang := strings.ToLower(strings.Split(a.Language, "-")[0])
	if lang != "" && aliasLang != "" && aliasLang != "any" && aliasLang != lang {
		return false
	}
	// Domain and co-occurrence are alternative disambiguating evidence.
	if a.Constraints.RequiresDomain != "" || len(a.Constraints.RequiresAny) > 0 {
		if a.Constraints.RequiresDomain != "" && Normalize(o.CurrentDomain) == Normalize(a.Constraints.RequiresDomain) {
			return true
		}
		for _, v := range a.Constraints.RequiresAny {
			if containsTerm(text, v) {
				return true
			}
		}
		return false
	}
	return true
}
func LowInformation(text string) bool {
	t := strings.Trim(Normalize(text), " .!?…,:;")
	if len(strings.Fields(t)) > 9 {
		return false
	}
	for _, p := range []string{"не знаю", "не помню", "не уверен", "без понятия", "затрудняюсь", "don't know", "do not know", "not sure", "can't remember", "cannot remember"} {
		if t == p || t == "i "+p || t == "я "+p || t == "я этого "+p {
			return true
		}
	}
	return false
}

// A rune trie supplies longest candidates without a scan of every alias at
// every answer position. Overlapping spans are resolved before ranking.
type trie struct {
	next    map[rune]*trie
	aliases []Alias
}

func Extract(text string, o ExtractOptions, c Catalog) []Match {
	normalized := Normalize(text)
	root := &trie{next: map[rune]*trie{}}
	for _, a := range c.Aliases {
		a.Text = Normalize(a.Text)
		if a.Text == "" || !allowed(a, normalized, o) {
			continue
		}
		n := root
		for _, r := range a.Text {
			if n.next[r] == nil {
				n.next[r] = &trie{next: map[rune]*trie{}}
			}
			n = n.next[r]
		}
		n.aliases = append(n.aliases, a)
	}
	found := []Match{}
	for _, seg := range segments(normalized) {
		runes := []rune(seg.text)
		offsets := make([]int, len(runes)+1)
		b := 0
		for i, r := range runes {
			offsets[i] = b
			b += utf8.RuneLen(r)
		}
		offsets[len(runes)] = b
		for i := range runes {
			n := root
			for j := i; j < len(runes); j++ {
				n = n.next[runes[j]]
				if n == nil {
					break
				}
				for _, a := range n.aliases {
					start, end := offsets[i], offsets[j+1]
					if a.WholeWord && !boundary(seg.text, start, end) {
						continue
					}
					src := "exact_word"
					if strings.Contains(a.Text, " ") {
						src = "exact_phrase"
					}
					if strings.ContainsAny(a.Text, "ьъэйюй") && a.Slug != "" {
						src = "transliteration"
					}
					if seg.weight < 1 {
						src = "code"
					}
					w := a.Weight
					if w == 0 {
						w = 1
					}
					strength := w * seg.weight
					// Discount only a copied question span, not a term legitimately explained.
					prompt := Normalize(o.CurrentQuestion)
					if prompt != "" && strings.Contains(seg.text, prompt) {
						p := strings.Index(seg.text, prompt)
						if start >= p && end <= p+len(prompt) {
							strength *= .05
						}
					}
					found = append(found, Match{a.ConceptID, a.Slug, a.Text, seg.offset + start, seg.offset + end, strength, src})
				}
			}
		}
	}
	sort.SliceStable(found, func(i, j int) bool {
		li, lj := found[i].End-found[i].Start, found[j].End-found[j].Start
		if li != lj {
			return li > lj
		}
		if found[i].Strength != found[j].Strength {
			return found[i].Strength > found[j].Strength
		}
		return found[i].Slug < found[j].Slug
	})
	exact := []Match{}
	for _, m := range found {
		overlap := false
		for _, keep := range exact {
			if m.Start < keep.End && m.End > keep.Start {
				overlap = true
				break
			}
		}
		if !overlap {
			exact = append(exact, m)
		}
	}
	// Conservative fuzzy fallback is restricted to unclaimed word tokens.
	for _, seg := range segments(normalized) {
		at := 0
		for _, token := range strings.FieldsFunc(seg.text, func(r rune) bool { return !word(r) }) {
			rel := strings.Index(seg.text[at:], token)
			if rel < 0 {
				continue
			}
			start := seg.offset + at + rel
			end := start + len(token)
			at += rel + len(token)
			if utf8.RuneCountInString(token) < 5 {
				continue
			}
			occupied := false
			for _, m := range exact {
				if start < m.End && end > m.Start {
					occupied = true
					break
				}
			}
			if occupied {
				continue
			}
			var best *Match
			for _, a := range c.Aliases {
				alias := Normalize(a.Text)
				if utf8.RuneCountInString(alias) < 6 || strings.Contains(alias, " ") || !allowed(a, normalized, o) || !distanceOne([]rune(token), []rune(alias)) {
					continue
				}
				w := a.Weight
				if w == 0 {
					w = 1
				}
				m := Match{a.ConceptID, a.Slug, token, start, end, w * .45 * seg.weight, "fuzzy"}
				if best == nil || m.Strength > best.Strength || (m.Strength == best.Strength && m.Slug < best.Slug) {
					best = &m
				}
			}
			if best != nil {
				exact = append(exact, *best)
			}
		}
	}
	byConcept := map[string]Match{}
	for _, m := range exact {
		n := math.Max(1, float64(c.QuestionCount))
		df := math.Min(n, float64(c.DocumentFrequency[m.Slug]))
		idf := math.Log((n+1)/(df+1)) + 1
		m.Strength = math.Min(1, m.Strength*(.55+.45*idf/(math.Log(n+1)+1)))
		if old, ok := byConcept[m.Slug]; !ok || m.Strength > old.Strength {
			byConcept[m.Slug] = m
		}
	}
	out := make([]Match, 0, len(byConcept))
	for _, m := range byConcept {
		out = append(out, m)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Strength != out[j].Strength {
			return out[i].Strength > out[j].Strength
		}
		return out[i].Slug < out[j].Slug
	})
	limit := o.MaxConcepts
	if limit <= 0 {
		limit = 6
	}
	if len(out) > limit {
		out = out[:limit]
	}
	return out
}
func distanceOne(a, b []rune) bool {
	if len(a)-len(b) > 1 || len(b)-len(a) > 1 {
		return false
	}
	i, j, edits := 0, 0, 0
	for i < len(a) && j < len(b) {
		if a[i] == b[j] {
			i++
			j++
			continue
		}
		edits++
		if edits > 1 {
			return false
		}
		if len(a) > len(b) {
			i++
		} else if len(b) > len(a) {
			j++
		} else {
			i++
			j++
		}
	}
	if i < len(a) || j < len(b) {
		edits++
	}
	return edits == 1
}
