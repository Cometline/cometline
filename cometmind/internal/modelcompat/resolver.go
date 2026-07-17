// Package modelcompat learns optional model feature compatibility from explicit
// provider rejections without exposing provider-specific rules to the agent loop.
package modelcompat

import (
	"context"
	"sync"
	"time"

	cometsdk "github.com/cometline/comet-sdk"
	"github.com/cometline/cometmind/internal/db"
	"github.com/cometline/cometmind/internal/logging"
)

const negativeTTL = 7 * 24 * time.Hour

type Resolver struct {
	q *db.Queries
}

func New(q *db.Queries) *Resolver { return &Resolver{q: q} }

func (r *Resolver) ResolveCapabilityPolicy(ctx context.Context, scope cometsdk.CapabilityScope) cometsdk.CapabilityPolicy {
	disabled := make(map[cometsdk.Capability]struct{})
	if r == nil || r.q == nil {
		return &policy{disabled: disabled}
	}
	features, err := r.q.ListActiveModelCapabilityNegatives(ctx, db.ListActiveModelCapabilityNegativesParams{
		ProviderID: scope.ProviderID,
		Endpoint:   scope.Endpoint,
		ModelID:    scope.ModelID,
		ExpiresAt:  time.Now().UnixMilli(),
	})
	if err != nil {
		logging.L().Warn("model_compat.cache_read_failed", "error", err, "provider", scope.ProviderID, "model", scope.ModelID)
	} else {
		for _, feature := range features {
			disabled[cometsdk.Capability(feature)] = struct{}{}
		}
		if len(disabled) > 0 {
			logging.L().Debug("model_compat.cache_hit", "provider", scope.ProviderID, "model", scope.ModelID, "features", len(disabled))
		}
	}
	return &policy{resolver: r, scope: scope, disabled: disabled}
}

type policy struct {
	resolver *Resolver
	scope    cometsdk.CapabilityScope
	mu       sync.RWMutex
	disabled map[cometsdk.Capability]struct{}
}

func (p *policy) Disabled(feature cometsdk.Capability) bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	_, ok := p.disabled[feature]
	return ok
}

func (p *policy) MarkUnsupported(feature cometsdk.Capability) {
	p.mu.Lock()
	if _, exists := p.disabled[feature]; exists {
		p.mu.Unlock()
		return
	}
	p.disabled[feature] = struct{}{}
	p.mu.Unlock()
	if p.resolver == nil || p.resolver.q == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	err := p.resolver.q.UpsertModelCapabilityNegative(ctx, db.UpsertModelCapabilityNegativeParams{
		ProviderID: p.scope.ProviderID,
		Endpoint:   p.scope.Endpoint,
		ModelID:    p.scope.ModelID,
		Feature:    string(feature),
		ExpiresAt:  time.Now().Add(negativeTTL).UnixMilli(),
	})
	if err != nil {
		logging.L().Warn("model_compat.cache_write_failed", "error", err, "provider", p.scope.ProviderID, "model", p.scope.ModelID, "feature", feature)
		return
	}
	if err := p.resolver.q.DeleteExpiredModelCapabilityNegatives(ctx, time.Now().UnixMilli()); err != nil {
		logging.L().Warn("model_compat.cache_cleanup_failed", "error", err)
	}
	logging.L().Debug("model_compat.unsupported", "provider", p.scope.ProviderID, "model", p.scope.ModelID, "feature", feature)
}

var _ cometsdk.CapabilityResolver = (*Resolver)(nil)
var _ cometsdk.CapabilityPolicy = (*policy)(nil)
