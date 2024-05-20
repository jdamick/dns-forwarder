package utils

import (
	"testing"

	"github.com/miekg/dns"
	"github.com/stretchr/testify/assert"
)

func TestMinMax(t *testing.T) {
	assert := assert.New(t)
	assert.Equal(1, Min(1, 2))
	assert.Equal(1, Min(2, 1))

	assert.Equal(2, Max(1, 2))
	assert.Equal(2, Max(2, 1))
}

func TestContainsSOA(t *testing.T) {
	assert := assert.New(t)
	d := &dns.Msg{}
	assert.False(ContainsSOA(d))
	soaRR, err := dns.NewRR("foo.com.		600	IN	SOA	ns-296.awsdns-37.com. awsdns-hostmaster.amazon.com. 1 7200 900 1209600 900")
	assert.Nil(err)
	d.Ns = append(d.Ns, soaRR)
	assert.True(ContainsSOA(d))
}

func TestContainsNS(t *testing.T) {
	assert := assert.New(t)
	d := &dns.Msg{}
	assert.False(ContainsNS(d))
	nsRR, err := dns.NewRR("foo.com.		600	IN	NS	ns-296.awsdns-37.com.")
	assert.Nil(err)
	d.Ns = append(d.Ns, nsRR)
	assert.True(ContainsNS(d))
}
