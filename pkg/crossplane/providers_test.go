package crossplane

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAddProviderPrefix(t *testing.T) {
	tt := []struct {
		desc string
		in   string
		exp  string
	}{
		{
			desc: "no prefix",
			in:   "myprovider",
			exp:  providerPrefix + "myprovider",
		},
		{
			desc: "don't double prefix",
			in:   providerPrefix + "myprovider",
			exp:  providerPrefix + "myprovider",
		},
	}

	for _, tc := range tt {
		t.Run(tc.desc, func(t *testing.T) {
			actual := AddProviderPrefix(tc.in)
			assert.Equal(t, tc.exp, actual)
		})
	}

}
