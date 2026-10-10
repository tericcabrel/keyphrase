// Package all registers every built-in language.
//
// Import it for its side effect when a program needs English, French,
// German, Spanish, and Italian together. A library user imports only the
// language packages they want.
package all

import (
	_ "github.com/tericcabrel/keyphrase/lang/de"
	_ "github.com/tericcabrel/keyphrase/lang/en"
	_ "github.com/tericcabrel/keyphrase/lang/es"
	_ "github.com/tericcabrel/keyphrase/lang/fr"
	_ "github.com/tericcabrel/keyphrase/lang/it"
)
