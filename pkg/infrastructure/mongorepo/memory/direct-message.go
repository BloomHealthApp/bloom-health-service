package memory

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/BloomHealthApp/bloom-health-service/pkg/domain/models/dao"
	apperrors "github.com/BloomHealthApp/bloom-health-service/pkg/errors"
	"github.com/BloomHealthApp/bloom-health-service/pkg/infrastructure/mongorepo"
)

type directMessageRepo struct {
	mu      sync.Mutex
	threads map[string]*dao.DMThread
	msgs    map[string]*dao.DirectMessage
}

func NewDirectMessageRepo() mongorepo.IDirectMessageRepo {
	return &directMessageRepo{threads: map[string]*dao.DMThread{}, msgs: map[string]*dao.DirectMessage{}}
}

func cloneThread(t *dao.DMThread) dao.DMThread {
	out := *t
	out.Members = append([]string(nil), t.Members...)
	if t.Last != nil {
		l := *t.Last
		out.Last = &l
	}
	if t.ReadAt != nil {
		out.ReadAt = map[string]time.Time{}
		for k, v := range t.ReadAt {
			out.ReadAt[k] = v
		}
	}
	return out
}

func (r *directMessageRepo) NextSeq(_ context.Context, pairKey string, members []string) (int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	t, ok := r.threads[pairKey]
	if !ok {
		t = &dao.DMThread{ID: pairKey, Members: append([]string(nil), members...)}
		r.threads[pairKey] = t
	}
	t.NextSeq++
	return t.NextSeq, nil
}

func (r *directMessageRepo) Insert(_ context.Context, m *dao.DirectMessage) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.msgs[m.ID]; ok {
		return apperrors.ErrRepoConflict
	}
	for _, x := range r.msgs {
		if x.PairKey == m.PairKey && x.Seq == m.Seq {
			return apperrors.ErrRepoConflict
		}
	}
	c := *m
	r.msgs[m.ID] = &c
	return nil
}

func (r *directMessageRepo) ByID(_ context.Context, id string) (*dao.DirectMessage, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if m, ok := r.msgs[id]; ok {
		c := *m
		return &c, nil
	}
	return nil, apperrors.ErrRepoNotFound
}

func (r *directMessageRepo) SetLast(_ context.Context, m *dao.DirectMessage) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	t, ok := r.threads[m.PairKey]
	if !ok || (t.Last != nil && t.Last.Seq >= m.Seq) {
		return nil
	}
	t.Last = &dao.DMLast{Seq: m.Seq, SenderID: m.SenderID, Text: m.Text, CreatedAt: m.CreatedAt}
	return nil
}

func (r *directMessageRepo) List(_ context.Context, pairKey string, beforeSeq, afterSeq int64, limit int) ([]dao.DirectMessage, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := []dao.DirectMessage{}
	for _, m := range r.msgs {
		if m.PairKey != pairKey || (afterSeq > 0 && m.Seq <= afterSeq) || (afterSeq <= 0 && beforeSeq > 0 && m.Seq >= beforeSeq) {
			continue
		}
		out = append(out, *m)
	}
	if afterSeq > 0 {
		sort.Slice(out, func(i, j int) bool { return out[i].Seq < out[j].Seq })
	} else {
		sort.Slice(out, func(i, j int) bool { return out[i].Seq > out[j].Seq })
	}
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (r *directMessageRepo) Thread(_ context.Context, pairKey string) (*dao.DMThread, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if t, ok := r.threads[pairKey]; ok {
		c := cloneThread(t)
		return &c, nil
	}
	return nil, apperrors.ErrRepoNotFound
}

func (r *directMessageRepo) ThreadsFor(_ context.Context, userID string, limit int) ([]dao.DMThread, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := []dao.DMThread{}
	for _, t := range r.threads {
		if t.Last != nil && contains(t.Members, userID) {
			out = append(out, cloneThread(t))
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Last.CreatedAt.After(out[j].Last.CreatedAt) })
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (r *directMessageRepo) MarkRead(_ context.Context, pairKey, userID string, at time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	t, ok := r.threads[pairKey]
	if !ok || !contains(t.Members, userID) {
		return nil
	}
	if t.ReadAt == nil {
		t.ReadAt = map[string]time.Time{}
	}
	if at.After(t.ReadAt[userID]) {
		t.ReadAt[userID] = at
	}
	return nil
}

func (r *directMessageRepo) CountSentSince(_ context.Context, senderID string, since time.Time) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	n := 0
	for _, m := range r.msgs {
		if m.SenderID == senderID && !m.CreatedAt.Before(since) {
			n++
		}
	}
	return n, nil
}

func (r *directMessageRepo) DeletePair(_ context.Context, pairKey string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.deletePairLocked(pairKey)
	return nil
}

func (r *directMessageRepo) deletePairLocked(pairKey string) {
	for id, m := range r.msgs {
		if m.PairKey == pairKey {
			delete(r.msgs, id)
		}
	}
	delete(r.threads, pairKey)
}

// eraseUser drops every thread userID is in, with its messages (account erase).
func (r *directMessageRepo) eraseUser(userID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for key, t := range r.threads {
		if contains(t.Members, userID) {
			r.deletePairLocked(key)
		}
	}
}

func contains(xs []string, v string) bool {
	for _, x := range xs {
		if x == v {
			return true
		}
	}
	return false
}
