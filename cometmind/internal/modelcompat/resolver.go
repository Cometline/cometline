// Package modelcompat learns optional model feature compatibility from explicit
// provider rejections without exposing provider-specific rules to the agent loop.
package modelcompat

import (
	"context"
	"sync"
	"time"

	cometsdk "github.com/Cometline/cometline/comet-sdk"
	"github.com/Cometline/cometline/cometmind/internal/db"
	"github.com/Cometline/cometline/cometmind/internal/logging"
)

const negativeTTL = 7 * 24 * time.Hour

type Resolver struct {
	q *db.Queries
}

func New(q *db.Queries) *Resolver { return &Resolver{q: q} }

// ResolveCapabilities returns the capabilities scope is cached as not
// supporting, plus a callback that persists newly rejected ones.
func (r *Resolver) ResolveCapabilities(ctx context.Context, scope cometsdk.CapabilityScope) cometsdk.CapabilityOptions {
	if r == nil || r.q == nil {
		return cometsdk.CapabilityOptions{}
	}
	return cometsdk.CapabilityOptions{
		Unsupported: cometsdk.NewCapabilitySet(r.cachedNegatives(ctx, scope)...),
		OnUnsupported: func(feature cometsdk.Capability) {
			r.recordUnsupported(scope, feature)
		},
	}
}

// ResolveCapabilityPolicy adapts ResolveCapabilities to the legacy policy
// contract.
//
// Deprecated: use ResolveCapabilities.
func (r *Resolver) ResolveCapabilityPolicy(ctx context.Context, scope cometsdk.CapabilityScope) cometsdk.CapabilityPolicy {
	return &legacyPolicy{opts: r.ResolveCapabilities(ctx, scope), marked: make(map[cometsdk.Capability]struct{})}
}

func (r *Resolver) cachedNegatives(ctx context.Context, scope cometsdk.CapabilityScope) []cometsdk.Capability {
	features, err := r.q.ListActiveModelCapabilityNegatives(ctx, db.ListActiveModelCapabilityNegativesParams{
		ProviderID: scope.ProviderID,
		Endpoint:   scope.Endpoint,
		ModelID:    scope.ModelID,
		ExpiresAt:  time.Now().UnixMilli(),
	})
	if err != nil {
		logging.L().Warn("model_compat.cache_read_failed", "error", err, "provider", scope.ProviderID, "model", scope.ModelID)
		return nil
	}
	negatives := make([]cometsdk.Capability, 0, len(features))
	for _, feature := range features {
		negatives = append(negatives, cometsdk.Capability(feature))
	}
	if len(negatives) > 0 {
		logging.L().Debug("model_compat.cache_hit", "provider", scope.ProviderID, "model", scope.ModelID, "features", len(negatives))
	}
	return negatives
}

func (r *Resolver) recordUnsupported(scope cometsdk.CapabilityScope, feature cometsdk.Capability) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	err := r.q.UpsertModelCapabilityNegative(ctx, db.UpsertModelCapabilityNegativeParams{
		ProviderID: scope.ProviderID,
		Endpoint:   scope.Endpoint,
		ModelID:    scope.ModelID,
		Feature:    string(feature),
		ExpiresAt:  time.Now().Add(negativeTTL).UnixMilli(),
	})
	if err != nil {
		logging.L().Warn("model_compat.cache_write_failed", "error", err, "provider", scope.ProviderID, "model", scope.ModelID, "feature", feature)
		return
	}
	if err := r.q.DeleteExpiredModelCapabilityNegatives(ctx, time.Now().UnixMilli()); err != nil {
		logging.L().Warn("model_compat.cache_cleanup_failed", "error", err)
	}
	logging.L().Debug("model_compat.unsupported", "provider", scope.ProviderID, "model", scope.ModelID, "feature", feature)
}

// legacyPolicy serves ResolveCapabilityPolicy callers: it remembers features
// marked during the request and persists each one once.
type legacyPolicy struct {
	opts   cometsdk.CapabilityOptions
	mu     sync.RWMutex
	marked map[cometsdk.Capability]struct{}
}

func (p *legacyPolicy) Disabled(feature cometsdk.Capability) bool {
	if p.opts.Unsupported.Has(feature) {
		return true
	}
	p.mu.RLock()
	defer p.mu.RUnlock()
	_, ok := p.marked[feature]
	return ok
}

func (p *legacyPolicy) MarkUnsupported(feature cometsdk.Capability) {
	if p.opts.Unsupported.Has(feature) {
		return
	}
	p.mu.Lock()
	if _, exists := p.marked[feature]; exists {
		p.mu.Unlock()
		return
	}
	p.marked[feature] = struct{}{}
	p.mu.Unlock()
	if p.opts.OnUnsupported != nil {
		p.opts.OnUnsupported(feature)
	}
}

var _ cometsdk.CapabilitySource = (*Resolver)(nil)
