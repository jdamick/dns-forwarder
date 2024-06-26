package plugins

import (
	"context"
	"fmt"
	"io"
	"time"

	utils "github.com/jdamick/dns-forwarder/pkg/utils"
	"github.com/maypok86/otter"
	"github.com/miekg/dns"
	log "github.com/rs/zerolog/log"
)

type CacheKeyFunc func(context.Context, *dns.Msg) (string, error)

type CachePlugin struct {
	CacheKey CacheKeyFunc
	config   CachePluginConfig
	cache    otter.Cache[string, *msgCacheEntry]
	handler  Handler
}

func SetNoCache(ctx context.Context, val bool) {
	ResponseMetadata(ctx)[noCacheKey] = val
}
func IsNoCache(ctx context.Context) bool {
	return ResponseMetadata(ctx)[noCacheKey] == true
}

// Default configuration values.
const (
	noCacheKey = "NoCacheKey"
)

type CachePluginConfig struct {
	MaxElements      int           `toml:"maxElements" comment:"Max Elements in cache" default:"1000"`
	MaxStaleElements int           `toml:"maxStaleElements" comment:"Max Elements in stale cache" default:"10000"`
	StaleDuration    time.Duration `toml:"staleDuration" comment:"Duration of stale cache" default:"24h"`
	StaleCache       bool          `toml:"staleCache" comment:"Enable Stale Caching" default:"false"`
	StaleTTL         time.Duration `toml:"staleTTL" comment:"Max TTL of record for a Stale Cache response" default:"30s"`
	NegativeAnswers  bool          `toml:"negativeAnswers" comment:"Enable Negative Answers Caching" default:"false"`
}

// Register this plugin with the DNS Forwarder.
func init() {
	RegisterPlugin(&CachePlugin{})
}

func (q *CachePlugin) Name() string {
	return "cache"
}

// PrintHelp prints the configuration help for the plugin.
func (c *CachePlugin) PrintHelp(out io.Writer) {
	PrintPluginHelp(c.Name(), &c.config, out)
}

// Configure the plugin.
func (c *CachePlugin) Configure(ctx context.Context, config map[string]interface{}) error {
	log.Debug().Any("config", config).Msg("CachePlugin.Configure")

	if err := UnmarshalConfiguration(config, &c.config); err != nil {
		return err
	}

	if c.CacheKey == nil {
		c.CacheKey = defaultCacheKeyFunc
	}

	cache, err := otter.MustBuilder[string, *msgCacheEntry](c.config.MaxElements).
		CollectStats().
		WithTTL(c.config.StaleDuration).
		DeletionListener(func(k string, m *msgCacheEntry, cause otter.DeletionCause) {
			log.Debug().Str("key", k).Msg("Cache Deletion Listener")
		}).
		Build()

	c.cache = cache
	log.Debug().Msgf("CachePlugin: %#v", c.config)
	return err
}

// Start the protocol plugin.
func (c *CachePlugin) StartClient(ctx context.Context, handler Handler) error {
	log.Info().Msg("Starting Cache Plugin")
	c.handler = handler
	return nil
}

// Stop the protocol plugin.
func (c *CachePlugin) StopClient(ctx context.Context) error {
	c.cache.Clear()
	return nil
}

func (c *CachePlugin) Query(ctx context.Context, msg *dns.Msg) error {
	key, err := c.CacheKey(ctx, msg)
	if err != nil {
		return err
	}
	if resp := getCacheMsg(c.cache.Extension(), key, false, c.config.StaleTTL); resp != nil {
		log.Debug().Str("key", key).Msg("Cache hit")
		SetNoCache(ctx, true)
		respMsg := resp.Copy()
		respMsg.SetReply(msg)
		_, err = c.handler.Handle(ctx, respMsg)
	} else {
		log.Debug().Str("key", key).Msg("Cache miss")
	}

	return err
}

type msgCacheEntry struct {
	received time.Time
	ttl      time.Duration
	msg      *dns.Msg
}

func getCacheMsg(cacheExt otter.Extension[string, *msgCacheEntry], key string, allowStale bool, staleTTL time.Duration) *dns.Msg {
	if entry, found := cacheExt.GetEntry(key); found {
		msgEntry := entry.Value()
		elapsed := time.Since(msgEntry.received)
		ttl := utils.Max(0, msgEntry.ttl-elapsed)
		if msgEntry.ttl-elapsed < 0 {
			if !allowStale {
				return nil
			}
			ttl = staleTTL
		}

		utils.UpdateTTL(msgEntry.msg, ttl)
		return msgEntry.msg
	}
	return nil
}

func (c *CachePlugin) Response(ctx context.Context, msg *dns.Msg) error {
	log.Debug().Msg("Cache Plugin Response")

	// Check stale cache if it's a failure response
	if c.config.StaleCache && msg.Rcode == dns.RcodeServerFailure {
		key, err := c.CacheKey(ctx, msg)
		if err != nil {
			return err
		}
		if resp := getCacheMsg(c.cache.Extension(), key, c.config.StaleCache, c.config.StaleTTL); resp != nil {
			log.Debug().Str("key", key).Msg("Stale Cache hit")
			SetNoCache(ctx, true)
			respMsg := resp.Copy()
			respMsg.SetReply(msg)
			respMsg.CopyTo(msg)
		}
		return nil
	}

	// Cache Storage
	if IsNoCache(ctx) {
		return nil
	}

	ttl := utils.ConstrainTTL(utils.FindTTL(msg), utils.DefaultMinTTL, utils.DefaultCapTTL)
	if ttl == 0 {
		return nil
	}

	// NegativeAnswers handling
	if utils.IsNXDomain(msg) || utils.IsNoData(msg) {
		if !c.config.NegativeAnswers {
			return nil
		}
	} else if msg.Rcode != dns.RcodeSuccess {
		return nil
	}

	key, err := c.CacheKey(ctx, msg)
	if err != nil {
		return err
	}
	if c.cache.Set(key, &msgCacheEntry{msg: msg, received: time.Now(), ttl: ttl}) {
		log.Debug().Str("key", key).Stringer("ttl", ttl).Msg("Cache set")
	} else {
		log.Debug().Str("key", key).Stringer("ttl", ttl).Msg("Cache set failed")
	}
	return nil
}

func defaultCacheKeyFunc(ctx context.Context, msg *dns.Msg) (string, error) {
	qname := msg.Question[0].Name
	qclass := msg.Question[0].Qclass
	qtype := msg.Question[0].Qtype
	cdFlag := 0
	if msg.MsgHdr.CheckingDisabled {
		cdFlag = 1
	}
	doFlag := 0
	if opt := msg.IsEdns0(); opt != nil && opt.Do() {
		doFlag = 1
	}

	key := fmt.Sprintf("%v:%v:%v:%v:%v", qname, qclass, qtype, cdFlag, doFlag)
	return key, nil
}
