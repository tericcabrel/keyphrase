package extractor

import "github.com/tericcabrel/keyphrase/internal/eval"

// The eval command is another package, and this call is not part of the public API.
func init() {
	eval.DefaultRanking = eval.Ranking{
		Rarity:          defaultRanking.Rarity,
		PhraseRarityMax: defaultRanking.PhraseRarityMax,
		MinSingleRarity: defaultRanking.MinSingleRarity,
	}
	eval.Extract = extractForEval
}

func extractForEval(text, language string, limit int, options eval.Options, rank eval.Ranking) ([]eval.Keyword, error) {
	if err := validateExtract(language, limit); err != nil {
		return nil, err
	}
	keywords, _, err := extract(text, language, limit, ExtractOptions{
		DropLoneNouns:     options.DropLoneNouns,
		LoneNounMinRarity: options.LoneNounMinRarity,
		DropOneOffPhrases: options.DropOneOffPhrases,
		OneOffMinOthers:   options.OneOffMinOthers,
	}, ranking{
		Rarity:          rank.Rarity,
		PhraseRarityMax: rank.PhraseRarityMax,
		MinSingleRarity: rank.MinSingleRarity,
	})
	if err != nil {
		return nil, err
	}
	out := make([]eval.Keyword, len(keywords))
	for i, keyword := range keywords {
		out[i] = eval.Keyword{
			Text:      keyword.Text,
			StartByte: keyword.StartByte,
			EndByte:   keyword.EndByte,
			Score:     keyword.Score,
		}
	}
	return out, nil
}
