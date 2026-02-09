package commands

import "context"

type Proxy interface {
	Authorize(ctx context.Context, cmd Command) error
}

type ProxyChain struct {
	proxies []Proxy
}

func NewProxyChain(proxies ...Proxy) *ProxyChain {
	items := make([]Proxy, 0, len(proxies))
	for _, proxy := range proxies {
		if proxy != nil {
			items = append(items, proxy)
		}
	}
	return &ProxyChain{proxies: items}
}

func (p *ProxyChain) Authorize(ctx context.Context, cmd Command) error {
	for _, proxy := range p.proxies {
		if err := proxy.Authorize(ctx, cmd); err != nil {
			return err
		}
	}
	return nil
}
