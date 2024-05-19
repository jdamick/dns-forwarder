package plugins

import (
	"net"
	"time"

	"github.com/miekg/dns"
	"golang.org/x/exp/constraints"
)

const (
	defaultMinTTL = 5 * time.Second
	defaultCapTTL = 1 * time.Hour
)

// DNS Utils

func SynthesizeErrorResponse(req *dns.Msg) *dns.Msg {
	resp := new(dns.Msg)
	resp.SetRcode(req, dns.RcodeServerFailure)
	return resp
}

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

func Min[T constraints.Ordered](d1, d2 T) T {
	if d1 < d2 {
		return d1
	}
	return d2
}

func Max[T constraints.Ordered](d1 T, d2 T) T {
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
		ttl = Min(ttl, time.Duration(r.Header().Ttl)*time.Second)
	}
	for _, r := range m.Ns {
		ttl = Min(ttl, time.Duration(r.Header().Ttl)*time.Second)
	}
	for _, r := range m.Extra {
		if r.Header().Rrtype != dns.TypeOPT {
			ttl = Min(ttl, time.Duration(r.Header().Ttl)*time.Second)
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

func DeepCopyAddr(addr net.Addr) net.Addr {
	switch addr := addr.(type) {
	case *net.UDPAddr:
		return &net.UDPAddr{
			IP:   append([]byte(nil), addr.IP...),
			Port: addr.Port,
			Zone: addr.Zone,
		}
	case *net.TCPAddr:
		return &net.TCPAddr{
			IP:   append([]byte(nil), addr.IP...),
			Port: addr.Port,
			Zone: addr.Zone,
		}
	case *net.UnixAddr:
		return &net.UnixAddr{
			Name: addr.Name,
			Net:  addr.Net,
		}
	case *net.IPAddr:
		return &net.IPAddr{
			IP:   append([]byte(nil), addr.IP...),
			Zone: addr.Zone,
		}
	case *net.IPNet:
		return &net.IPNet{
			IP:   append([]byte(nil), addr.IP...),
			Mask: addr.Mask,
		}
	default:
		return nil
	}
}
