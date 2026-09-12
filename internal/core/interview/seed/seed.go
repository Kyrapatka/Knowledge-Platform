// Package seed embeds the answer-free Dynamic Interview Graph source bank.
package seed

import _ "embed"

// Version identifies the source corpus, independently of user edits and profiles.
const Version = "2026-09-12.1"

// QuestionCount is the number of question profiles in the supplied specification.
const QuestionCount = 368

//go:embed bank.json
var bank []byte

// Raw returns an independent copy so callers cannot mutate the embedded bank.
func Raw() []byte {
	return append([]byte(nil), bank...)
}
