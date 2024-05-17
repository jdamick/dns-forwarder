package plugins

import (
	"context"
	"io"
	"net"
	"strings"

	"github.com/miekg/dns"
	"github.com/rs/zerolog/log"
)

type QueryLoggerPlugin struct {
}

// Register this plugin with the DNS Forwarder.
func init() {
	RegisterPlugin(&QueryLoggerPlugin{})
}

func (q *QueryLoggerPlugin) Name() string {
	return "querylogger"
}

// PrintHelp prints the configuration help for the plugin.
func (q *QueryLoggerPlugin) PrintHelp(out io.Writer) {
	PrintPluginHelp(q.Name(), nil, out)
}

// Configure the plugin.
func (q *QueryLoggerPlugin) Configure(ctx context.Context, config map[string]interface{}) error {
	return nil
}

const emptyValue = "-"

/* Following: RFC 8427
"queryMessage": { "ID": 32784, "QR": 0, "Opcode": 0, "AA": 0,
                      "TC": 0, "RD": 0, "RA": 0, "AD": 0, "CD": 0,
                      "RCODE": 0, "QDCOUNT": 1, "ANCOUNT": 0,
                      "NSCOUNT": 0, "ARCOUNT": 0,
                      "QNAME": "example.com.",
                      "QTYPE": 1, "QCLASS": 1 },
*/

func (q *QueryLoggerPlugin) Query(ctx context.Context, msg *dns.Msg) error {
	LogRfc8427Style(ctx, msg)
	LogTextStyle(ctx, msg)
	return nil
}

func (c *QueryLoggerPlugin) Response(ctx context.Context, msg *dns.Msg) error {
	LogRfc8427Style(ctx, msg)
	LogTextStyle(ctx, msg)
	return nil
}

func LogRfc8427Style(ctx context.Context, msg *dns.Msg) error {
	return LogRfc8427StyleWithPrefix("", "", ctx, msg)
}

func LogMsgWithPrefix(prefixKey, prefixVal string, ctx context.Context, msg *dns.Msg) error {
	return LogRfc8427StyleWithPrefix("", "", ctx, msg)
}

// This version is significantly slower (~6x) than the text-based version
func LogRfc8427StyleWithPrefix(prefixKey, prefixVal string, ctx context.Context, msg *dns.Msg) error {
	question := safeQuestion(msg)
	proto, srcIp := remoteAddr(ctx)
	opt := msg.IsEdns0()

	logInfo := log.Info().
		Str("src", srcIp).
		Str("proto", proto).
		Uint16("ID", msg.MsgHdr.Id).
		Uint8("QR", boolToUint8(msg.MsgHdr.Response)).
		Int("Opcode", msg.MsgHdr.Opcode).
		Uint8("AA", boolToUint8(msg.MsgHdr.Authoritative)).
		Uint8("TC", boolToUint8(msg.MsgHdr.Truncated)).
		Uint8("RD", boolToUint8(msg.MsgHdr.RecursionDesired)).
		Uint8("RA", boolToUint8(msg.MsgHdr.RecursionAvailable)).
		Uint8("AD", boolToUint8(msg.MsgHdr.AuthenticatedData)).
		Uint8("CD", boolToUint8(msg.MsgHdr.CheckingDisabled)).
		Uint8("DO", boolToUint8(opt != nil && opt.Do())).
		Int("RCODE", msg.MsgHdr.Rcode).
		Int("QDCOUNT", len(msg.Question)).
		Int("ANCOUNT", len(msg.Answer)).
		Int("NSCOUNT", len(msg.Ns)).
		Int("ARCOUNT", len(msg.Extra)).
		Str("QNAME", question.Name).
		Uint16("QTYPE", question.Qtype).
		Str("QTYPEname", dns.Type(question.Qtype).String()).
		Uint16("QCLASS", question.Qclass).
		Str("QCLASSname", dns.Class(question.Qclass).String())

	if prefixKey != "" && prefixVal != "" {
		logInfo = logInfo.Str(prefixKey, prefixVal)
	}

	if msg.MsgHdr.Response {
		logInfo.Msg("responseMessage")
		return nil
	}

	logInfo.Msg("queryMessage")
	return nil
}

// Similar to CoreDNS Format:
// 	CommonLogFormat = `{remote}:{port} ` + replacer.EmptyValue + ` {>id} "{type} {class} {name} {proto} {size} {>do} {>bufsize}" {rcode} {>rflags} {rsize} {duration}`
// [1553775695] unbound[32655:0] info: 127.0.0.1 clients4.google.com. A IN

func LogTextStyle(ctx context.Context, msg *dns.Msg) error {
	question := safeQuestion(msg)
	qname := question.Name
	qtype := dns.Type(question.Qtype).String()
	qclass := dns.Class(question.Qclass).String()
	proto, addr := remoteAddr(ctx)
	rcode := dns.RcodeToString[msg.MsgHdr.Rcode]

	if msg.MsgHdr.Response {
		log.Info().Msgf("response: %s %s %d %s %s %s %s %s",
			addr, proto, msg.MsgHdr.Id, qname, qclass, qtype, rcode, flagsAsLetters(msg))
		return nil
	}

	log.Info().Msgf("query: %s %s %d %s %s %s %s %s",
		addr, proto, msg.MsgHdr.Id, qname, qclass, qtype, rcode, flagsAsLetters(msg))
	return nil
}

func localAddr(ctx context.Context) (proto string, addr string) {
	return addrInfo(ctx, "LocalAddr")
}
func remoteAddr(ctx context.Context) (proto string, addr string) {
	return addrInfo(ctx, "RemoteAddr")
}

func addrInfo(ctx context.Context, key string) (proto string, addr string) {
	proto = emptyValue
	addr = emptyValue
	laddr := QueryMetadata(ctx)[key].(net.Addr)
	if laddr != nil {
		proto = laddr.Network()
		addr = laddr.String()
	}
	return
}

func flagsAsLetters(msg *dns.Msg) string {
	flags := []string{}
	if msg.MsgHdr.Authoritative {
		flags = append(flags, "aa")
	}
	if msg.MsgHdr.RecursionDesired {
		flags = append(flags, "rd")
	}
	if msg.MsgHdr.Truncated {
		flags = append(flags, "tc")
	}
	if msg.MsgHdr.RecursionAvailable {
		flags = append(flags, "ra")
	}
	if msg.MsgHdr.AuthenticatedData {
		flags = append(flags, "ad")
	}
	if msg.MsgHdr.CheckingDisabled {
		flags = append(flags, "cd")
	}
	if msg.MsgHdr.Zero {
		flags = append(flags, "z")
	}
	opt := msg.IsEdns0()
	if opt != nil && opt.Do() {
		flags = append(flags, "do")
	}
	return strings.Join(flags, ",")
}

func boolToUint8(val bool) uint8 {
	if val {
		return 1
	}
	return 0
}

var emptyQuestion = dns.Question{Name: emptyValue, Qtype: dns.TypeNone, Qclass: dns.ClassNONE}

func safeQuestion(msg *dns.Msg) *dns.Question {
	if len(msg.Question) > 0 {
		return &msg.Question[0]
	}
	return &emptyQuestion
}
