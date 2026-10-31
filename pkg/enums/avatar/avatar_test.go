package avatar

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAll(t *testing.T) {
	all := All()
	assert.Len(t, all, 15)
	assert.Equal(t, "bubu-01", all[0])
	assert.Equal(t, "bubu-15", all[14])
	for _, id := range all {
		assert.True(t, IsValid(id), id)
	}
}

func TestIsValid(t *testing.T) {
	for _, s := range []string{"", "bubu-00", "bubu-16", "bubu-1", "bubu-001", "BUBU-01", "Bubu-01", " bubu-01", "bubu-01 ", "bubu-01\n",
		"bubu_01", "bubu-0١", "http://evil/x.png", "evil", strings.Repeat("a", 500)} {
		assert.False(t, IsValid(s), "%q", s)
	}
}
