// Copyright 2021 The Kanister Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package kopia

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"time"

	"github.com/kopia/kopia/fs"
	"github.com/kopia/kopia/fs/virtualfs"
	"github.com/kopia/kopia/repo"
	"github.com/kopia/kopia/snapshot"
	"github.com/kopia/kopia/snapshot/policy"
	"github.com/kopia/kopia/snapshot/snapshotfs"
	"github.com/pkg/errors"
)

// Write creates a kopia snapshot from the given reader with the given path as source
func Write(ctx context.Context, path string, source io.Reader) (string, string, error) {
	password, ok := repo.GetPersistedPassword(ctx, defaultConfigFilePath)
	if !ok || password == "" {
		return "", "", errors.New("Failed to retrieve Kopia client passphrase")
	}

	rep, err := OpenRepository(ctx, defaultConfigFilePath, password, "kanister stream push")
	if err != nil {
		return "", "", errors.Wrap(err, "Failed to open kopia repository")
	}

	// Populate the source info with source path and file
	sourceInfo := snapshot.SourceInfo{
		UserName: rep.ClientOptions().Username,
		Host:     rep.ClientOptions().Hostname,
		Path:     filepath.Dir(path),
	}

	rootDir := virtualfs.NewStaticDirectory(sourceInfo.Path, fs.Entries{
		virtualfs.StreamingFileFromReader(filepath.Base(path), source),
	})

	// Setup kopia uploader
	u := snapshotfs.NewUploader(rep)

	// Create a kopia snapshot
	return SnapshotSource(ctx, rep, u, sourceInfo, rootDir, "Kanister Database Backup")
}

// Read reads a kopia snapshot with the given snapshot ID and copies it to the given target
func Read(ctx context.Context, backupID string, target io.Writer) error {
	password, ok := repo.GetPersistedPassword(ctx, defaultConfigFilePath)
	if !ok || password == "" {
		return errors.New("Failed to retrieve Kopia client passphrase")
	}

	rep, err := OpenRepository(ctx, defaultConfigFilePath, password, "kanister stream push")
	if err != nil {
		return errors.Wrap(err, "Failed to open kopia repository")
	}

	oid, err := snapshotfs.ParseObjectIDWithPath(ctx, rep, backupID)
	if err != nil {
		return errors.Wrapf(err, "Failed to parse backupID: %v", backupID)
	}

	// Open repository object and copy the data to the target
	r, err := rep.OpenObject(ctx, oid)
	if err != nil {
		return errors.Wrapf(err, "error opening object %v", oid)
	}

	defer r.Close() //nolint:errcheck

	_, err = Copy(target, r)

	return errors.Wrap(err, "Failed to copy data to target")
}

// SnapshotSource creates and uploads a kopia snapshot to the given repository
func SnapshotSource(
	ctx context.Context,
	rep repo.RepositoryWriter,
	u *snapshotfs.Uploader,
	sourceInfo snapshot.SourceInfo,
	rootDir fs.Entry,
	description string,
) (string, string, error) {
	fmt.Printf("Snapshotting %v ...\n", sourceInfo)

	t0 := time.Now()

	previous, err := findPreviousSnapshotManifest(ctx, rep, sourceInfo, nil)
	if err != nil {
		return "", "", errors.Wrap(err, "Failed to find previous kopia manifests")
	}

	policyTree, err := policy.TreeForSource(ctx, rep, sourceInfo)
	if err != nil {
		return "", "", errors.Wrap(err, "Failed to get kopia policy tree")
	}

	manifest, err := u.Upload(ctx, rootDir, policyTree, sourceInfo, previous...)
	if err != nil {
		return "", "", errors.Wrap(err, "Failed to upload the kopia snapshot")
	}

	manifest.Description = description

	snapID, err := snapshot.SaveSnapshot(ctx, rep, manifest)
	if err != nil {
		return "", "", errors.Wrap(err, "Failed to save kopia manifest")
	}

	_, err = policy.ApplyRetentionPolicy(ctx, rep, sourceInfo, true)
	if err != nil {
		return "", "", errors.Wrap(err, "Failed to apply kopia retention policy")
	}

	if err = policy.SetManual(ctx, rep, sourceInfo); err != nil {
		return "", "", errors.Wrap(err, "Failed to set manual field in kopia scheduling policy for source")
	}

	if ferr := rep.Flush(ctx); ferr != nil {
		return "", "", errors.Wrap(ferr, "Failed to flush kopia repository")
	}

	fmt.Printf("\nCreated snapshot with root %v and ID %v in %v\n", manifest.RootObjectID(), snapID, time.Since(t0).Truncate(time.Second))

	return string(snapID), string(manifest.RootObjectID()), nil
}

// findPreviousSnapshotManifest returns the list of previous snapshots for a given source,
// including last complete snapshot
func findPreviousSnapshotManifest(ctx context.Context, rep repo.Repository, sourceInfo snapshot.SourceInfo, noLaterThan *time.Time) ([]*snapshot.Manifest, error) {
	man, err := snapshot.ListSnapshots(ctx, rep, sourceInfo)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to list previous kopia snapshots")
	}

	// find latest complete snapshot
	var previousComplete *snapshot.Manifest
	var result []*snapshot.Manifest

	for _, p := range man {
		if noLaterThan != nil && p.StartTime.After(*noLaterThan) {
			continue
		}

		if p.IncompleteReason == "" && (previousComplete == nil || p.StartTime.After(previousComplete.StartTime)) {
			previousComplete = p
		}
	}

	if previousComplete != nil {
		result = append(result, previousComplete)
	}

	return result, nil
}
