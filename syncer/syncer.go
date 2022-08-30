/*
Copyright 2022.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package syncer

import (
	"context"
	"github.com/go-logr/logr"
	appstudioshared "github.com/redhat-appstudio/managed-gitops/appstudio-shared/apis/appstudio.redhat.com/v1alpha1"
	"k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Syncer struct {
	client client.Client
	ctx    context.Context
	logger logr.Logger
}

// NewSyncer creates a new Syncer with the given client and logger.
func NewSyncer(client client.Client, logger logr.Logger) *Syncer {
	return NewSyncerWithContext(client, logger, context.TODO())
}

// NewSyncerWithContext creates a new Syncer with the given client, logger and context.
func NewSyncerWithContext(client client.Client, logger logr.Logger, ctx context.Context) *Syncer {
	return &Syncer{
		client: client,
		ctx:    ctx,
		logger: logger,
	}
}

// SetContext sets a new context for the Syncer.
func (s *Syncer) SetContext(ctx context.Context) {
	s.ctx = ctx
}

// SyncSnapshot syncs a Snapshot into the given namespace. If an exiting one is found, no operations will be taken.
func (s *Syncer) SyncSnapshot(snapshot *appstudioshared.ApplicationSnapshot, namespace string) error {
	syncedSnapshot := snapshot.DeepCopy()
	syncedSnapshot.ObjectMeta = v1.ObjectMeta{
		Name:        snapshot.Name,
		Namespace:   namespace,
		Annotations: snapshot.Annotations,
		Labels:      snapshot.Labels,
	}
	err := s.client.Create(s.ctx, syncedSnapshot)
	if err != nil && !errors.IsAlreadyExists(err) {
		return err
	}

	s.logger.Info("Snapshot synced", "Name", syncedSnapshot.Name,
		"Origin namespace", snapshot.Namespace, "Target namespace", syncedSnapshot.Namespace)

	return nil
}
