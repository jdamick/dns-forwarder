package plugins

import (
	"testing"

	"github.com/miekg/dns"
	"github.com/stretchr/testify/assert"
)

func TestSafeQuestion(t *testing.T) {
	assert := assert.New(t)
	d := &dns.Msg{}
	q := safeQuestion(d)
	assert.NotNil(q)
	assert.Equal("-", q.Name)

	d.Question = append(d.Question, dns.Question{Name: "example.com", Qtype: dns.TypeA, Qclass: dns.ClassINET})
	q = safeQuestion(d)
	assert.NotNil(q)
	assert.Equal("example.com", q.Name)
}

func TestFlagsAsLetters(t *testing.T) {
	assert := assert.New(t)
	d := &dns.Msg{}

	d.MsgHdr.AuthenticatedData = true
	d.MsgHdr.Authoritative = true
	d.MsgHdr.CheckingDisabled = true
	d.MsgHdr.RecursionAvailable = true
	d.MsgHdr.RecursionDesired = true
	d.MsgHdr.Response = true
	d.MsgHdr.Truncated = true
	d.MsgHdr.Zero = true

	s := flagsAsLetters(d)
	assert.NotEmpty(s)
	assert.Contains(s, "ad")
	assert.Contains(s, "aa")
	assert.Contains(s, "cd")
	assert.Contains(s, "ra")
	assert.Contains(s, "rd")
	assert.Contains(s, "tc")
	assert.Contains(s, "z")
}

func TestBoolToUint8(t *testing.T) {
	assert := assert.New(t)
	assert.Equal(uint8(0), boolToUint8(false))
	assert.Equal(uint8(1), boolToUint8(true))
}
