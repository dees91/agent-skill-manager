package advisor

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/dees91/agent-skill-manager/internal/model"
)

const (
	// CapabilityRankedSearch identifies the additive ranked-search contract.
	CapabilityRankedSearch = "ranked_search_v1"
	// DefaultSearchLimit bounds the first-party advisor's normal candidate view.
	DefaultSearchLimit = 20
	// MaxSearchLimit is the largest public ranked-search result set.
	MaxSearchLimit = 50
	// MaxSearchQueryRunes bounds untrusted query work without constraining metadata phrases.
	MaxSearchQueryRunes = 256
	// MaxSearchQueryTokens bounds BM25F and fuzzy comparisons per request.
	MaxSearchQueryTokens = 32

	bm25K1               = 1.2
	bm25B                = 0.75
	fuzzyWeight          = 0.35
	pairPhraseWeight     = 0.35
	fullPhraseWeight     = 0.50
	maxFieldRunes        = 8192
	maxFieldTokens       = 256
	maxIndexedTokenRunes = 64
)

const (
	searchFieldName = iota
	searchFieldDescription
	searchFieldGroup
	searchFieldSource
	searchFieldCount
)

var searchFieldWeights = [searchFieldCount]float64{4.0, 1.0, 0.75, 0.25}

var searchStopWords = map[string]struct{}{
	"a": {}, "an": {}, "and": {}, "are": {}, "as": {}, "at": {},
	"be": {}, "by": {}, "for": {}, "from": {}, "in": {}, "into": {},
	"is": {}, "it": {}, "of": {}, "on": {}, "or": {}, "that": {},
	"the": {}, "to": {}, "use": {}, "using": {}, "when": {}, "with": {},
}

// SearchOptions controls one read-only ranked metadata search.
type SearchOptions struct {
	Tool  model.Tool
	Query string
	Limit int
}

type searchDocument struct {
	row        model.SkillRow
	fields     [searchFieldCount][]string
	vocabulary []string
}

type searchCorpus struct {
	documents          []searchDocument
	averageFieldLength [searchFieldCount]float64
	documentFrequency  map[string]int
}

type scoredRow struct {
	row   model.SkillRow
	score float64
}

// Search ranks current tool-specific ON/OFF skill rows without exposing scores.
func Search(rows []model.SkillRow, options SearchOptions) ([]model.SkillRow, error) {
	queryTokens, err := validateSearchOptions(options)
	if err != nil {
		return nil, err
	}

	corpus := buildSearchCorpus(rows, options.Tool)
	if len(corpus.documents) == 0 {
		return []model.SkillRow{}, nil
	}
	uniqueQueryTokens := uniqueStrings(queryTokens)
	scored := make([]scoredRow, 0, len(corpus.documents))
	for index := range corpus.documents {
		document := &corpus.documents[index]
		score := corpus.scoreDocument(document, uniqueQueryTokens)
		score += corpus.phraseScore(document, queryTokens)
		if score > 0 {
			scored = append(scored, scoredRow{row: document.row, score: score})
		}
	}
	sort.SliceStable(scored, func(i, j int) bool {
		if scored[i].score != scored[j].score {
			return scored[i].score > scored[j].score
		}
		return scored[i].row.Name < scored[j].row.Name
	})
	if len(scored) > options.Limit {
		scored = scored[:options.Limit]
	}
	results := make([]model.SkillRow, 0, len(scored))
	for _, current := range scored {
		results = append(results, current.row)
	}
	return results, nil
}

// ValidateSearchOptions validates the public boundary without scanning skills.
func ValidateSearchOptions(options SearchOptions) error {
	_, err := validateSearchOptions(options)
	return err
}

func validateSearchOptions(options SearchOptions) ([]string, error) {
	if _, ok := model.ParseTool(options.Tool.String()); !ok {
		return nil, fmt.Errorf("unsupported tool %q", options.Tool)
	}
	query := strings.TrimSpace(options.Query)
	if query == "" {
		return nil, fmt.Errorf("advisor search requires a non-empty query")
	}
	if utf8.RuneCountInString(query) > MaxSearchQueryRunes {
		return nil, fmt.Errorf("advisor search query must contain at most %d characters", MaxSearchQueryRunes)
	}
	queryTokens := tokenizeSearchText(query, MaxSearchQueryTokens+1, MaxSearchQueryRunes)
	if len(queryTokens) == 0 {
		return nil, fmt.Errorf("advisor search query must contain at least one searchable token")
	}
	if len(queryTokens) > MaxSearchQueryTokens {
		return nil, fmt.Errorf("advisor search query must contain at most %d searchable tokens", MaxSearchQueryTokens)
	}
	if options.Limit < 1 || options.Limit > MaxSearchLimit {
		return nil, fmt.Errorf("advisor search limit must be between 1 and %d", MaxSearchLimit)
	}
	return queryTokens, nil
}

func buildSearchCorpus(rows []model.SkillRow, tool model.Tool) searchCorpus {
	corpus := searchCorpus{documentFrequency: map[string]int{}}
	fieldTotals := [searchFieldCount]int{}
	for _, row := range rows {
		cell := searchCell(row, tool)
		if cell == nil || cell.ReadOnly || (cell.State != model.SkillStateOn && cell.State != model.SkillStateOff) {
			continue
		}
		document := searchDocument{row: row}
		document.fields[searchFieldName] = tokenizeSearchText(row.Name, maxFieldTokens, maxFieldRunes)
		document.fields[searchFieldDescription] = tokenizeSearchText(row.Description, maxFieldTokens, maxFieldRunes)
		document.fields[searchFieldGroup] = tokenizeSearchText(row.Group.String(), maxFieldTokens, maxFieldRunes)
		document.fields[searchFieldSource] = tokenizeSearchText(row.Source.String(), maxFieldTokens, maxFieldRunes)

		seen := map[string]struct{}{}
		for field := 0; field < searchFieldCount; field++ {
			fieldTotals[field] += len(document.fields[field])
			for _, token := range document.fields[field] {
				if _, ok := seen[token]; ok {
					continue
				}
				seen[token] = struct{}{}
				document.vocabulary = append(document.vocabulary, token)
			}
		}
		for token := range seen {
			corpus.documentFrequency[token]++
		}
		corpus.documents = append(corpus.documents, document)
	}
	for field := 0; field < searchFieldCount; field++ {
		if len(corpus.documents) == 0 || fieldTotals[field] == 0 {
			corpus.averageFieldLength[field] = 1
			continue
		}
		corpus.averageFieldLength[field] = float64(fieldTotals[field]) / float64(len(corpus.documents))
	}
	return corpus
}

func searchCell(row model.SkillRow, tool model.Tool) *model.ToolSkill {
	switch tool {
	case model.ToolClaude:
		return row.Claude
	case model.ToolCodex:
		return row.Codex
	default:
		return nil
	}
}

func (corpus searchCorpus) scoreDocument(document *searchDocument, queryTokens []string) float64 {
	score := 0.0
	for _, token := range queryTokens {
		exact := corpus.termScore(document, token)
		if exact > 0 {
			score += exact
			continue
		}
		score += corpus.fuzzyScore(document, token)
	}
	return score
}

func (corpus searchCorpus) termScore(document *searchDocument, token string) float64 {
	weightedFrequency := 0.0
	for field := 0; field < searchFieldCount; field++ {
		frequency := tokenFrequency(document.fields[field], token)
		if frequency == 0 {
			continue
		}
		lengthNormalization := 1 - bm25B + bm25B*(float64(len(document.fields[field]))/corpus.averageFieldLength[field])
		weightedFrequency += searchFieldWeights[field] * float64(frequency) / lengthNormalization
	}
	if weightedFrequency == 0 {
		return 0
	}
	idf := corpus.inverseDocumentFrequency(token)
	return idf * ((weightedFrequency * (bm25K1 + 1)) / (weightedFrequency + bm25K1))
}

func (corpus searchCorpus) fuzzyScore(document *searchDocument, queryToken string) float64 {
	allowedDistance := fuzzyDistanceLimit(utf8.RuneCountInString(queryToken))
	if allowedDistance == 0 {
		return 0
	}
	queryLength := utf8.RuneCountInString(queryToken)
	best := 0.0
	for _, candidate := range document.vocabulary {
		candidateLength := utf8.RuneCountInString(candidate)
		if absoluteInt(queryLength-candidateLength) > allowedDistance {
			continue
		}
		distance := damerauLevenshtein(queryToken, candidate)
		if distance == 0 || distance > allowedDistance {
			continue
		}
		maximumLength := maxInt(queryLength, candidateLength)
		similarity := 1 - float64(distance)/float64(maximumLength)
		candidateScore := corpus.termScore(document, candidate) * similarity * fuzzyWeight
		if candidateScore > best {
			best = candidateScore
		}
	}
	return best
}

func (corpus searchCorpus) phraseScore(document *searchDocument, queryTokens []string) float64 {
	if len(queryTokens) < 2 {
		return 0
	}
	score := 0.0
	for field := 0; field < searchFieldCount; field++ {
		for index := 0; index < len(queryTokens)-1; index++ {
			pair := queryTokens[index : index+2]
			if containsTokenSequence(document.fields[field], pair) {
				score += pairPhraseWeight * searchFieldWeights[field] * corpus.sequenceIDF(pair)
			}
		}
		if containsTokenSequence(document.fields[field], queryTokens) {
			score += fullPhraseWeight * searchFieldWeights[field] * corpus.sequenceIDF(queryTokens)
		}
	}
	return score
}

func (corpus searchCorpus) inverseDocumentFrequency(token string) float64 {
	documents := float64(len(corpus.documents))
	frequency := float64(corpus.documentFrequency[token])
	return math.Log(1 + (documents-frequency+0.5)/(frequency+0.5))
}

func (corpus searchCorpus) sequenceIDF(tokens []string) float64 {
	total := 0.0
	for _, token := range tokens {
		total += corpus.inverseDocumentFrequency(token)
	}
	return total
}

func tokenizeSearchText(value string, tokenLimit, runeLimit int) []string {
	tokens := make([]string, 0)
	current := make([]rune, 0, 32)
	processedRunes := 0
	flush := func() bool {
		if len(current) == 0 {
			return false
		}
		token := normalizeSearchToken(string(current))
		current = current[:0]
		if token == "" || utf8.RuneCountInString(token) > maxIndexedTokenRunes {
			return false
		}
		if _, stop := searchStopWords[token]; stop {
			return false
		}
		tokens = append(tokens, token)
		return len(tokens) >= tokenLimit
	}
	for _, currentRune := range value {
		processedRunes++
		if processedRunes > runeLimit {
			break
		}
		if unicode.IsLetter(currentRune) || unicode.IsDigit(currentRune) {
			current = append(current, unicode.ToLower(currentRune))
			continue
		}
		if flush() {
			return tokens
		}
	}
	flush()
	return tokens
}

func normalizeSearchToken(token string) string {
	runes := []rune(token)
	length := len(runes)
	if length >= 5 && strings.HasSuffix(token, "ies") {
		return string(runes[:length-3]) + "y"
	}
	if length >= 4 && strings.HasSuffix(token, "s") &&
		!strings.HasSuffix(token, "ss") && !strings.HasSuffix(token, "us") && !strings.HasSuffix(token, "is") {
		return string(runes[:length-1])
	}
	return token
}

func tokenFrequency(tokens []string, target string) int {
	frequency := 0
	for _, token := range tokens {
		if token == target {
			frequency++
		}
	}
	return frequency
}

func uniqueStrings(values []string) []string {
	seen := map[string]struct{}{}
	unique := make([]string, 0, len(values))
	for _, value := range values {
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		unique = append(unique, value)
	}
	return unique
}

func containsTokenSequence(tokens, sequence []string) bool {
	if len(sequence) == 0 || len(sequence) > len(tokens) {
		return false
	}
	for start := 0; start <= len(tokens)-len(sequence); start++ {
		matches := true
		for offset := range sequence {
			if tokens[start+offset] != sequence[offset] {
				matches = false
				break
			}
		}
		if matches {
			return true
		}
	}
	return false
}

func fuzzyDistanceLimit(length int) int {
	switch {
	case length <= 3:
		return 0
	case length <= 7:
		return 1
	default:
		return 2
	}
}

// damerauLevenshtein returns the optimal-string-alignment distance.
func damerauLevenshtein(left, right string) int {
	leftRunes := []rune(left)
	rightRunes := []rune(right)
	previousPrevious := make([]int, len(rightRunes)+1)
	previous := make([]int, len(rightRunes)+1)
	current := make([]int, len(rightRunes)+1)
	for index := range previous {
		previous[index] = index
	}
	for leftIndex := 1; leftIndex <= len(leftRunes); leftIndex++ {
		current[0] = leftIndex
		for rightIndex := 1; rightIndex <= len(rightRunes); rightIndex++ {
			cost := 0
			if leftRunes[leftIndex-1] != rightRunes[rightIndex-1] {
				cost = 1
			}
			current[rightIndex] = minInt(
				previous[rightIndex]+1,
				current[rightIndex-1]+1,
				previous[rightIndex-1]+cost,
			)
			if leftIndex > 1 && rightIndex > 1 &&
				leftRunes[leftIndex-1] == rightRunes[rightIndex-2] &&
				leftRunes[leftIndex-2] == rightRunes[rightIndex-1] {
				current[rightIndex] = minInt(current[rightIndex], previousPrevious[rightIndex-2]+1)
			}
		}
		previousPrevious, previous, current = previous, current, previousPrevious
	}
	return previous[len(rightRunes)]
}

func minInt(values ...int) int {
	minimum := values[0]
	for _, value := range values[1:] {
		if value < minimum {
			minimum = value
		}
	}
	return minimum
}

func maxInt(left, right int) int {
	if left > right {
		return left
	}
	return right
}

func absoluteInt(value int) int {
	if value < 0 {
		return -value
	}
	return value
}
