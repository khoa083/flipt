package git

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"slices"
	"sync"

	"github.com/go-git/go-billy/v5/osfs"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/cache"
	"github.com/go-git/go-git/v5/plumbing/transport"
	gitstorage "github.com/go-git/go-git/v5/storage"
	"github.com/go-git/go-git/v5/storage/filesystem"
	"github.com/go-git/go-git/v5/storage/memory"
	"go.flipt.io/flipt/internal/containers"
	"go.flipt.io/flipt/internal/gitfs"
	"go.flipt.io/flipt/internal/storage"
	storagefs "go.flipt.io/flipt/internal/storage/fs"
	"go.uber.org/zap"
)

// REFERENCE_CACHE_EXTRA_CAPACITY is the additionally capacity reserved in the cache
// for non-default references
const REFERENCE_CACHE_EXTRA_CAPACITY = 3

// ensure that the git *Store implements storage.ReferencedSnapshotStore
var _ storagefs.ReferencedSnapshotStore = (*SnapshotStore)(nil)

// SnapshotStore is an implementation of storage.SnapshotStore
// This implementation is backed by a Git repository and it tracks an upstream reference.
// When subscribing to this source, the upstream reference is tracked
// by polling the upstream on a configurable interval.
type SnapshotStore struct {
	*storagefs.Poller

	logger            *zap.Logger
	storage           gitstorage.Storer
	url               string
	baseRef           string
	refTypeTag        bool
	referenceResolver referenceResolver
	directory         string
	path              string
	auth              transport.AuthMethod
	insecureSkipTLS   bool
	caBundle          []byte
	pollOpts          []containers.Option[storagefs.Poller]

	mu   sync.RWMutex
	repo *git.Repository

	snaps *storagefs.SnapshotCache[plumbing.Hash]
}

// WithRef configures the target reference to be used when fetching
// and building fs.FS implementations.
// If it is a valid hash, then the fixed SHA value is used.
// Otherwise, it is treated as a reference in the origin upstream.
func WithRef(ref string) containers.Option[SnapshotStore] {
	return func(s *SnapshotStore) {
		s.baseRef = ref
	}
}

// WithSemverResolver configures how the reference will be resolved for the repository.
func WithSemverResolver() containers.Option[SnapshotStore] {
	return func(s *SnapshotStore) {
		s.refTypeTag = true
		s.referenceResolver = semverResolver()
	}
}

// WithPollOptions configures the poller used to trigger update procedures
func WithPollOptions(opts ...containers.Option[storagefs.Poller]) containers.Option[SnapshotStore] {
	return func(s *SnapshotStore) {
		s.pollOpts = append(s.pollOpts, opts...)
	}
}

// WithAuth returns an option which configures the auth method used
// by the provided source.
func WithAuth(auth transport.AuthMethod) containers.Option[SnapshotStore] {
	return func(s *SnapshotStore) {
		s.auth = auth
	}
}

// WithInsecureTLS returns an option which configures the insecure TLS
// setting for the provided source.
func WithInsecureTLS(insecureSkipTLS bool) containers.Option[SnapshotStore] {
	return func(s *SnapshotStore) {
		s.insecureSkipTLS = insecureSkipTLS
	}
}

// WithCABundle returns an option which configures the CA Bundle used for
// validating the TLS connection to the provided source.
func WithCABundle(caCertBytes []byte) containers.Option[SnapshotStore] {
	return func(s *SnapshotStore) {
		if caCertBytes != nil {
			s.caBundle = caCertBytes
		}
	}
}

// WithDirectory sets a root directory which the store will walk from
// to discover feature flag state files.
func WithDirectory(directory string) containers.Option[SnapshotStore] {
	return func(ss *SnapshotStore) {
		ss.directory = directory
	}
}

// WithFilesystemStorage configures the Git repository to clone into
// the local filesystem, instead of the default which is in-memory.
// The provided path is location for the dotgit folder.
func WithFilesystemStorage(path string) containers.Option[SnapshotStore] {
	return func(ss *SnapshotStore) {
		ss.path = path
		fs := osfs.New(path)
		ss.storage = filesystem.NewStorage(fs, cache.NewObjectLRUDefault())
	}
}

// NewSnapshotStore constructs and configures a Store.
// The store uses the connection and credential details provided to build
// fs.FS implementations around a target git repository.
func NewSnapshotStore(ctx context.Context, logger *zap.Logger, url string, opts ...containers.Option[SnapshotStore]) (_ *SnapshotStore, err error) {
	store := &SnapshotStore{
		logger:            logger.With(zap.String("repository", url)),
		storage:           memory.NewStorage(),
		url:               url,
		baseRef:           "main",
		referenceResolver: staticResolver(),
	}
	containers.ApplyAll(store, opts...)

	store.logger = store.logger.With(zap.String("ref", store.baseRef))

	store.snaps, err = storagefs.NewSnapshotCache[plumbing.Hash](logger, REFERENCE_CACHE_EXTRA_CAPACITY)
	if err != nil {
		return nil, err
	}

	empty := true
	// if the path is set then we need to check if the directory is empty before
	// attempting to clone the repository
	if store.path != "" {
		entries, err := os.ReadDir(store.path)
		if empty = err != nil || len(entries) == 0; empty {
			// either the directory is empty or we failed to read it
			if err != nil && !os.IsNotExist(err) {
				return nil, fmt.Errorf("failed to read directory: %w", err)
			}
		}
	}

	if !plumbing.IsHash(store.baseRef) {
		// if the base ref is not an explicit SHA then
		// attempt to clone either the explicit branch
		// or all references for tag based semver
		cloneOpts := &git.CloneOptions{
			Auth:            store.auth,
			URL:             store.url,
			CABundle:        store.caBundle,
			InsecureSkipTLS: store.insecureSkipTLS,
		}

		// if our reference is a branch type then we can assume it exists
		// and attempt to only clone from this branch initially
		if !store.refTypeTag {
			cloneOpts.ReferenceName = plumbing.NewBranchReferenceName(store.baseRef)
			cloneOpts.SingleBranch = true
		}

		if empty {
			store.repo, err = git.Clone(store.storage, nil, cloneOpts)
			if err != nil {
				return nil, fmt.Errorf("performing initial clone: %w", err)
			}
		} else {
			store.repo, err = git.Open(store.storage, nil)
			if err != nil {
				return nil, fmt.Errorf("opening existing repository: %w", err)
			}
		}

		// do an initial fetch to setup remote tracking branches
		if _, err := store.fetch(ctx, []string{store.baseRef}); err != nil {
			return nil, fmt.Errorf("performing initial fetch: %w", err)
		}
	} else {
		// fetch single reference
		if empty {
			store.repo, err = git.InitWithOptions(store.storage, nil, git.InitOptions{
				DefaultBranch: plumbing.Main,
			})
			if err != nil {
				return nil, err
			}
		} else {
			store.repo, err = git.Open(store.storage, nil)
			if err != nil {
				return nil, fmt.Errorf("opening existing repository: %w", err)
			}
		}

		if _, err = store.repo.CreateRemote(&config.RemoteConfig{
			Name: "origin",
			URLs: []string{store.url},
		}); err != nil {
			return nil, err
		}

		if err := store.repo.FetchContext(ctx, &git.FetchOptions{
			Auth:            store.auth,
			CABundle:        store.caBundle,
			InsecureSkipTLS: store.insecureSkipTLS,
			Depth:           1,
			RefSpecs: []config.RefSpec{
				config.RefSpec(fmt.Sprintf("%[1]s:%[1]s", store.baseRef)),
			},
		}); err != nil {
			return nil, err
		}
	}

	// fetch base ref snapshot at-least once before returning store
	// to ensure we have a servable default state
	snap, hash, err := store.buildReference(ctx, store.baseRef)
	if err != nil {
		return nil, err
	}

	// base reference is stored as fixed in the cache
	// meaning the reference will never be evicted and
	// always point to a live snapshot
	store.snaps.AddFixed(ctx, store.baseRef, hash, snap)

	store.Poller = storagefs.NewPoller(store.logger, ctx, store.update, store.pollOpts...)

	go store.Poll()

	return store, nil
}

// String returns an identifier string for the store type.
func (*SnapshotStore) String() string {
	return "git"
}

// View accepts a function which takes a *StoreSnapshot.
// It supplies the provided function with a *Snapshot if one can be resolved for the requested revision reference.
// Providing an empty reference defaults View to using the stores base reference.
// The base reference will always be quickly accessible via minimal locking (single read-lock).
// Alternative references which have not yet been observed will be resolved and newly built into snapshots on demand.
func (s *SnapshotStore) View(ctx context.Context, storeRef storage.Reference, fn func(storage.ReadOnlyStore) error) error {
	ref := string(storeRef)
	if ref == "" {
		ref = s.baseRef
	}

	snap, ok := s.snaps.Get(ref)
	if ok {
		return fn(snap)
	}

	refs := s.snaps.References()
	if !slices.Contains(refs, ref) {
		refs = append(refs, ref)
	}

	// force attempt a fetch to get the latest references
	if _, err := s.fetch(ctx, refs); err != nil {
		return err
	}

	hash, err := s.resolve(ref)
	if err != nil {
		return err
	}

	snap, err = s.snaps.AddOrBuild(ctx, ref, hash, s.buildSnapshot)
	if err != nil {
		return err
	}

	return fn(snap)
}

// listRemoteRefs returns a set of branch and tag names present on the remote.
func (s *SnapshotStore) listRemoteRefs(ctx context.Context) (map[string]struct{}, error) {
	remotes, err := s.repo.Remotes()
	if err != nil {
		return nil, err
	}
	var origin *git.Remote
	for _, r := range remotes {
		if r.Config().Name == "origin" {
			origin = r
			break
		}
	}
	if origin == nil {
		return nil, fmt.Errorf("origin remote not found")
	}
	refs, err := origin.ListContext(ctx, &git.ListOptions{
		Auth:            s.auth,
		InsecureSkipTLS: s.insecureSkipTLS,
		CABundle:        s.caBundle,
		Timeout:         10, // in seconds
	})
	if err != nil {
		return nil, err
	}
	result := make(map[string]struct{})
	for _, ref := range refs {
		name := ref.Name()
		if name.IsBranch() {
			result[name.Short()] = struct{}{}
		} else if name.IsTag() {
			result[name.Short()] = struct{}{}
		}
	}
	return result, nil
}

// update fetches from the remote and given that a the target reference
// HEAD updates to a new revision, it builds a snapshot and updates it
// on the store.
func (s *SnapshotStore) update(ctx context.Context) (bool, error) {
	updated, fetchErr := s.fetch(ctx, s.snaps.References())

	if !updated && fetchErr == nil {
		return false, nil
	}

	// If we can't fetch, we need to check if the remote refs have changed
	// and remove any references that are no longer present
	if fetchErr != nil {
		remoteRefs, listErr := s.listRemoteRefs(ctx)
		if listErr != nil {
			// If we can't list remote refs, log and continue (don't remove anything)
			s.logger.Warn("could not list remote refs", zap.Error(listErr))
		} else {
			for _, ref := range s.snaps.References() {
				if ref == s.baseRef {
					continue // never remove the base ref
				}
				if _, ok := remoteRefs[ref]; !ok {
					s.logger.Info("removing missing git ref from cache", zap.String("ref", ref))
					if err := s.snaps.Delete(ref); err != nil {
						s.logger.Error("failed to delete missing git ref from cache", zap.String("ref", ref), zap.Error(err))
					}
				}
			}
		}
	}

	var errs []error
	if fetchErr != nil {
		errs = append(errs, fetchErr)
	}
	for _, ref := range s.snaps.References() {
		hash, err := s.resolve(ref)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		if _, err := s.snaps.AddOrBuild(ctx, ref, hash, s.buildSnapshot); err != nil {
			errs = append(errs, err)
		}
	}
	return true, errors.Join(errs...)
}

func (s *SnapshotStore) fetch(ctx context.Context, heads []string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	refSpecs := []config.RefSpec{}

	if s.refTypeTag {
		refSpecs = append(refSpecs, "+refs/tags/*:refs/tags/*")
	}

	for _, head := range heads {
		refSpecs = append(refSpecs,
			config.RefSpec(fmt.Sprintf("+refs/heads/%[1]s:refs/heads/%[1]s", head)),
		)
	}

	if err := s.repo.FetchContext(ctx, &git.FetchOptions{
		Auth:            s.auth,
		RefSpecs:        refSpecs,
		InsecureSkipTLS: s.insecureSkipTLS,
		CABundle:        s.caBundle,
		Prune:           true,
	}); err != nil {
		if !errors.Is(err, git.NoErrAlreadyUpToDate) {
			return false, err
		}

		return false, nil
	}

	return true, nil
}

func (s *SnapshotStore) buildReference(ctx context.Context, ref string) (*storagefs.Snapshot, plumbing.Hash, error) {
	hash, err := s.resolve(ref)
	if err != nil {
		return nil, plumbing.ZeroHash, err
	}

	snap, err := s.buildSnapshot(ctx, hash)
	if err != nil {
		return nil, plumbing.ZeroHash, err
	}

	return snap, hash, nil
}

func (s *SnapshotStore) resolve(ref string) (plumbing.Hash, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.referenceResolver(s.repo, ref)
}

// buildSnapshot builds a new store snapshot based on the provided hash.
func (s *SnapshotStore) buildSnapshot(ctx context.Context, hash plumbing.Hash) (*storagefs.Snapshot, error) {
	var gfs fs.FS
	gfs, err := gitfs.NewFromRepoHash(s.logger, s.repo, hash)
	if err != nil {
		return nil, err
	}

	if s.directory != "" {
		gfs, err = fs.Sub(gfs, s.directory)
		if err != nil {
			return nil, err
		}
	}

	return storagefs.SnapshotFromFS(s.logger, gfs, storagefs.WithEtag(hash.String()))
}
