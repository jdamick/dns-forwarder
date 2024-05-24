package utils

import (
	"net"
	"testing"
	"time"

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
	assert.NoError(err)
	d.Ns = append(d.Ns, soaRR)
	assert.True(ContainsSOA(d))
}

func TestContainsNS(t *testing.T) {
	assert := assert.New(t)
	d := &dns.Msg{}
	assert.False(ContainsNS(d))
	nsRR, err := dns.NewRR("foo.com.		600	IN	NS	ns-296.awsdns-37.com.")
	assert.NoError(err)
	d.Ns = append(d.Ns, nsRR)
	assert.True(ContainsNS(d))
}

func TestFindTTL(t *testing.T) {
	assert := assert.New(t)
	d := &dns.Msg{}
	nsRR, err := dns.NewRR("foo.com.		600	IN	NS	ns-296.awsdns-37.com.")
	assert.NoError(err)
	d.Ns = append(d.Ns, nsRR)

	ttl := FindTTL(d)
	assert.Equal(600*time.Second, ttl)
	// add more..
	aRR, err := dns.NewRR("foo.com.		100	IN	A	127.0.0.1")
	assert.NoError(err)
	d.Answer = append(d.Answer, aRR)

	ttl = FindTTL(d)
	assert.Equal(100*time.Second, ttl)

	// empty
	ttl = FindTTL(&dns.Msg{})
	assert.Equal(0*time.Second, ttl)
}

func TestConstrainTTL(t *testing.T) {
	assert := assert.New(t)
	ttl := 0 * time.Second
	ttl = ConstrainTTL(ttl, 1*time.Second, 10*time.Second)
	assert.Equal(1*time.Second, ttl)

	ttl = 500 * time.Second
	ttl = ConstrainTTL(ttl, 1*time.Second, 10*time.Second)
	assert.Equal(10*time.Second, ttl)
}

func TestReverseString(t *testing.T) {
	assert := assert.New(t)
	assert.Equal("oof", ReverseString("foo"))
}

func TestDeepCopyAddr(t *testing.T) {
	assert := assert.New(t)
	ip := net.ParseIP("127.0.0.1")
	{
		addr := &net.IPAddr{
			IP:   ip,
			Zone: "1234",
		}
		addr2 := DeepCopyAddr(addr)
		assert.Equal(addr, addr2)
	}
	{
		addr := &net.UDPAddr{
			IP:   ip,
			Port: 53,
			Zone: "1234",
		}
		addr2 := DeepCopyAddr(addr)
		assert.Equal(addr, addr2)
	}
	{
		addr := &net.TCPAddr{
			IP:   ip,
			Port: 53,
			Zone: "1234",
		}
		addr2 := DeepCopyAddr(addr)
		assert.Equal(addr, addr2)
	}
	{
		addr := &net.UnixAddr{
			Name: "1234",
			Net:  "unix",
		}
		addr2 := DeepCopyAddr(addr)
		assert.Equal(addr, addr2)
	}
	{
		addr := &net.IPNet{
			IP:   ip,
			Mask: net.CIDRMask(24, 32),
		}
		addr2 := DeepCopyAddr(addr)
		assert.Equal(addr, addr2)
	}
}
