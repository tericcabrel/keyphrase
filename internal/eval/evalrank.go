package eval

type Options struct {
	DropLoneNouns     bool
	LoneNounMinRarity float64
	DropOneOffPhrases bool
	OneOffMinOthers   int
}

type Ranking struct {
	Rarity          float64
	PhraseRarityMax bool
	MinSingleRarity float64
}

type Keyword struct {
	Text      string
	StartByte int
	EndByte   int
	Score     float64
}

var (
	Extract        func(text, language string, limit int, options Options, ranking Ranking) ([]Keyword, error)
	DefaultRanking Ranking
)
