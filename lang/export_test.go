package lang

// TestingPack exposes the stopword and connector sets of a pack built by New.
func TestingPack(p Pack) (stopwords, connectors map[string]struct{}) {
	dp := p.(*dataPack)
	return dp.stopwords, dp.connectors
}
