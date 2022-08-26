/*
Copyright 2022 The Kubernetes Authors.

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

package cluster

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/pkg/errors"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"sigs.k8s.io/cluster-api/cmd/clusterctl/internal/scheme"
	logf "sigs.k8s.io/cluster-api/cmd/clusterctl/log"
)

// crdMigrator migrates CRs to the storage version of new CRDs.
// This is necessary when the new CRD drops a version which
// was previously used as a storage version.
type crdMigrator struct {
	Client client.Client
}

// newCRDMigrator creates a new CRD migrator.
func newCRDMigrator(client client.Client) *crdMigrator {
	return &crdMigrator{
		Client: client,
	}
}

// Run migrates CRs to the storage version of new CRDs.
// This is necessary when the new CRD drops a version which
// was previously used as a storage version.
func (m *crdMigrator) Run(ctx context.Context, objs []unstructured.Unstructured) error {
	for i := range objs {
		obj := objs[i]

		if obj.GetKind() == "CustomResourceDefinition" {
			crd := &apiextensionsv1.CustomResourceDefinition{}
			if err := scheme.Scheme.Convert(&obj, crd, nil); err != nil {
				return errors.Wrapf(err, "failed to convert CRD %q", obj.GetName())
			}

			if _, err := m.run(ctx, crd); err != nil {
				return err
			}
		}
	}
	return nil
}

// run migrates CRs of a new CRD.
// This is necessary when the new CRD drops a version which
// was previously used as a storage version.
func (m *crdMigrator) run(ctx context.Context, newCRD *apiextensionsv1.CustomResourceDefinition) (bool, error) {
	log := logf.Log

	// Gets the list of version supported by the new CRD
	newVersions := sets.NewString()
	for _, version := range newCRD.Spec.Versions {
		newVersions.Insert(version.Name)
	}

	// Get the current CRD.
	currentCRD := &apiextensionsv1.CustomResourceDefinition{}
	if err := retryWithExponentialBackoff(newReadBackoff(), func() error {
		return m.Client.Get(ctx, client.ObjectKeyFromObject(newCRD), currentCRD)
	}); err != nil {
		// Return if the CRD doesn't exist yet. We only have to migrate if the CRD exists already.
		if apierrors.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}

	// Get the storage version of the current CRD.
	currentStorageVersion, err := storageVersionForCRD(currentCRD)
	if err != nil {
		return false, err
	}

	// Return an error, if the current storage version has been dropped in the new CRD.
	if !newVersions.Has(currentStorageVersion) {
		return false, errors.Errorf("unable to upgrade CRD %q because the new CRD does not contain the storage version %q of the current CRD, thus not allowing CR migration", newCRD.Name, currentStorageVersion)
	}

	currentStatusStoredVersions := sets.NewString(currentCRD.Status.StoredVersions...)

	// If the new CRD still contains all current stored versions, nothing to do
	// as no previous storage version will be dropped.
	if newVersions.HasAll(currentStatusStoredVersions.List()...) {
		log.V(2).Info("CRD migration check passed", "name", newCRD.Name)
		return false, nil
	}

	// Otherwise a version that has been used as storage version will be dropped, so it is necessary to migrate all the
	// objects and drop the storage version from the current CRD status before installing the new CRD.
	// Ref https://kubernetes.io/docs/tasks/extend-kubernetes/custom-resources/custom-resource-definition-versioning/#writing-reading-and-updating-versioned-customresourcedefinition-objects
	// Note: We are simply migrating all CR objects independent of the version in which they are actually stored in etcd.
	// This way we can make sure that all CR objects are now stored in the current storage version.
	// Alternatively, we would have to figure out which objects are stored in which version but this information is not
	// exposed by the apiserver.
	storedVersionsToDelete := currentStatusStoredVersions.Difference(newVersions)
	storedVersionsToPreserve := currentStatusStoredVersions.Intersection(newVersions)
	log.Info("CR migration required", "kind", newCRD.Spec.Names.Kind, "storedVersionsToDelete", strings.Join(storedVersionsToDelete.List(), ","), "storedVersionsToPreserve", strings.Join(storedVersionsToPreserve.List(), ","))

	if err := m.migrateResourcesForCRD(ctx, currentCRD, currentStorageVersion); err != nil {
		return false, err
	}

	if err := m.patchCRDStoredVersions(ctx, currentCRD, currentStorageVersion); err != nil {
		return false, err
	}

	return true, nil
}

func (m *crdMigrator) migrateResourcesForCRD(ctx context.Context, crd *apiextensionsv1.CustomResourceDefinition, currentStorageVersion string) error {
	log := logf.Log
	log.Info("Migrating CRs, this operation may take a while...", "kind", crd.Spec.Names.Kind)

	list := &unstructured.UnstructuredList{}
	list.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   crd.Spec.Group,
		Version: currentStorageVersion,
		Kind:    crd.Spec.Names.ListKind,
	})

	var i int
	for {
		if err := retryWithExponentialBackoff(newReadBackoff(), func() error {
			return m.Client.List(ctx, list, client.Continue(list.GetContinue()))
		}); err != nil {
			return errors.Wrapf(err, "failed to list %q", list.GetKind())
		}

		for i := range list.Items {
			obj := list.Items[i]

			log.V(5).Info("Migrating", logf.UnstructuredToValues(obj)...)
			if err := retryWithExponentialBackoff(newWriteBackoff(), func() error {
				return handleMigrateErr(m.Client.Update(ctx, &obj))
			}); err != nil {
				return errors.Wrapf(err, "failed to migrate %s/%s", obj.GetNamespace(), obj.GetName())
			}

			// Add some random delays to avoid pressure on the API server.
			i++
			if i%10 == 0 {
				log.V(2).Info(fmt.Sprintf("%d objects migrated", i))
				time.Sleep(time.Duration(rand.IntnRange(50*int(time.Millisecond), 250*int(time.Millisecond))))
			}
		}

		if list.GetContinue() == "" {
			break
		}
	}

	log.V(2).Info(fmt.Sprintf("CR migration completed: migrated %d objects", i), "kind", crd.Spec.Names.Kind)
	return nil
}

func (m *crdMigrator) patchCRDStoredVersions(ctx context.Context, crd *apiextensionsv1.CustomResourceDefinition, currentStorageVersion string) error {
	crd.Status.StoredVersions = []string{currentStorageVersion}
	if err := retryWithExponentialBackoff(newWriteBackoff(), func() error {
		return m.Client.Status().Update(ctx, crd)
	}); err != nil {
		return errors.Wrapf(err, "failed to update status.storedVersions for CRD %q", crd.Name)
	}
	return nil
}

// handleMigrateErr will absorb certain types of errors that we know can be skipped/passed on
// during a migration of a particular object.
func handleMigrateErr(err error) error {
	if err == nil {
		return nil
	}

	// If the resource no longer exists, don't return the error as the object no longer
	// needs updating to the new API version.
	if apierrors.IsNotFound(err) {
		return nil
	}

	// If there was a conflict, another client must have written the object already which
	// means we don't need to force an update.
	if apierrors.IsConflict(err) {
		return nil
	}
	return err
}

// storageVersionForCRD discovers the storage version for a given CRD.
func storageVersionForCRD(crd *apiextensionsv1.CustomResourceDefinition) (string, error) {
	for _, v := range crd.Spec.Versions {
		if v.Storage {
			return v.Name, nil
		}
	}
	return "", errors.Errorf("could not find storage version for CRD %q", crd.Name)
}
