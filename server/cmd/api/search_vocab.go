package main

import (
	"container/heap"
	"context"
	"database/sql"
	"fmt"
	"sort"
	"sync"
)

const (
	searchVocabMaxTerms      = 100_000
	searchVocabMaxVisited    = 512
	searchVocabMaxTokenRunes = 64
)

type searchVocabTerm struct {
	term  string
	runes []rune
	doc   int64
}

type searchVocabEdge struct {
	distance int
	child    *searchVocabNode
}

type searchVocabNode struct {
	value    searchVocabTerm
	children []searchVocabEdge
}

// searchVocabIndex partitions terms by rune length before applying BK-tree
// traversal. A true Levenshtein metric drives pruning; OSA distance performs
// the final typo check so adjacent transpositions retain their existing cost.
type searchVocabIndex struct {
	roots map[int]*searchVocabNode
}

type cachedSearchVocabIndex struct {
	generation int64
	index      *searchVocabIndex
}

type searchVocabCache struct {
	mu      sync.RWMutex
	buildMu sync.Mutex
	indexes map[string]cachedSearchVocabIndex
}

type searchVocabCandidate struct {
	term string
	dist int
	doc  int64
}

type searchVocabNodeHeap []*searchVocabNode

func (h searchVocabNodeHeap) Len() int { return len(h) }

func (h searchVocabNodeHeap) Less(i, j int) bool {
	if h[i].value.doc != h[j].value.doc {
		return h[i].value.doc > h[j].value.doc
	}
	return h[i].value.term < h[j].value.term
}

func (h searchVocabNodeHeap) Swap(i, j int) { h[i], h[j] = h[j], h[i] }

func (h *searchVocabNodeHeap) Push(value any) {
	*h = append(*h, value.(*searchVocabNode))
}

func (h *searchVocabNodeHeap) Pop() any {
	old := *h
	n := len(old)
	value := old[n-1]
	*h = old[:n-1]
	return value
}

func exactLevenshtein(a, b []rune) int {
	if len(a) > len(b) {
		a, b = b, a
	}

	previous := make([]int, len(a)+1)
	current := make([]int, len(a)+1)
	for i := range previous {
		previous[i] = i
	}

	for i, rb := range b {
		current[0] = i + 1
		for j, ra := range a {
			cost := 1
			if ra == rb {
				cost = 0
			}
			current[j+1] = min(current[j]+1, previous[j+1]+1, previous[j]+cost)
		}
		previous, current = current, previous
	}
	return previous[len(a)]
}

func newSearchVocabIndex(terms []searchVocabTerm) *searchVocabIndex {
	sort.Slice(terms, func(i, j int) bool {
		if terms[i].doc != terms[j].doc {
			return terms[i].doc > terms[j].doc
		}
		return terms[i].term < terms[j].term
	})

	index := &searchVocabIndex{roots: make(map[int]*searchVocabNode)}
	for _, term := range terms {
		index.insert(term)
	}
	return index
}

func (index *searchVocabIndex) insert(term searchVocabTerm) {
	root := index.roots[len(term.runes)]
	if root == nil {
		index.roots[len(term.runes)] = &searchVocabNode{value: term}
		return
	}

	node := root
	for {
		distance := exactLevenshtein(term.runes, node.value.runes)
		var child *searchVocabNode
		for _, edge := range node.children {
			if edge.distance == distance {
				child = edge.child
				break
			}
		}
		if child == nil {
			node.children = append(node.children, searchVocabEdge{
				distance: distance,
				child:    &searchVocabNode{value: term},
			})
			return
		}
		node = child
	}
}

func (index *searchVocabIndex) corrections(token string, maxDist, visitLimit int) ([]string, int) {
	queryRunes := []rune(token)
	metricRadius := maxDist * 2
	frontier := &searchVocabNodeHeap{}
	heap.Init(frontier)
	for length := len(queryRunes) - maxDist; length <= len(queryRunes)+maxDist; length++ {
		if root := index.roots[length]; root != nil {
			heap.Push(frontier, root)
		}
	}

	candidates := make([]searchVocabCandidate, 0, searchVocabMaxCorrections)
	visited := 0
	for frontier.Len() > 0 && visited < visitLimit {
		node := heap.Pop(frontier).(*searchVocabNode)
		visited++

		metricDistance := exactLevenshtein(queryRunes, node.value.runes)
		if node.value.term != token && metricDistance <= metricRadius {
			distance := damerauLevenshtein(token, node.value.term, maxDist)
			if distance <= maxDist {
				candidates = append(candidates, searchVocabCandidate{
					term: node.value.term,
					dist: distance,
					doc:  node.value.doc,
				})
			}
		}

		minEdge := metricDistance - metricRadius
		maxEdge := metricDistance + metricRadius
		for _, edge := range node.children {
			if edge.distance < minEdge || edge.distance > maxEdge {
				continue
			}
			if visited+frontier.Len() >= visitLimit {
				break
			}
			heap.Push(frontier, edge.child)
		}
	}

	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].dist != candidates[j].dist {
			return candidates[i].dist < candidates[j].dist
		}
		if candidates[i].doc != candidates[j].doc {
			return candidates[i].doc > candidates[j].doc
		}
		return candidates[i].term < candidates[j].term
	})
	if len(candidates) > searchVocabMaxCorrections {
		candidates = candidates[:searchVocabMaxCorrections]
	}

	corrections := make([]string, len(candidates))
	for i, candidate := range candidates {
		corrections[i] = candidate.term
	}
	return corrections, visited
}

func (cache *searchVocabCache) get(vocabTable string, generation int64) (*searchVocabIndex, bool) {
	cache.mu.RLock()
	defer cache.mu.RUnlock()

	cached, ok := cache.indexes[vocabTable]
	if !ok || cached.generation != generation {
		return nil, false
	}
	return cached.index, true
}

func (cache *searchVocabCache) set(vocabTable string, generation int64, index *searchVocabIndex) {
	cache.mu.Lock()
	defer cache.mu.Unlock()

	if cache.indexes == nil {
		cache.indexes = make(map[string]cachedSearchVocabIndex)
	}
	cache.indexes[vocabTable] = cachedSearchVocabIndex{
		generation: generation,
		index:      index,
	}
}

func searchVocabSelectSQL(vocabTable string) (string, error) {
	switch vocabTable {
	case "movies_fts_vocab", "albums_fts_vocab", "musicians_fts_vocab", "tracks_search_fts_vocab":
		return "SELECT term, doc FROM " + vocabTable + " LIMIT ?", nil
	default:
		return "", fmt.Errorf("unsupported search vocabulary table %q", vocabTable)
	}
}

func (app *Application) searchVocabIndex(ctx context.Context, vocabTable string) (*searchVocabIndex, error) {
	var generation int64
	err := app.DB.QueryRowContext(ctx, `
		SELECT generation
		FROM search_vocab_generations
		WHERE vocab_table = ?
	`, vocabTable).Scan(&generation)
	if err != nil {
		return nil, err
	}
	if index, ok := app.SearchVocab.get(vocabTable, generation); ok {
		return index, nil
	}

	app.SearchVocab.buildMu.Lock()
	defer app.SearchVocab.buildMu.Unlock()

	tx, err := app.DB.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	err = tx.QueryRowContext(ctx, `
		SELECT generation
		FROM search_vocab_generations
		WHERE vocab_table = ?
	`, vocabTable).Scan(&generation)
	if err != nil {
		return nil, err
	}
	if index, ok := app.SearchVocab.get(vocabTable, generation); ok {
		return index, nil
	}

	query, err := searchVocabSelectSQL(vocabTable)
	if err != nil {
		return nil, err
	}
	rows, err := tx.QueryContext(ctx, query, searchVocabMaxTerms+1)
	if err != nil {
		return nil, err
	}

	terms := make([]searchVocabTerm, 0)
	truncated := false
	rowsRead := 0
	for rows.Next() {
		rowsRead++
		if rowsRead > searchVocabMaxTerms {
			truncated = true
			break
		}

		var term string
		var doc int64
		err = rows.Scan(&term, &doc)
		if err != nil {
			rows.Close()
			return nil, err
		}

		runes := []rune(term)
		if len(runes) < 3 || len(runes) > searchVocabMaxTokenRunes {
			continue
		}
		terms = append(terms, searchVocabTerm{term: term, runes: runes, doc: doc})
	}
	rowsErr := rows.Err()
	closeErr := rows.Close()
	if rowsErr != nil {
		return nil, rowsErr
	}
	if closeErr != nil {
		return nil, closeErr
	}

	index := newSearchVocabIndex(terms)
	err = tx.Commit()
	if err != nil {
		return nil, err
	}
	app.SearchVocab.set(vocabTable, generation, index)

	if truncated && app.Logger != nil {
		app.Logger.Warn("search vocabulary cache reached term limit", "vocab_table", vocabTable, "limit", searchVocabMaxTerms)
	}
	return index, nil
}
