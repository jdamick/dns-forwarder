package plugins

import (
	"time"

	"github.com/miekg/dns"
)

const (
	defaultMinTTL = 5 * time.Second
	defaultCapTTL = 1 * time.Hour
)

// DNS Utils

func IsNoData(m *dns.Msg) bool {
	return m.Rcode == dns.RcodeSuccess && len(m.Answer) == 0 && ContainsSOA(m)
}

func IsNXDomain(m *dns.Msg) bool {
	return m.Rcode == dns.RcodeNameError && ContainsSOA(m)
}

func ContainsSOA(m *dns.Msg) bool {
	soa := false
	for _, r := range m.Ns {
		if r.Header().Rrtype == dns.TypeSOA {
			soa = true
			break
		}
	}
	return soa
}

func ContainsNS(m *dns.Msg) bool {
	ns := false
	for _, r := range m.Ns {
		if r.Header().Rrtype == dns.TypeNS {
			ns = true
			break
		}
	}
	return ns
}

func MinDuration(d1, d2 time.Duration) time.Duration {
	if d1 < d2 {
		return d1
	}
	return d2
}

func MaxDuration(d1, d2 time.Duration) time.Duration {
	if d1 < d2 {
		return d2
	}
	return d1
}

func FindTTL(m *dns.Msg) time.Duration {
	ttl := defaultMinTTL

	if len(m.Answer) == 0 && len(m.Ns) == 0 && len(m.Extra) == 0 {
		return ttl
	}

	ttl = defaultCapTTL
	for _, r := range m.Answer {
		// fmt.Printf("r: %v\n", r)
		ttl = MinDuration(ttl, time.Duration(r.Header().Ttl)*time.Second)
	}
	for _, r := range m.Ns {
		// fmt.Printf("r: %v\n", r)
		ttl = MinDuration(ttl, time.Duration(r.Header().Ttl)*time.Second)
	}
	for _, r := range m.Extra {
		// fmt.Printf("r: %v\n", r)
		if r.Header().Rrtype != dns.TypeOPT {
			ttl = MinDuration(ttl, time.Duration(r.Header().Ttl)*time.Second)
		}
	}

	return ttl
}

func UpdateTTL(m *dns.Msg, ttlDuration time.Duration) {
	ttl := uint32(ttlDuration.Seconds())
	for _, r := range m.Answer {
		r.Header().Ttl = ttl
	}
	for _, r := range m.Ns {
		r.Header().Ttl = ttl
	}
	for _, r := range m.Extra {
		if r.Header().Rrtype != dns.TypeOPT {
			r.Header().Ttl = ttl
		}
	}
}

func ReverseString(s string) string {
	r := []rune(s)
	for i, j := 0, len(r)-1; i < len(r)/2; i, j = i+1, j-1 {
		r[i], r[j] = r[j], r[i]
	}
	return string(r)
}
