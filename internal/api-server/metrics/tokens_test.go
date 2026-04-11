package metrics

import "testing"

func TestRecordTokenGenerate(t *testing.T) {
	t.Parallel()
	RecordTokenGenerate(false, TokenResultCreated)
	RecordTokenGenerate(true, TokenResultCreated)
	RecordTokenGenerate(true, TokenResultInternalError)
}
