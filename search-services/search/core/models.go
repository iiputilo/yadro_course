package core

type Comic struct {
	ID  int
	URL string
}

type Index map[string][]int

type SearchParams struct {
	Phrase string
	Limit  int
}

type SearchResult struct {
	Comics []Comic
	Total  int
}
